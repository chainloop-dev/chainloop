//
// Copyright 2026 The Chainloop Authors.
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

package action

import (
	"context"
	"errors"
	"testing"

	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeTraceWorkflowAPI records the calls ensureTraceWorkflow makes and returns
// canned answers, so the decision logic is testable without a control plane.
type fakeTraceWorkflowAPI struct {
	viewResult *WorkflowItem
	viewErr    error

	contractFound bool
	contractErr   error

	createErr error

	contractCalls int
	createCalls   int
	createdWith   string
}

func (f *fakeTraceWorkflowAPI) viewWorkflow(_ context.Context, _, _ string) (*WorkflowItem, error) {
	return f.viewResult, f.viewErr
}

func (f *fakeTraceWorkflowAPI) contractExists(_ context.Context, _ string) (bool, error) {
	f.contractCalls++
	return f.contractFound, f.contractErr
}

// defaultContract stands in for the empty contract the control plane creates
// when a create request carries no contract name.
const defaultContract = "default-contract"

func (f *fakeTraceWorkflowAPI) createWorkflow(_ context.Context, _, _, contract string) (*WorkflowItem, error) {
	f.createCalls++
	f.createdWith = contract
	if f.createErr != nil {
		return nil, f.createErr
	}

	// The control plane echoes back the contract it attached, naming its own
	// default when the request carried none.
	if contract == "" {
		contract = defaultContract
	}

	return &WorkflowItem{ContractName: contract}, nil
}

func TestEnsureTraceWorkflow(t *testing.T) {
	const (
		project  = "my-project"
		workflow = "ai-coding-session"
		contract = "chainloop-ai-coding-session"
	)

	notFound := status.Error(codes.NotFound, "not found")
	const forbidden = "forbidden"
	denied := status.Error(codes.PermissionDenied, forbidden)

	testCases := []struct {
		name string
		api  *fakeTraceWorkflowAPI
		opts EnsureTraceWorkflowOpts

		wantErr          string
		wantCreated      bool
		wantContract     string
		wantContractCall bool
		wantCreateCall   bool
		// wantCreatedWith is the contract name sent on create; empty means the
		// control plane picks its default.
		wantCreatedWith string
	}{
		{
			name:         "existing workflow is left alone",
			api:          &fakeTraceWorkflowAPI{viewResult: &WorkflowItem{ContractName: "some-contract"}},
			opts:         EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: contract},
			wantContract: "some-contract",
		},
		{
			name:             "missing workflow with an existing contract is created attached to it",
			api:              &fakeTraceWorkflowAPI{viewErr: notFound, contractFound: true},
			opts:             EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: contract},
			wantCreated:      true,
			wantContract:     contract,
			wantContractCall: true,
			wantCreateCall:   true,
			wantCreatedWith:  contract,
		},
		{
			name:             "missing contract falls back to the control plane default",
			api:              &fakeTraceWorkflowAPI{viewErr: notFound},
			opts:             EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: contract},
			wantCreated:      true,
			wantContract:     defaultContract,
			wantContractCall: true,
			wantCreateCall:   true,
		},
		{
			name:             "a contract lookup failure also falls back",
			api:              &fakeTraceWorkflowAPI{viewErr: notFound, contractErr: denied},
			opts:             EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: contract},
			wantCreated:      true,
			wantContract:     defaultContract,
			wantContractCall: true,
			wantCreateCall:   true,
		},
		{
			name:             "a required contract that does not exist fails",
			api:              &fakeTraceWorkflowAPI{viewErr: notFound},
			opts:             EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: "typo", ContractRequired: true},
			wantErr:          `contract "typo" does not exist`,
			wantContractCall: true,
		},
		{
			name:             "a required contract that cannot be looked up fails",
			api:              &fakeTraceWorkflowAPI{viewErr: notFound, contractErr: denied},
			opts:             EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: "typo", ContractRequired: true},
			wantErr:          forbidden,
			wantContractCall: true,
		},
		{
			name:           "no contract requested skips the lookup",
			api:            &fakeTraceWorkflowAPI{viewErr: notFound},
			opts:           EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow},
			wantCreated:    true,
			wantContract:   defaultContract,
			wantCreateCall: true,
		},
		{
			name:             "a workflow created concurrently is not an error",
			api:              &fakeTraceWorkflowAPI{viewErr: notFound, contractFound: true, createErr: status.Error(codes.AlreadyExists, "already exists")},
			opts:             EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: contract},
			wantContractCall: true,
			wantCreateCall:   true,
			wantCreatedWith:  contract,
		},
		{
			name:    "a view failure other than not-found fails",
			api:     &fakeTraceWorkflowAPI{viewErr: denied},
			opts:    EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: contract},
			wantErr: forbidden,
		},
		{
			name:    "an unauthenticated view fails",
			api:     &fakeTraceWorkflowAPI{viewErr: status.Error(codes.Unauthenticated, "JWT token is missing")},
			opts:    EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow},
			wantErr: "JWT token is missing",
		},
		{
			name:             "a create failure fails",
			api:              &fakeTraceWorkflowAPI{viewErr: notFound, contractFound: true, createErr: denied},
			opts:             EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: contract},
			wantErr:          forbidden,
			wantContractCall: true,
			wantCreateCall:   true,
			wantCreatedWith:  contract,
		},
		{
			name:    "the project name is required",
			api:     &fakeTraceWorkflowAPI{},
			opts:    EnsureTraceWorkflowOpts{WorkflowName: workflow},
			wantErr: "project and workflow names are required",
		},
		{
			name:    "the workflow name is required",
			api:     &fakeTraceWorkflowAPI{},
			opts:    EnsureTraceWorkflowOpts{ProjectName: project},
			wantErr: "project and workflow names are required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ensureTraceWorkflow(context.Background(), tc.api, zerolog.Nop(), tc.opts)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tc.wantCreated, got.Created)
				assert.Equal(t, tc.wantContract, got.ContractName)
			}

			assert.Equal(t, tc.wantContractCall, tc.api.contractCalls > 0, "contract lookup calls")
			assert.Equal(t, tc.wantCreateCall, tc.api.createCalls > 0, "create calls")
			if tc.wantCreateCall {
				assert.Equal(t, tc.wantCreatedWith, tc.api.createdWith)
			}
		})
	}

	t.Run("errors keep the underlying gRPC status", func(t *testing.T) {
		api := &fakeTraceWorkflowAPI{viewErr: denied}
		_, err := ensureTraceWorkflow(context.Background(), api, zerolog.Nop(), EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow})
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	// Whichever call hits it, an authentication failure has to stay
	// recognizable through the context added on the way up, so the CLI renders
	// its own "run chainloop auth login" message instead of this plumbing.
	t.Run("authentication errors stay recognizable", func(t *testing.T) {
		authErr := jwtMiddleware.ErrMissingJwtToken.GRPCStatus().Err()

		for name, api := range map[string]*fakeTraceWorkflowAPI{
			"on view":     {viewErr: authErr},
			"on contract": {viewErr: notFound, contractErr: authErr},
			"on create":   {viewErr: notFound, contractFound: true, createErr: authErr},
		} {
			t.Run(name, func(t *testing.T) {
				_, err := ensureTraceWorkflow(context.Background(), api, zerolog.Nop(),
					EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: contract, ContractRequired: true})
				require.Error(t, err)

				msg, matched := AuthErrorMessage(err)
				require.True(t, matched)
				assert.Equal(t, `authentication required, please run "chainloop auth login"`, msg)
			})
		}
	})

	t.Run("a non-status contract error still falls back", func(t *testing.T) {
		api := &fakeTraceWorkflowAPI{viewErr: status.Error(codes.NotFound, "not found"), contractErr: errors.New("boom")}
		got, err := ensureTraceWorkflow(context.Background(), api, zerolog.Nop(), EnsureTraceWorkflowOpts{ProjectName: project, WorkflowName: workflow, ContractName: contract})
		require.NoError(t, err)
		assert.True(t, got.Created)
		assert.Equal(t, 1, api.createCalls)
		assert.Empty(t, api.createdWith)
	})
}
