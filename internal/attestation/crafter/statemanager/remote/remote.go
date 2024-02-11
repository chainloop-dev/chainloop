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

package remote

import (
	"context"
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
)

type Remote struct {
	client pb.AttestationStateServiceClient
}

func New(c pb.AttestationStateServiceClient) (*Remote, error) {
	if c == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	return &Remote{c}, nil
}

func (r *Remote) Info(_ context.Context, key string) string {
	return fmt.Sprintf("remote://%s", key)
}

func (r *Remote) Initialized(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("key cannot be empty")
	}

	resp, err := r.client.Initialized(ctx, &pb.AttestationStateServiceInitializedRequest{WorkflowRunId: key})
	if err != nil {
		return false, fmt.Errorf("failed to check state: %w", err)
	}

	return resp.Result.GetInitialized(), nil
}

func (r *Remote) Write(ctx context.Context, key string, state *v1.CraftingState) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	} else if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	if _, err := r.client.Save(ctx, &pb.AttestationStateServiceSaveRequest{WorkflowRunId: key, AttestationState: state}); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

func (r *Remote) Read(ctx context.Context, key string, state *v1.CraftingState) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	} else if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	resp, err := r.client.Read(ctx, &pb.AttestationStateServiceReadRequest{WorkflowRunId: key})
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	*state = *resp.Result.GetAttestationState()
	return nil
}

func (r *Remote) Reset(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	if _, err := r.client.Reset(ctx, &pb.AttestationStateServiceResetRequest{WorkflowRunId: key}); err != nil {
		return fmt.Errorf("failed to reset state: %w", err)
	}

	return nil
}
