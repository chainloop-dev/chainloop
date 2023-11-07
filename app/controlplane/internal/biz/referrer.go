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
	"fmt"
	"io"
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
	Digest       string
	ArtifactType string
	// Wether the item is downloadable from CAS or not
	Downloadable bool
	// points to other digests
	References []string
}

// Actual referrer stored in the DB which includes a nested list of storedReferences
type StoredReferrer struct {
	ID           uuid.UUID
	Digest       string
	ArtifactType string
	// Wether the item is downloadable from CAS or not
	Downloadable bool
	CreatedAt    *time.Time
	// Fully expanded list of 1-level off references
	References []*StoredReferrer
	OrgIDs     []uuid.UUID
}

type ReferrerMap map[string]*Referrer

type ReferrerRepo interface {
	Save(ctx context.Context, input ReferrerMap, orgID uuid.UUID) error
	GetFromRoot(ctx context.Context, digest string) (*StoredReferrer, error)
}

type ReferrerUseCase struct {
	repo    ReferrerRepo
	orgRepo OrganizationRepo
	logger  *log.Helper
}

func NewReferrerUseCase(repo ReferrerRepo, orgRepo OrganizationRepo, l log.Logger) *ReferrerUseCase {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	return &ReferrerUseCase{repo, orgRepo, servicelogger.ScopedHelper(l, "biz/Referrer")}
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
func (s *ReferrerUseCase) GetFromRoot(ctx context.Context, digest string) (*StoredReferrer, error) {
	ref, err := s.repo.GetFromRoot(ctx, digest)
	if err != nil {
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

// ExtractReferrers extracts the referrers from the given attestation
// this means
// 1 - write an entry for the attestation itself
// 2 - then to all the materials contained in the predicate
// 3 - and the subjects (some of them)
// 4 - creating link between the attestation and the materials/subjects as needed
// see tests for examples
func extractReferrers(att *dsse.Envelope) (ReferrerMap, error) {
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

	referrers := make(ReferrerMap)
	// 1 - Attestation referrer
	// Add the attestation itself as a referrer to the map without references yet
	attestationHash := h.String()
	referrers[attestationHash] = &Referrer{
		Digest:       attestationHash,
		ArtifactType: referrerAttestationType,
		Downloadable: true,
	}

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
		// If we are providing different types for the same digest, we should error out
		if r, ok := referrers[material.Hash.String()]; ok {
			if r.ArtifactType != material.Type {
				return nil, fmt.Errorf("material %s has different types: %s and %s", material.Hash.String(), r.ArtifactType, material.Type)
			}

			continue
		}

		referrers[material.Hash.String()] = &Referrer{
			Digest:       material.Hash.String(),
			ArtifactType: material.Type,
			Downloadable: material.UploadedToCAS,
		}

		// Add the reference to the attestation
		referrers[attestationHash].References = append(referrers[attestationHash].References, material.Hash.String())
	}

	// 3 - Subject that points to the attestation
	statement, err := chainloop.ExtractStatement(att)
	if err != nil {
		return nil, fmt.Errorf("extracting predicate: %w", err)
	}

	for _, subject := range statement.Subject {
		subjectRef, err := intotoSubjectToReferrer(subject)
		if err != nil {
			return nil, fmt.Errorf("transforming subject to referrer: %w", err)
		}

		if subjectRef == nil {
			continue
		}

		// check if we already have a referrer for this digest and set it otherwise
		// this is the case for example for git.Head ones
		if _, ok := referrers[subjectRef.Digest]; !ok {
			referrers[subjectRef.Digest] = subjectRef
			// add it to the list of of attestation-referenced digests
			referrers[attestationHash].References = append(referrers[attestationHash].References, subjectRef.Digest)
		}

		// Update referrer to point to the attestation
		referrers[subjectRef.Digest].References = []string{attestationHash}
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
			Digest:       digestStr,
			ArtifactType: referrerGitHeadType,
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
		ArtifactType: materialType,
		Downloadable: uploadedToCAS,
	}, nil
}
