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

package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowrun"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type AttestationStateRepo struct {
	data *Data
	log  *log.Helper
}

func NewAttestationStateRepo(data *Data, logger log.Logger) biz.AttestationStateRepo {
	return &AttestationStateRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// find the workflow run by its ID and check that it has attestation state
func (r *AttestationStateRepo) Initialized(ctx context.Context, runID uuid.UUID) (bool, error) {
	exists, err := r.data.DB.WorkflowRun.Query().Where(workflowrun.ID(runID)).Where(workflowrun.AttestationStateNotNil()).Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check attestation state: %w", err)
	}

	return exists, nil
}

// baseDigest, when provided will be used to check that it matches the digest of the state currently in the DB
// if the digests do not match, the state has been modified and the caller should retry
func (r *AttestationStateRepo) Save(ctx context.Context, runID uuid.UUID, state []byte, baseDigest string) error {
	return WithTx(ctx, r.data.DB, func(tx *ent.Tx) error {
		// compared the provided digest with the digest of the state in the DB
		// TODO: make digest check mandatory on updates
		if baseDigest != "" {
			// Get the run but BLOCK IT for update
			run, err := tx.WorkflowRun.Query().ForUpdate().Where(workflowrun.ID(runID)).Only(ctx)
			if err != nil && !ent.IsNotFound(err) {
				return fmt.Errorf("failed to read attestation state: %w", err)
			} else if run == nil || run.AttestationState == nil {
				return biz.NewErrNotFound("attestation state")
			}

			// calculate the digest of the current state
			storedDigest, err := digest(run.AttestationState)
			if err != nil {
				return fmt.Errorf("failed to calculate digest: %w", err)
			}

			if baseDigest != storedDigest {
				return biz.NewErrAttestationStateConflict(storedDigest, baseDigest)
			}
		}

		// Update it in the DB if the digest matches
		err := tx.WorkflowRun.UpdateOneID(runID).SetAttestationState(state).Exec(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return fmt.Errorf("failed to store attestation state: %w", err)
		} else if err != nil {
			return biz.NewErrNotFound("workflow run")
		}
		return nil
	})
}

func (r *AttestationStateRepo) Read(ctx context.Context, runID uuid.UUID) ([]byte, string, error) {
	run, err := r.data.DB.WorkflowRun.Query().Where(workflowrun.ID(runID)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, "", fmt.Errorf("failed to read attestation state: %w", err)
	} else if run == nil || run.AttestationState == nil {
		return nil, "", biz.NewErrNotFound("attestation state")
	}

	// calculate the digest of the state
	digest, err := digest(run.AttestationState)
	if err != nil {
		return nil, "", fmt.Errorf("failed to calculate digest: %w", err)
	}

	return run.AttestationState, digest, nil
}

func (r *AttestationStateRepo) Reset(ctx context.Context, runID uuid.UUID) error {
	err := r.data.DB.WorkflowRun.UpdateOneID(runID).ClearAttestationState().Exec(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("failed to clear attestation state: %w", err)
	} else if err != nil {
		return biz.NewErrNotFound("attestation state")
	}

	return nil
}

func digest(data []byte) (string, error) {
	m, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshalling state: %w", err)
	}

	hash := sha256.Sum256(m)
	return hex.EncodeToString(hash[:]), nil
}
