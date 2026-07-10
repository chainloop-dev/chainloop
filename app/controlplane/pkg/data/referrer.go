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

	"entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
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
		// Iterate on the items it refer to (references)
		var references []uuid.UUID
		for _, ref := range parentRef.References {
			// amd find it in the DB
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

	// Prepare the workflow query predicate
	predicateWF := []predicate.Workflow{
		workflow.DeletedAtIsNil(), workflow.HasOrganizationWith(organization.IDIn(orgIDs...)),
	}

	// Attach the workflow predicate
	predicateReferrer = append(predicateReferrer, referrer.HasWorkflowsWith(predicateWF...))

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

// projectVisibilityPredicate builds a project predicate that accepts a project iff it belongs to
// one of the allowed orgs AND, when RBAC applies to that org, the project is in the caller's
// visible set. Returns nil when no org grants any project visibility, so callers can treat that
// as "nothing is visible".
func projectVisibilityPredicate(orgIDs []uuid.UUID, visibleProjectsMap map[uuid.UUID][]uuid.UUID) predicate.Project {
	perOrg := make([]predicate.Project, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		visible, hasRBAC := visibleProjectsMap[orgID]
		if !hasRBAC {
			perOrg = append(perOrg, project.HasOrganizationWith(organization.ID(orgID)))
			continue
		}
		if len(visible) == 0 {
			continue // RBAC applies but no project is visible in this org
		}
		perOrg = append(perOrg, project.And(
			project.HasOrganizationWith(organization.ID(orgID)),
			project.IDIn(visible...),
		))
	}
	if len(perOrg) == 0 {
		return nil
	}
	return project.Or(perOrg...)
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

	predicateWF := []predicate.Workflow{
		workflow.DeletedAtIsNil(), workflow.HasOrganizationWith(organization.IDIn(allowedOrgs...)),
	}

	// Attach the workflow predicate
	predicateReferrer = append(predicateReferrer, referrer.HasWorkflowsWith(predicateWF...))

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
				referrer.HasWorkflowsWith(predicateWF...),
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
