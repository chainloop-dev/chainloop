//
// Copyright 2023 The Chainloop Authors.
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/uuid"
	v1 "github.com/in-toto/attestation/go/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type Referrer struct {
	Digest string
	Kind   string
	// Wether the item is downloadable from CAS or not
	Downloadable bool
	References   []*Referrer
}

// Actual referrer stored in the DB which includes a nested list of storedReferences
type StoredReferrer struct {
	*Referrer
	ID        uuid.UUID
	CreatedAt *time.Time
	// Fully expanded list of 1-level off references
	References []*StoredReferrer
	OrgIDs     []uuid.UUID
}

type ReferrerRepo interface {
	Save(ctx context.Context, input []*Referrer, orgID uuid.UUID) error
	// GetFromRoot returns the referrer identified by the provided content digest, including its first-level references
	// For example if sha:deadbeef represents an attestation, the result will contain the attestation + materials associated to it
	// OrgIDs represent an allowList of organizations where the referrers should be looked for
	GetFromRoot(ctx context.Context, digest, kind string, orgIDS []uuid.UUID) (*StoredReferrer, error)
}

type ReferrerUseCase struct {
	repo           ReferrerRepo
	orgRepo        OrganizationRepo
	membershipRepo MembershipRepo
	logger         *log.Helper
}

func NewReferrerUseCase(repo ReferrerRepo, orgRepo OrganizationRepo, mRepo MembershipRepo, l log.Logger) *ReferrerUseCase {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	return &ReferrerUseCase{repo, orgRepo, mRepo, servicelogger.ScopedHelper(l, "biz/Referrer")}
}

// ExtractAndPersist extracts the referrers (subject + materials) from the given attestation
// and store it as part of the referrers index table
func (s *ReferrerUseCase) ExtractAndPersist(ctx context.Context, att *dsse.Envelope, orgID string) error {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	if org, err := s.orgRepo.FindByID(ctx, orgUUID); err != nil {
		return fmt.Errorf("finding organization: %w", err)
	} else if org == nil {
		return NewErrNotFound("organization")
	}

	m, err := extractReferrers(att)
	if err != nil {
		return fmt.Errorf("extracting referrers: %w", err)
	}

	if err := s.repo.Save(ctx, m, orgUUID); err != nil {
		return fmt.Errorf("saving referrers: %w", err)
	}

	return nil
}

// GetFromRoot returns the referrer identified by the provided content digest, including its first-level references
// For example if sha:deadbeef represents an attestation, the result will contain the attestation + materials associated to it
// It only returns referrers that belong to organizations the user is member of
func (s *ReferrerUseCase) GetFromRoot(ctx context.Context, digest string, rootKind, userID string) (*StoredReferrer, error) {
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

	ref, err := s.repo.GetFromRoot(ctx, digest, rootKind, orgIDs)
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
func extractReferrers(att *dsse.Envelope) ([]*Referrer, error) {
	// Calculate the attestation hash
	jsonAtt, err := json.Marshal(att)
	if err != nil {
		return nil, fmt.Errorf("marshaling attestation: %w", err)
	}

	// Calculate the attestation hash
	h, _, err := cr_v1.SHA256(bytes.NewBuffer(jsonAtt))
	if err != nil {
		return nil, fmt.Errorf("calculating attestation hash: %w", err)
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

	// Create new referrers for each material
	// and link them to the attestation
	for _, material := range predicate.GetMaterials() {
		// Skip materials that don't have a digest
		if material.Hash == nil {
			continue
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

		// Add the reference to the attestation
		attestationReferrer.References = append(attestationReferrer.References, &Referrer{
			Digest: materialReferrer.Digest, Kind: materialReferrer.Kind,
		})
	}

	// 3 - Subject that points to the attestation
	statement, err := chainloop.ExtractStatement(att)
	if err != nil {
		return nil, fmt.Errorf("extracting predicate: %w", err)
	}

	for _, subject := range statement.Subject {
		subjectReferrer, err := intotoSubjectToReferrer(subject)
		if err != nil {
			return nil, fmt.Errorf("transforming subject to referrer: %w", err)
		}

		if subjectReferrer == nil {
			continue
		}

		subjectRef := newRef(subjectReferrer.Digest, subjectReferrer.Kind)

		// check if we already have a referrer for this digest and set it otherwise
		// this is the case for example for git.Head ones
		if _, ok := referrersMap[subjectRef]; !ok {
			referrersMap[subjectRef] = subjectReferrer
			// add it to the list of of attestation-referenced digests
			attestationReferrer.References = append(attestationReferrer.References,
				&Referrer{
					Digest: subjectReferrer.Digest, Kind: subjectReferrer.Kind,
				})
		}

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
