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

package data

import (
	"context"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/referrer"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type ReferrerRepo struct {
	data *Data
	log  *log.Helper
}

func NewReferrerRepo(data *Data, logger log.Logger) biz.ReferrerRepo {
	return &ReferrerRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

type storedReferrerMap map[string]*ent.Referrer

func (r *ReferrerRepo) Save(ctx context.Context, input biz.ReferrerMap, orgID uuid.UUID) error {
	// Start transaction
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	storedMap := make(storedReferrerMap)
	// 1 - Find or create each referrer
	for id, r := range input {
		// Check if it exists already, if not create it
		storedRef, err := tx.Referrer.Query().Where(referrer.Digest(r.Digest), referrer.Kind(r.Kind)).Only(ctx)
		if err != nil {
			if !ent.IsNotFound(err) {
				return fmt.Errorf("failed to query referrer: %w", err)
			}

			storedRef, err = tx.Referrer.Create().
				SetDigest(r.Digest).SetKind(r.Kind).SetDownloadable(r.Downloadable).AddOrganizationIDs(orgID).Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to create referrer: %w", err)
			}
		}

		// associate it with the organization
		storedRef, err = storedRef.Update().AddOrganizationIDs(orgID).Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to add organization to referrer: %w", err)
		}

		// Store it in the map
		storedMap[id] = storedRef
	}

	// 2 - define the relationship between referrers
	for id, inputRef := range input {
		// This is the current item stored in DB
		storedReferrer := storedMap[id]
		// Iterate on the items it refer to (references)
		for _, ref := range inputRef.References {
			// amd find it in the DB
			storedReference, ok := storedMap[id]
			if !ok {
				return fmt.Errorf("referrer %s not found", ref)
			}

			// Create the relationship
			_, err := storedReferrer.Update().AddReferenceIDs(storedReference.ID).Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to create referrer relationship: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *ReferrerRepo) GetFromRoot(ctx context.Context, digest string, orgIDs []uuid.UUID) (*biz.StoredReferrer, error) {
	// Find the referrer recursively starting from the root
	res, err := r.doGet(ctx, digest, orgIDs, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get referrer: %w", err)
	}

	return res, nil
}

// max number of recursive levels to traverse
// we just care about 1 level, i.e att -> commit, or commit -> attestation
// we also need to limit this because there might be cycles
const maxTraverseLevels = 1

func (r *ReferrerRepo) doGet(ctx context.Context, digest string, orgIDs []uuid.UUID, level int) (*biz.StoredReferrer, error) {
	// Find the referrer
	// if there is more than 1 item with the same digest+artifactType it will fail
	ref, err := r.data.db.Referrer.Query().Where(referrer.Digest(digest)).
		Where(referrer.HasOrganizationsWith(organization.IDIn(orgIDs...))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		} else if ent.IsNotSingular(err) {
			return nil, fmt.Errorf("found more than one referrer with digest %s, please provide artifact type", digest)
		}

		return nil, fmt.Errorf("failed to query referrer: %w", err)
	}

	// Assemble the referrer to return
	res := &biz.StoredReferrer{
		ID:           ref.ID,
		CreatedAt:    toTimePtr(ref.CreatedAt),
		Digest:       ref.Digest,
		Kind:         ref.Kind,
		Downloadable: ref.Downloadable,
	}

	// with all the organizationIDs attached
	res.OrgIDs, err = ref.QueryOrganizations().IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query organizations: %w", err)
	}

	// Next: We'll find the references recursively up to a max of maxTraverseLevels levels
	if level >= maxTraverseLevels {
		return res, nil
	}

	// Find the references and call recursively
	refs, err := ref.QueryReferences().
		Where(referrer.HasOrganizationsWith(organization.IDIn(orgIDs...))).
		Order(referrer.ByDigest()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query references: %w", err)
	}

	// Add the references to the result
	for _, reference := range refs {
		// Call recursively the function
		ref, err := r.doGet(ctx, reference.Digest, orgIDs, level+1)
		if err != nil {
			return nil, fmt.Errorf("failed to get referrer: %w", err)
		}

		res.References = append(res.References, ref)
	}

	return res, nil
}
