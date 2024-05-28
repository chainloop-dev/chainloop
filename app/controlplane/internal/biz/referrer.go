//
// Copyright 2024 The Chainloop Authors.
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

package biz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/internal/attestation"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	v1 "github.com/in-toto/attestation/go/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type ReferrerUseCase struct {
	repo           ReferrerRepo
	membershipRepo MembershipRepo
	workflowRepo   WorkflowRepo
	logger         *log.Helper
	indexConfig    *conf.ReferrerSharedIndex
}

func NewReferrerUseCase(repo ReferrerRepo, wfRepo WorkflowRepo, mRepo MembershipRepo, indexCfg *conf.ReferrerSharedIndex, l log.Logger) (*ReferrerUseCase, error) {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}
	logger := servicelogger.ScopedHelper(l, "biz/referrer")

	if indexCfg != nil {
		if err := indexCfg.ValidateOrgs(); err != nil {
			return nil, fmt.Errorf("invalid shared index config: %w", err)
		}

		if indexCfg.Enabled {
			logger.Infow("msg", "shared index enabled", "allowedOrgs", indexCfg.AllowedOrgs)
		}
	}

	return &ReferrerUseCase{
		repo:           repo,
		membershipRepo: mRepo,
		indexConfig:    indexCfg,
		workflowRepo:   wfRepo,
		logger:         logger,
	}, nil
}

type ReferrerRepo interface {
	Save(ctx context.Context, input []*Referrer, workflowID uuid.UUID) error
	// GetFromRoot returns the referrer identified by the provided content digest, including its first-level references
	// For example if sha:deadbeef represents an attestation, the result will contain the attestation + materials associated to it
	// OrgIDs represent an allowList of organizations where the referrers should be looked for
	GetFromRoot(ctx context.Context, digest string, orgIDS []uuid.UUID, filters ...GetFromRootFilter) (*StoredReferrer, error)
	// Check if a given referrer by digest exist.
	// The query can be scoped further down if needed by providing the kind or visibility status
	Exist(ctx context.Context, digest string, filters ...GetFromRootFilter) (bool, error)
}

type Referrer struct {
	Digest string
	Kind   string
	// Wether the item is downloadable from CAS or not
	Downloadable bool
	// If this referrer is part of a public workflow
	InPublicWorkflow bool
	References       []*Referrer

	Metadata, Annotations map[string]string
}

// Actual referrer stored in the DB which includes a nested list of storedReferences
type StoredReferrer struct {
	*Referrer
	ID        uuid.UUID
	CreatedAt *time.Time
	// Fully expanded list of 1-level off references
	References          []*StoredReferrer
	OrgIDs, WorkflowIDs []uuid.UUID
}

type GetFromRootFilters struct {
	// RootKind is the kind of the root referrer, i.e ATTESTATION
	RootKind *string
	// Wether to filter by visibility or not
	Public *bool
}

type GetFromRootFilter func(*GetFromRootFilters)

func WithKind(kind string) func(*GetFromRootFilters) {
	return func(o *GetFromRootFilters) {
		o.RootKind = &kind
	}
}

func WithPublicVisibility(public bool) func(*GetFromRootFilters) {
	return func(o *GetFromRootFilters) {
		o.Public = &public
	}
}

// ExtractAndPersist extracts the referrers (subject + materials) from the given attestation
// and store it as part of the referrers index table
func (s *ReferrerUseCase) ExtractAndPersist(ctx context.Context, att *dsse.Envelope, workflowID string) error {
	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// Check that the workflow belongs to the organization
	if wf, err := s.workflowRepo.FindByID(ctx, workflowUUID); err != nil {
		return fmt.Errorf("finding workflow: %w", err)
	} else if wf == nil {
		return NewErrNotFound("workflow")
	}

	referrers, err := extractReferrers(att, s.repo)
	if err != nil {
		return fmt.Errorf("extracting referrers: %w", err)
	}

	if err := s.repo.Save(ctx, referrers, workflowUUID); err != nil {
		return fmt.Errorf("saving referrers: %w", err)
	}

	return nil
}

// GetFromRootUser returns the referrer identified by the provided content digest, including its first-level references
// For example if sha:deadbeef represents an attestation, the result will contain the attestation + materials associated to it
// It only returns referrers that belong to organizations the user is member of
func (s *ReferrerUseCase) GetFromRootUser(ctx context.Context, digest, rootKind, userID string) (*StoredReferrer, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// We pass the list of organizationsIDs from where to look for the referrer
	// For now we just pass the list of organizations the user is member of
	// in the future we will expand this to publicly available orgs and so on.
	memberships, err := s.membershipRepo.FindByUser(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("finding memberships: %w", err)
	}

	orgIDs := make([]uuid.UUID, 0, len(memberships))
	for _, m := range memberships {
		orgIDs = append(orgIDs, m.OrganizationID)
	}

	return s.GetFromRoot(ctx, digest, rootKind, orgIDs)
}

func (s *ReferrerUseCase) GetFromRoot(ctx context.Context, digest, rootKind string, orgIDs []uuid.UUID) (*StoredReferrer, error) {
	filters := make([]GetFromRootFilter, 0)
	if rootKind != "" {
		filters = append(filters, WithKind(rootKind))
	}

	ref, err := s.repo.GetFromRoot(ctx, digest, orgIDs, filters...)
	if err != nil {
		if errors.As(err, &ErrAmbiguousReferrer{}) {
			return nil, NewErrValidation(fmt.Errorf("please provide the referrer kind: %w", err))
		}

		return nil, fmt.Errorf("getting referrer from root: %w", err)
	} else if ref == nil {
		return nil, NewErrNotFound("referrer")
	}

	return ref, nil
}

// Get the list of public referrers from organizations
// that have been allowed to be shown in a shared index
// NOTE: This is a public endpoint under /discover/[sha256:deadbeef]
func (s *ReferrerUseCase) GetFromRootInPublicSharedIndex(ctx context.Context, digest, rootKind string) (*StoredReferrer, error) {
	if s.indexConfig == nil || !s.indexConfig.Enabled {
		return nil, NewErrUnauthorizedStr("shared referrer index functionality is not enabled")
	}

	// Load the organizations that are allowed to appear in the shared index
	orgIDs := make([]uuid.UUID, 0)
	for _, orgID := range s.indexConfig.AllowedOrgs {
		orgUUID, err := uuid.Parse(orgID)
		if err != nil {
			return nil, NewErrInvalidUUID(err)
		}
		orgIDs = append(orgIDs, orgUUID)
	}

	// and ask only for the public referrers of those orgs
	filters := []GetFromRootFilter{WithPublicVisibility(true)}
	if rootKind != "" {
		filters = append(filters, WithKind(rootKind))
	}

	ref, err := s.repo.GetFromRoot(ctx, digest, orgIDs, filters...)
	if err != nil {
		if errors.As(err, &ErrAmbiguousReferrer{}) {
			return nil, NewErrValidation(fmt.Errorf("please provide the referrer kind: %w", err))
		}

		return nil, fmt.Errorf("getting referrer from root: %w", err)
	} else if ref == nil {
		return nil, NewErrNotFound("referrer")
	}

	return ref, nil
}

const (
	referrerAttestationType = "ATTESTATION"
	referrerGitHeadType     = "GIT_HEAD_COMMIT"
)

func newRef(digest, kind string) string {
	return fmt.Sprintf("%s-%s", kind, digest)
}

func (r *Referrer) MapID() string {
	return newRef(r.Digest, r.Kind)
}

// ExtractReferrers extracts the referrers from the given attestation
// this means
// 1 - write an entry for the attestation itself
// 2 - then to all the materials contained in the predicate
// 3 - and the subjects (some of them)
// 4 - creating link between the attestation and the materials/subjects as needed
// see tests for examples
func extractReferrers(att *dsse.Envelope, repo ReferrerRepo) ([]*Referrer, error) {
	_, h, err := attestation.JSONEnvelopeWithDigest(att)
	if err != nil {
		return nil, fmt.Errorf("marshaling attestation: %w", err)
	}

	referrersMap := make(map[string]*Referrer)
	// 1 - Attestation referrer
	// Add the attestation itself as a referrer to the map without references yet
	attestationHash := h.String()
	attestationReferrer := &Referrer{
		Digest:       attestationHash,
		Kind:         referrerAttestationType,
		Downloadable: true,
	}

	referrersMap[newRef(attestationHash, referrerAttestationType)] = attestationReferrer

	// 2 - Predicate that's referenced from the attestation
	predicate, err := chainloop.ExtractPredicate(att)
	if err != nil {
		return nil, fmt.Errorf("extracting predicate: %w", err)
	}

	// We currently only support adding additional information about the attestation kind
	// We add both annotations and workflow metadata
	attestationReferrer.Annotations = predicate.GetAnnotations()
	attestationReferrer.Metadata = map[string]string{
		// workflow name, team and project
		"name":         predicate.GetMetadata().Name,
		"team":         predicate.GetMetadata().Team,
		"project":      predicate.GetMetadata().Project,
		"organization": predicate.GetMetadata().Organization,
	}

	// Create new referrers for each material
	// and link them to the attestation
	for _, material := range predicate.GetMaterials() {
		// Skip materials that don't have a digest
		if material.Hash == nil {
			continue
		}

		// If we are inserting an attestation as a dependent, we want to make sure it already exists
		// stored in the system. This is so we can ensure that the attestations nodes are created through
		// an attestation process, not as a referenced provided by the user
		if material.Type == referrerAttestationType {
			if exists, err := repo.Exist(context.Background(), material.Hash.String(), WithKind(referrerAttestationType)); err != nil {
				return nil, fmt.Errorf("checking if attestation exists: %w", err)
			} else if !exists {
				return nil, fmt.Errorf("attestation material does not exist %q", material.Hash.String())
			}
		}

		// Create its referrer entry if it doesn't exist yet
		// the reason it might exist is because you might be attaching the same material twice
		// i.e the same SBOM twice, in that case we don't want to create a new referrer
		materialRef := newRef(material.Hash.String(), material.Type)
		if _, ok := referrersMap[materialRef]; ok {
			continue
		}

		referrersMap[materialRef] = &Referrer{
			Digest:       material.Hash.String(),
			Kind:         material.Type,
			Downloadable: material.UploadedToCAS,
		}

		materialReferrer := referrersMap[materialRef]

		// We create a bidirectional link between the attestation and the material
		// material -> attestation (1-1)
		materialReferrer.References = []*Referrer{{Digest: attestationReferrer.Digest, Kind: attestationReferrer.Kind}}
		// attestation -> material (1-N)
		attestationReferrer.References = append(attestationReferrer.References, &Referrer{
			Digest: materialReferrer.Digest, Kind: materialReferrer.Kind,
		})
	}

	// 3 - Subject that points to the attestation
	statement, err := chainloop.ExtractStatement(att)
	if err != nil {
		return nil, fmt.Errorf("extracting predicate: %w", err)
	}

	// Materials can also be subjects, but there are cases that we will have a subject that is not a material
	// For example, a git head commit, that's why we need to also add the subjects as referrers (if needed)
	for _, subject := range statement.Subject {
		subjectReferrer, err := intotoSubjectToReferrer(subject)
		if err != nil {
			return nil, fmt.Errorf("transforming subject to referrer: %w", err)
		}

		if subjectReferrer == nil {
			continue
		}

		subjectRef := newRef(subjectReferrer.Digest, subjectReferrer.Kind)

		// check if we already have a referrer for this digest and skip if it's the case
		if _, ok := referrersMap[subjectRef]; ok {
			continue
		}

		// We are now in the case where a subject is not a material, i.e a git head commit, we need to add it to the referrers
		// with a bidirectional link to the attestation like we did for the materials
		referrersMap[subjectRef] = subjectReferrer
		// add it to the list of of attestation-referenced digests
		attestationReferrer.References = append(attestationReferrer.References,
			&Referrer{
				Digest: subjectReferrer.Digest, Kind: subjectReferrer.Kind,
			})

		// Update referrer to point to the attestation
		referrersMap[subjectRef].References = []*Referrer{{Digest: attestationReferrer.Digest, Kind: attestationReferrer.Kind}}
	}

	// Return a sorted list of referrers
	mapKeys := make([]string, 0, len(referrersMap))
	for k := range referrersMap {
		mapKeys = append(mapKeys, k)
	}
	sort.Strings(mapKeys)

	referrers := make([]*Referrer, 0, len(referrersMap))
	for _, k := range mapKeys {
		referrers = append(referrers, referrersMap[k])
	}

	return referrers, nil
}

// transforms the in-toto subject to a referrer by deterministically picking
// the subject types we care about (and return nil otherwise), for now we just care about the subjects
// - git.Head and
// - material types
func intotoSubjectToReferrer(r *v1.ResourceDescriptor) (*Referrer, error) {
	var digestStr string
	for alg, val := range r.Digest {
		digestStr = fmt.Sprintf("%s:%s", alg, val)
		break
	}

	// it's a.git head type
	if r.Name == chainloop.SubjectGitHead {
		if digestStr == "" {
			return nil, fmt.Errorf("no digest found for subject %s", r.Name)
		}

		return &Referrer{
			Digest: digestStr,
			Kind:   referrerGitHeadType,
		}, nil
	}

	// Iterate on material types
	var materialType string
	var uploadedToCAS bool
	// it's a material type
	for k, v := range r.Annotations.AsMap() {
		// It's a material type
		if k == chainloop.AnnotationMaterialType {
			materialType = v.(string)
		} else if k == chainloop.AnnotationMaterialCAS {
			uploadedToCAS = v.(bool)
		}
	}

	// it's not a material type
	if materialType == "" {
		return nil, nil
	}

	if digestStr == "" {
		return nil, fmt.Errorf("no digest found for subject %s", r.Name)
	}

	return &Referrer{
		Digest:       digestStr,
		Kind:         materialType,
		Downloadable: uploadedToCAS,
	}, nil
}
