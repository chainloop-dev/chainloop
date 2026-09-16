//
// Copyright 2023-2026 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package data

import (
	"context"
	"fmt"
	"slices"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/predicate"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/project"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/projectversion"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/referrer"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowrun"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"golang.org/x/exp/maps"
)

var referrerRepoTracer = otelx.Tracer("chainloop-controlplane", "data/referrer")

type ReferrerRepo struct {
	data         *Data
	log          *log.Helper
	workflowRepo biz.WorkflowRepo
}

func NewReferrerRepo(data *Data, wfRepo biz.WorkflowRepo, logger log.Logger) biz.ReferrerRepo {
	return &ReferrerRepo{
		data:         data,
		log:          log.NewHelper(logger),
		workflowRepo: wfRepo,
	}
}

type storedReferrerMap map[string]uuid.UUID

func (r *ReferrerRepo) Save(ctx context.Context, referrers []*biz.Referrer, workflowID uuid.UUID) (err error) {
	ctx, span := otelx.Start(ctx, referrerRepoTracer, "ReferrerRepo.Save")
	defer span.End()

	// find the workflow
	wf, err := r.workflowRepo.FindByID(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to find workflow: %w", err)
	} else if wf == nil {
		return biz.NewErrNotFound("workflow")
	}

	storedMap := make(storedReferrerMap)

	for _, ref := range referrers {
		// Check if it exists already, if not create it
		storedID, err := r.data.DB.Referrer.Create().
			SetDigest(ref.Digest).SetKind(ref.Kind).SetDownloadable(ref.Downloadable).
			SetMetadata(ref.Metadata).SetAnnotations(ref.Annotations).
			AddWorkflowIDs(workflowID).
			OnConflictColumns(
				referrer.FieldDigest, referrer.FieldKind,
			).UpdateNewValues().ID(ctx)
		if err != nil {
			return fmt.Errorf("failed to create referrer: %w", err)
		}

		storedRef, err := r.data.DB.Referrer.Query().Select(referrer.FieldID).Where(referrer.ID(storedID)).First(ctx)
		if err != nil {
			return fmt.Errorf("failed to load referrer: %w", err)
		} else if storedRef == nil {
			return fmt.Errorf("failed to load referrer: %w", err)
		}

		// Store it in the map
		storedMap[ref.MapID()] = storedRef.ID
	}

	// 2 - define the relationship between referrers
	for _, parentRef := range referrers {
		// This is the current item stored in DB
		storedReferrer := storedMap[parentRef.MapID()]
		// Iterate on the items it refers to (references)
		var references []uuid.UUID
		for _, ref := range parentRef.References {
			// and find it in the DB
			storedReference, ok := storedMap[ref.MapID()]
			if !ok {
				return fmt.Errorf("referrer %v not found", ref)
			}

			references = append(references, storedReference)
		}

		if len(references) == 0 {
			continue
		}

		// Create the relationship
		if err := r.data.DB.Referrer.UpdateOneID(storedReferrer).AddReferenceIDs(references...).Exec(ctx); err != nil {
			return fmt.Errorf("failed to create referrer relationship: %w", err)
		}
	}

	return nil
}

// Check if a given referrer by digest exist. The query can be scoped further down if needed by providing the kind or visibility status
func (r *ReferrerRepo) Exist(ctx context.Context, digest string, filters ...biz.GetFromRootFilter) (bool, error) {
	ctx, span := otelx.Start(ctx, referrerRepoTracer, "ReferrerRepo.Exist")
	defer span.End()

	opts := &biz.GetFromRootFilters{}
	for _, f := range filters {
		f(opts)
	}

	query := r.data.DB.Referrer.Query().Where(referrer.DigestEQ(digest))
	// We might be filtering by the rootKind, this will prevent ambiguity
	if opts.RootKind != nil {
		query = query.Where(referrer.Kind(*opts.RootKind))
	}

	return query.Exist(ctx)
}

func (r *ReferrerRepo) GetFromRoot(ctx context.Context, digest string, orgIDs []uuid.UUID, p *pagination.CursorOptions, filters ...biz.GetFromRootFilter) (*biz.StoredReferrer, string, error) {
	ctx, span := otelx.Start(ctx, referrerRepoTracer, "ReferrerRepo.GetFromRoot")
	defer span.End()

	opts := &biz.GetFromRootFilters{}
	for _, f := range filters {
		f(opts)
	}

	// Find the referrer from its digest + artifactType (optional)
	// if there is more than 1 item we return ReferrerAmbiguous error
	// filter by the allowed organizations and by the visibility of the attached workflows if needed
	predicateReferrer := []predicate.Referrer{
		referrer.Digest(digest),
	}

	// We might be filtering by the rootKind, this will prevent ambiguity
	if opts.RootKind != nil {
		predicateReferrer = append(predicateReferrer, referrer.Kind(*opts.RootKind))
	}

	// Attach the visibility predicate
	predicateReferrer = append(predicateReferrer, referrerVisibleToOrgs(orgIDs))

	// If a project filter is requested, attach it as a subquery predicate. An attestation root
	// matches only when its digest is one of the attestation_digests produced by a workflow run
	// in the requested project (and, when set, version). Non-attestation roots pass through here
	// and are validated later through their references. The cost is independent of how many
	// runs the project has — Postgres executes it as a single semi-join, no Go-side digest list.
	var projectPred predicate.Referrer
	if opts.ProjectName != nil && *opts.ProjectName != "" {
		version := ""
		if opts.ProjectVersion != nil {
			version = *opts.ProjectVersion
		}
		projectPred = r.projectScopePredicate(*opts.ProjectName, version, orgIDs, opts.ProjectIDs)
		predicateReferrer = append(predicateReferrer, referrer.Or(
			referrer.KindNEQ(biz.ReferrerAttestationType),
			projectPred,
		))
	}

	refs, err := r.data.DB.Referrer.Query().Where(predicateReferrer...).WithWorkflows().All(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query referrer: %w", err)
	}

	// No items found
	if numrefs := len(refs); numrefs == 0 {
		return nil, "", nil
	} else if numrefs > 1 {
		// if there is more than 1 item with the same digest+artifactType we will fail
		var kinds []string
		for _, r := range refs {
			kinds = append(kinds, r.Kind)
		}
		return nil, "", biz.NewErrReferrerAmbiguous(digest, kinds)
	}

	// Find the referrer recursively starting from the root
	res, nextCursor, err := r.doGet(ctx, refs[0], orgIDs, opts.ProjectIDs, projectPred, p, 0)
	if err != nil && !biz.IsErrUnauthorized(err) {
		return nil, "", fmt.Errorf("failed to get referrer: %w", err)
	}

	return res, nextCursor, nil
}

// projectScopePredicate returns a predicate matching referrers whose digest is the attestation
// digest of a workflow run in the requested project (and, when non-empty, version), visible to
// the caller. The predicate compiles to a SQL subquery — no digest list is materialized in Go,
// so the cost is independent of how many runs the project has. Postgres plans this as a
// semi-join via the index on workflow_run.attestation_digest, which is what makes the filter
// scale at thousands of runs per project.
//
// Visibility mirrors isReferrerVisible: a run is included when its workflow's project is in the
// caller's RBAC-visible set. visibleProjectsMap follows the existing convention — an org entry
// present means RBAC applies for that org and only the listed project IDs are visible; an org
// absent means no RBAC restriction.
func (r *ReferrerRepo) projectScopePredicate(projectName, version string, orgIDs []uuid.UUID, visibleProjectsMap map[uuid.UUID][]uuid.UUID) predicate.Referrer {
	versionPredicates := []predicate.ProjectVersion{
		projectversion.DeletedAtIsNil(),
		projectversion.HasProjectWith(
			project.NameEQ(projectName),
			project.DeletedAtIsNil(),
		),
	}
	if version != "" {
		versionPredicates = append(versionPredicates, projectversion.VersionEQ(version))
	}
	runPredicates := []predicate.WorkflowRun{
		workflowrun.AttestationDigestNEQ(""),
		workflowrun.HasVersionWith(versionPredicates...),
	}

	// Visibility — same semantics as isReferrerVisible: the run's workflow project must be in the
	// caller's RBAC-visible set. If no project is visible in any allowed org, nothing matches.
	rbacScope := projectVisibilityPredicate(orgIDs, visibleProjectsMap)
	if rbacScope == nil {
		return func(s *sql.Selector) { s.Where(sql.False()) }
	}
	runPredicates = append(runPredicates, workflowrun.HasWorkflowWith(
		workflow.DeletedAtIsNil(),
		workflow.HasProjectWith(rbacScope),
	))

	return func(s *sql.Selector) {
		t := sql.Table(workflowrun.Table)
		sub := sql.Select(t.C(workflowrun.FieldAttestationDigest)).From(t)
		for _, p := range runPredicates {
			p(sub)
		}
		s.Where(sql.In(s.C(referrer.FieldDigest), sub))
	}
}

// referrerVisibleToOrgs matches a referrer that is attached to at least one live workflow in the
// given organizations.
//
// It is written by hand rather than with referrer.HasWorkflowsWith because the generated form
// emits an uncorrelated `id IN (subquery)`: the planner materialises every referrer reachable by
// the caller's organizations and joins afterwards, which on a hub node means scanning the whole
// join table. Correlating the subquery to the referrer row turns it into a lookup per candidate.
// The organization is matched on the workflow's own column for the same reason the project filter
// does: a nested relation predicate here is flattened back into the uncorrelated shape.
func referrerVisibleToOrgs(orgIDs []uuid.UUID) predicate.Referrer {
	orgs := make([]any, 0, len(orgIDs))
	for _, id := range orgIDs {
		orgs = append(orgs, id)
	}

	return func(s *sql.Selector) {
		joinTable := sql.Table(referrer.WorkflowsTable).As("visible_rw")
		workflows := sql.Table(referrer.WorkflowsInverseTable).As("visible_wf")
		sub := sql.Dialect(s.Dialect()).
			Select(joinTable.C(referrer.WorkflowsPrimaryKey[0])).
			From(joinTable).
			Join(workflows).
			On(joinTable.C(referrer.WorkflowsPrimaryKey[1]), workflows.C(workflow.FieldID)).
			Where(sql.And(
				sql.ColumnsEQ(joinTable.C(referrer.WorkflowsPrimaryKey[0]), s.C(referrer.FieldID)),
				sql.IsNull(workflows.C(workflow.FieldDeletedAt)),
				sql.In(workflows.C(workflow.FieldOrganizationID), orgs...),
			))
		s.Where(sql.Exists(sub))
	}
}

// projectVisibilityPredicate builds a project predicate that accepts a project iff it belongs to
// one of the allowed orgs AND, when RBAC applies to that org, the project is in the caller's
// visible set. Returns nil when no org grants any project visibility, so callers can treat that
// as "nothing is visible".
//
// A caller reaches a project one of two ways, so the predicate has at most two branches:
// every project of an org whose role carries no project restriction, or an individually
// granted project in an org whose role does. Each branch is emitted once for the whole set
// of orgs rather than once per org, which keeps the filter a constant size: a disjunction
// that grows with the caller's org count stops the planner from using the root's index and
// turns the surrounding referrer query into a full scan.
//
// The second branch matches (org, project) pairs rather than the two sets independently.
// Intersecting the sets would also admit a project that belongs to one of the caller's
// restricted orgs and happens to be granted in another, which is not a grant the caller
// holds. Pairs cannot express that, and they keep the branch a single condition.
func projectVisibilityPredicate(orgIDs []uuid.UUID, visibleProjectsMap map[uuid.UUID][]uuid.UUID) predicate.Project {
	unrestrictedOrgs := make([]uuid.UUID, 0, len(orgIDs))
	grants := make([]projectGrant, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		visible, restricted := visibleProjectsMap[orgID]
		if !restricted {
			unrestrictedOrgs = append(unrestrictedOrgs, orgID)
			continue
		}
		// Restricted: nothing in this org is visible beyond the projects granted in it,
		// so an org with no grants contributes nothing.
		for _, projectID := range visible {
			grants = append(grants, projectGrant{orgID: orgID, projectID: projectID})
		}
	}

	predicates := make([]predicate.Project, 0, 2)
	if len(unrestrictedOrgs) > 0 {
		// The org is a column on the project row, so this needs no subquery.
		predicates = append(predicates, project.OrganizationIDIn(unrestrictedOrgs...))
	}
	if len(grants) > 0 {
		predicates = append(predicates, projectGrantsPredicate(grants))
	}

	switch len(predicates) {
	case 0:
		return nil
	case 1:
		return predicates[0]
	default:
		return project.Or(predicates...)
	}
}

// projectGrant is a project the caller may see and the org the grant was recorded under.
type projectGrant struct {
	orgID, projectID uuid.UUID
}

// projectGrantsPredicate matches a project iff it is one of the granted projects AND sits in the
// org that grant was recorded under, as a single (organization_id, id) IN ((..), (..)) condition.
func projectGrantsPredicate(grants []projectGrant) predicate.Project {
	return func(s *sql.Selector) {
		s.Where(sql.P().Append(func(b *sql.Builder) {
			b.Wrap(func(nb *sql.Builder) {
				nb.IdentComma(s.C(project.FieldOrganizationID), s.C(project.FieldID))
			})
			b.WriteString(" IN ")
			b.Wrap(func(nb *sql.Builder) {
				for i, g := range grants {
					if i > 0 {
						nb.Comma()
					}
					nb.Wrap(func(vb *sql.Builder) {
						vb.Args(g.orgID, g.projectID)
					})
				}
			})
		}))
	}
}

// max number of recursive levels to traverse
// we just care about 1 level, i.e att -> commit, or commit -> attestation
// we also need to limit this because there might be cycles
const maxTraverseLevels = 1

func (r *ReferrerRepo) doGet(ctx context.Context, root *ent.Referrer, allowedOrgs []uuid.UUID, visibleProjectsMap map[uuid.UUID][]uuid.UUID, projectPred predicate.Referrer, p *pagination.CursorOptions, level int) (*biz.StoredReferrer, string, error) {
	// Assemble the referrer to return
	res := &biz.StoredReferrer{
		ID:        root.ID,
		CreatedAt: toTimePtr(root.CreatedAt),
		Referrer: &biz.Referrer{
			Digest:       root.Digest,
			Kind:         root.Kind,
			Downloadable: root.Downloadable,
			Metadata:     root.Metadata,
			Annotations:  root.Annotations,
		},
	}

	// add additional information related to the workflows
	hydrateWorkflowsInfo(root, res)

	// check that, if RBAC is required, the user has visibility on the artifact in at least 1 org/project
	if visible := isReferrerVisible(res, allowedOrgs, visibleProjectsMap); !visible {
		return nil, "", biz.NewErrUnauthorizedStr("referrer not allowed")
	}

	// When a project filter is active, an attestation root has already been filtered by the
	// initial referrer lookup (the projectPred subquery), so it is guaranteed to belong to the
	// requested project here. A material root passes that lookup unconditionally and is
	// validated through its references below (or via the pagination-independent existence
	// check after the references query).
	projectFilterActive := projectPred != nil

	// Next: We'll find the references recursively up to a max of maxTraverseLevels levels
	if level >= maxTraverseLevels {
		return res, "", nil
	}

	// Find the references and call recursively filtered out by the allowed organizations
	// and by the visibility if needed
	predicateReferrer := []predicate.Referrer{}

	// Attach the visibility predicate
	predicateReferrer = append(predicateReferrer, referrerVisibleToOrgs(allowedOrgs))

	// When scoping to a project, attestation references must belong to that project (optionally
	// narrowed to a version). Non-attestation references (materials/subjects) are kept as-is:
	// they inherit the project through the attestation that references them.
	if projectFilterActive {
		predicateReferrer = append(predicateReferrer, referrer.Or(
			referrer.KindNEQ(biz.ReferrerAttestationType),
			projectPred,
		))
	}

	// Defense-in-depth: if the caller did not supply pagination options, fall back
	// to the package-wide default instead of emitting an unbounded query. This
	// guarantees the response is bounded even when a future biz-layer caller
	// forgets to pass options through — see chainloop-dev/chainloop#2890.
	if p == nil {
		p = &pagination.CursorOptions{Limit: pagination.DefaultCursorLimit}
	}

	// Sort references by creation date and ID in descending order for deterministic pagination
	q := root.QueryReferences().Where(predicateReferrer...).WithWorkflows().
		// Ent adds a DISTINCT to any traversal by default. Here it can only cost: the selected
		// columns include the referrer's primary key, so deduplicating them is a no-op unless a
		// predicate multiplies rows, and none of the predicates above can — they are semi-joins
		// (EXISTS, IN) and scalar comparisons. Dropping it keeps the jsonb columns out of the
		// sort key. Any predicate added here must stay join-free or this silently starts
		// returning a reference more than once, which would also shorten the page.
		Unique(false).
		Order(referrer.ByCreatedAt(sql.OrderDesc())).
		Order(referrer.ByID(sql.OrderDesc())).
		Limit(p.Limit + 1) // fetch limit+1 to detect next page

	if p.Cursor != nil {
		q = q.Where(func(s *sql.Selector) {
			s.Where(sql.CompositeLT(
				[]string{s.C(referrer.FieldCreatedAt), s.C(referrer.FieldID)},
				p.Cursor.Timestamp, p.Cursor.ID,
			))
		})
	}

	refs, err := q.All(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query references: %w", err)
	}

	// Determine if there is a next page and encode the cursor
	var nextCursor string
	if len(refs) > p.Limit {
		lastVisible := refs[p.Limit-1]
		nextCursor = pagination.EncodeCursor(lastVisible.CreatedAt, lastVisible.ID)
		refs = refs[:p.Limit]
	}

	// Add the references to the result
	for _, reference := range refs {
		// Call recursively the function — pagination only applies to the first level
		ref, _, err := r.doGet(ctx, reference, allowedOrgs, visibleProjectsMap, projectPred, nil, level+1)
		if err != nil && !biz.IsErrUnauthorized(err) {
			return nil, "", fmt.Errorf("failed to get referrer: %w", err)
		}

		if ref != nil {
			res.References = append(res.References, ref)
		}
	}

	// A non-attestation root (a material/subject) belongs to the requested project only if it
	// is referenced by at least one attestation in that project (or in the specific version,
	// if one was requested). When the current page yields no references we cannot conclude
	// absence from the page alone (a later page can be empty simply because we paged past the
	// results), so we run a pagination-independent existence check before rejecting the root.
	if projectFilterActive && level == 0 && root.Kind != biz.ReferrerAttestationType && len(res.References) == 0 {
		inProject, err := root.QueryReferences().
			Where(
				referrer.KindEQ(biz.ReferrerAttestationType),
				projectPred,
				referrerVisibleToOrgs(allowedOrgs),
			).
			Exist(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("failed to validate project membership: %w", err)
		}
		if !inProject {
			return nil, "", biz.NewErrUnauthorizedStr("referrer not part of the requested project")
		}
	}

	return res, nextCursor, nil
}

func isReferrerVisible(ref *biz.StoredReferrer, allowedOrgs []uuid.UUID, visibleProjectsMap map[uuid.UUID][]uuid.UUID) bool {
	for _, oid := range ref.OrgIDs {
		if !slices.Contains(allowedOrgs, oid) {
			// skip check in organizations where the user doesn't have access
			continue
		}
		if visibleProjects, ok := visibleProjectsMap[oid]; ok {
			// if entry is present, it means we need to apply RBAC
			// check if visible projects and referrer projects match
			// by checking if any project is visible by the user
			for _, pid := range ref.ProjectIDs {
				if slices.Contains(visibleProjects, pid) {
					return true
				}
			}
		} else {
			// if entry is not found in the map, it means that RBAC is not needed for this org, we have finished
			return true
		}
	}

	return false
}

// hydrate the referrer with the following information:
// - workflowIDs: the list of associated workflows
// - orgIDs: the list of associated organizations
func hydrateWorkflowsInfo(root *ent.Referrer, out *biz.StoredReferrer) {
	workflowIDs := make([]uuid.UUID, 0, len(root.Edges.Workflows))
	projectIDs := make(map[uuid.UUID]bool, 0)
	orgIDs := make([]uuid.UUID, 0)
	orgsMap := make(map[uuid.UUID]struct{}, 0)
	for _, wf := range root.Edges.Workflows {
		workflowIDs = append(workflowIDs, wf.ID)
		if _, ok := orgsMap[wf.OrganizationID]; !ok {
			orgIDs = append(orgIDs, wf.OrganizationID)
		}
		if _, ok := projectIDs[wf.ProjectID]; !ok {
			projectIDs[wf.ProjectID] = true
		}
		orgsMap[wf.OrganizationID] = struct{}{}
	}

	out.ProjectIDs = maps.Keys(projectIDs)
	out.WorkflowIDs = workflowIDs
	out.OrgIDs = orgIDs
}

// EdgesAmong returns the connections between the given referrers, as index pairs into nodes.
//
// The work is bounded by the number of referrers asked about, not by how many references any of
// them has: a shared material can be referenced by a hundred thousand attestations, and asking
// "which of these hundred nodes are connected" must not walk that. It therefore reads the store
// from the attestation side of each connection, whose degree is the number of materials in one
// attestation rather than the number of attestations sharing a material.
//
// That is complete because of how connections are written: a connection only ever exists between
// an attestation and one of its materials or subjects (or another attestation it depends on), and
// the attestation's own row is always written, since ingest builds its reference list from every
// material and subject it carries. The material's row pointing back is written too, but it is not
// what this relies on — older data holds many connections in the attestation direction only.
func (r *ReferrerRepo) EdgesAmong(ctx context.Context, nodes []*biz.ReferrerRef, orgIDs []uuid.UUID, filters ...biz.GetFromRootFilter) ([]biz.ReferrerEdge, error) {
	ctx, span := otelx.Start(ctx, referrerRepoTracer, "ReferrerRepo.EdgesAmong")
	defer span.End()

	if len(nodes) < 2 {
		return nil, nil
	}

	opts := &biz.GetFromRootFilters{}
	for _, f := range filters {
		f(opts)
	}

	// Resolve the referrers the caller may see. Anything invisible or unknown simply does not
	// come back and so contributes no edges.
	predicates := []predicate.Referrer{
		referrerDigestKindIn(nodes),
		referrerVisibleToOrgs(orgIDs),
	}
	if opts.ProjectName != nil && *opts.ProjectName != "" {
		version := ""
		if opts.ProjectVersion != nil {
			version = *opts.ProjectVersion
		}
		projectPred := r.projectScopePredicate(*opts.ProjectName, version, orgIDs, opts.ProjectIDs)
		predicates = append(predicates, referrer.Or(referrer.KindNEQ(biz.ReferrerAttestationType), projectPred))
	}

	visible, err := r.data.DB.Referrer.Query().
		Where(predicates...).
		Select(referrer.FieldID, referrer.FieldDigest, referrer.FieldKind).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve referrers: %w", err)
	}
	if len(visible) < 2 {
		return nil, nil
	}

	// Index the answer by the position the caller gave each referrer, so only positions travel back.
	indexOf := make(map[string]int, len(nodes))
	for i, n := range nodes {
		indexOf[newRefKey(n.Digest, n.Kind)] = i
	}
	position := make(map[uuid.UUID]int, len(visible))
	attestationIDs := make([]uuid.UUID, 0, len(visible))
	visibleIDs := make([]uuid.UUID, 0, len(visible))
	for _, ref := range visible {
		i, ok := indexOf[newRefKey(ref.Digest, ref.Kind)]
		if !ok {
			continue
		}
		position[ref.ID] = i
		visibleIDs = append(visibleIDs, ref.ID)
		if ref.Kind == biz.ReferrerAttestationType {
			attestationIDs = append(attestationIDs, ref.ID)
		}
	}
	if len(attestationIDs) == 0 || len(visibleIDs) < 2 {
		return nil, nil
	}

	// Built with the dialect's builder so the placeholders are numbered for it, and run on the
	// raw handle because the join table has no entity of its own to query through.
	query, args := edgesAmongQuery(attestationIDs, visibleIDs)
	rows, err := r.data.SQLDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query edges: %w", err)
	}
	defer rows.Close()

	// A connection can be read twice: once from each end when both ends are attestations, and
	// once per stored direction where ingest wrote both. The pair is normalised and deduplicated
	// so the caller sees each connection once.
	seen := make(map[biz.ReferrerEdge]struct{})
	edges := make([]biz.ReferrerEdge, 0)
	for rows.Next() {
		var from, to uuid.UUID
		if err := rows.Scan(&from, &to); err != nil {
			return nil, fmt.Errorf("failed to scan edge: %w", err)
		}
		a, okA := position[from]
		b, okB := position[to]
		if !okA || !okB || a == b {
			continue
		}
		if a > b {
			a, b = b, a
		}
		edge := biz.ReferrerEdge{From: a, To: b}
		if _, dup := seen[edge]; dup {
			continue
		}
		seen[edge] = struct{}{}
		edges = append(edges, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read edges: %w", err)
	}

	return edges, nil
}

// referrerDigestKindIn matches any of the given referrers, pairing each digest with its kind so a
// digest stored under two kinds cannot be confused for the other. It is a single composite IN
// rather than a chain of ORs, which keeps it usable through the unique index on (digest, kind).
func referrerDigestKindIn(nodes []*biz.ReferrerRef) predicate.Referrer {
	return func(s *sql.Selector) {
		s.Where(sql.P().Append(func(b *sql.Builder) {
			b.Wrap(func(nb *sql.Builder) {
				nb.IdentComma(s.C(referrer.FieldDigest), s.C(referrer.FieldKind))
			})
			b.WriteString(" IN ")
			b.Wrap(func(nb *sql.Builder) {
				for i, n := range nodes {
					if i > 0 {
						nb.Comma()
					}
					nb.Wrap(func(vb *sql.Builder) {
						vb.Args(n.Digest, n.Kind)
					})
				}
			})
		}))
	}
}

// edgesAmongQuery reads the join table from the attestation side: for each attestation it walks
// only that attestation's own references, restricted to the referrers the caller asked about.
// Both columns are the table's primary key, so this is an index scan whose size is the number of
// materials in the attestations involved.
func edgesAmongQuery(fromIDs, toIDs []uuid.UUID) (string, []any) {
	t := sql.Dialect(dialect.Postgres).Table(referrer.ReferencesTable)
	from := make([]any, 0, len(fromIDs))
	for _, id := range fromIDs {
		from = append(from, id)
	}
	to := make([]any, 0, len(toIDs))
	for _, id := range toIDs {
		to = append(to, id)
	}

	return sql.Dialect(dialect.Postgres).
		Select(referrer.ReferencesPrimaryKey[0], referrer.ReferencesPrimaryKey[1]).
		From(t).
		Where(sql.And(
			sql.In(referrer.ReferencesPrimaryKey[0], from...),
			sql.In(referrer.ReferencesPrimaryKey[1], to...),
		)).
		Query()
}

func newRefKey(digest, kind string) string { return kind + "-" + digest }
