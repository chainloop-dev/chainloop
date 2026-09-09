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
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EnsureTraceWorkflowOpts identifies the workflow `chainloop trace` attests to
// and the contract it should be attached to when it has to be created.
type EnsureTraceWorkflowOpts struct {
	ProjectName  string
	WorkflowName string
	// ContractName is the contract to attach when the workflow is created.
	// Empty, or a contract the organization does not have, leaves the control
	// plane to create its default one.
	ContractName string
	// ContractRequired fails the call when ContractName cannot be found instead
	// of falling back to that default. Set it when the name came from the user
	// (--contract), where a typo must not silently bind another contract.
	ContractRequired bool
}

// TraceWorkflowResult reports what EnsureWorkflow did.
type TraceWorkflowResult struct {
	// Created is true when this call created the workflow.
	Created bool
	// ContractName is the contract the workflow is attached to. It is empty
	// when the workflow was created concurrently by someone else, since the
	// call does not read it back.
	ContractName string
}

// traceWorkflowAPI is the slice of the control-plane API that
// ensureTraceWorkflow drives, so the decision logic can be tested without a
// live control plane.
type traceWorkflowAPI interface {
	// viewWorkflow returns the workflow, or a NotFound error when the workflow
	// or its project does not exist.
	viewWorkflow(ctx context.Context, project, workflow string) (*WorkflowItem, error)
	// findContract returns the contract with that name, or nil when the
	// organization does not have one. A lookup failure is returned as an error,
	// not as a missing contract.
	findContract(ctx context.Context, name string) (*WorkflowContractItem, error)
	// createWorkflow creates the workflow, attached to contract when non-empty.
	createWorkflow(ctx context.Context, project, workflow, contract string) (*WorkflowItem, error)
}

// EnsureWorkflow makes sure the workflow trace attestations target exists,
// creating it attached to opts.ContractName when the organization has that
// contract. `chainloop trace` calls it so the contract decision happens once,
// with a human watching, instead of on the push path: the pre-push hook would
// otherwise let the control plane create the workflow implicitly with an empty
// contract, with nobody reading the output.
//
// It runs over the executor's connection, so it targets the organization pinned
// with WithForcedOrganization like every other trace operation.
func (e *AttestationExecutor) EnsureWorkflow(ctx context.Context, opts EnsureTraceWorkflowOpts) (*TraceWorkflowResult, error) {
	return ensureTraceWorkflow(ctx, &cpTraceWorkflowAPI{cfg: e.actionOpts}, e.actionOpts.Logger, opts)
}

func ensureTraceWorkflow(ctx context.Context, api traceWorkflowAPI, log zerolog.Logger, opts EnsureTraceWorkflowOpts) (*TraceWorkflowResult, error) {
	if opts.ProjectName == "" || opts.WorkflowName == "" {
		return nil, fmt.Errorf("project and workflow names are required")
	}

	wf, err := api.viewWorkflow(ctx, opts.ProjectName, opts.WorkflowName)
	if err == nil {
		log.Debug().Str("workflow", opts.WorkflowName).Str("contract", wf.ContractName).Msg("trace workflow already exists")
		return &TraceWorkflowResult{ContractName: wf.ContractName}, nil
	}
	if status.Code(err) != codes.NotFound {
		return nil, fmt.Errorf("looking up workflow %q in project %q: %w", opts.WorkflowName, opts.ProjectName, err)
	}

	contract, err := resolveTraceContract(ctx, api, log, opts)
	if err != nil {
		return nil, err
	}

	created, err := api.createWorkflow(ctx, opts.ProjectName, opts.WorkflowName, contract)
	if err != nil {
		// Someone else created it in the meantime, which is all this call
		// promises. Their contract choice stands.
		if status.Code(err) == codes.AlreadyExists {
			log.Debug().Str("workflow", opts.WorkflowName).Msg("trace workflow was created concurrently")
			return &TraceWorkflowResult{}, nil
		}

		return nil, fmt.Errorf("creating workflow %q in project %q: %w", opts.WorkflowName, opts.ProjectName, err)
	}

	return &TraceWorkflowResult{Created: true, ContractName: created.ContractName}, nil
}

// resolveTraceContract returns the contract name to attach on creation: the
// requested one when the organization has it and the project may use it, or an
// empty string to let the control plane create its per-project default. A
// contract that cannot be used is only an error when the caller marked it
// required, because the shipped default is absent in organizations that never
// imported it, and a contract of the user's own may happen to carry its name.
func resolveTraceContract(ctx context.Context, api traceWorkflowAPI, log zerolog.Logger, opts EnsureTraceWorkflowOpts) (string, error) {
	if opts.ContractName == "" {
		return "", nil
	}

	contract, err := api.findContract(ctx, opts.ContractName)
	switch {
	case err != nil && opts.ContractRequired:
		return "", fmt.Errorf("looking up contract %q: %w", opts.ContractName, err)
	case err != nil:
		log.Debug().Err(err).Str("contract", opts.ContractName).Msg("could not look up the contract; falling back to the default one")
		return "", nil
	case contract == nil && opts.ContractRequired:
		return "", fmt.Errorf("contract %q does not exist in this organization", opts.ContractName)
	case contract == nil:
		log.Debug().Str("contract", opts.ContractName).Msg("contract not found; falling back to the default one")
		return "", nil
	}

	// A contract scoped to a project belongs to that project alone: the control
	// plane refuses to attach it anywhere else, comparing project names. The
	// name is all we have to go on here, and it is the same thing it compares.
	if scope := contract.ScopedEntity; scope != nil && scope.Type == contractScopeProject && scope.Name != opts.ProjectName {
		if opts.ContractRequired {
			return "", fmt.Errorf("contract %q is scoped to project %q, so it cannot be used in project %q",
				opts.ContractName, scope.Name, opts.ProjectName)
		}

		log.Debug().Str("contract", opts.ContractName).Str("scoped_to", scope.Name).
			Msg("contract belongs to another project; falling back to the default one")

		return "", nil
	}

	return opts.ContractName, nil
}

// contractScopeProject is the scope a contract carries when it belongs to a
// single project rather than to the whole organization, which has none.
const contractScopeProject = "project"

// cpTraceWorkflowAPI implements traceWorkflowAPI against the control plane,
// reusing the actions the equivalent commands run.
type cpTraceWorkflowAPI struct {
	cfg *ActionsOpts
}

func (a *cpTraceWorkflowAPI) viewWorkflow(ctx context.Context, project, workflow string) (*WorkflowItem, error) {
	return NewWorkflowDescribe(a.cfg).Run(ctx, workflow, project)
}

func (a *cpTraceWorkflowAPI) findContract(ctx context.Context, name string) (*WorkflowContractItem, error) {
	described, err := NewWorkflowContractDescribe(a.cfg).Run(ctx, name, 0)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}

		return nil, err
	}

	return described.Contract, nil
}

func (a *cpTraceWorkflowAPI) createWorkflow(ctx context.Context, project, workflow, contract string) (*WorkflowItem, error) {
	return NewWorkflowCreate(a.cfg).Run(ctx, &NewWorkflowCreateOpts{
		Name:         workflow,
		Project:      project,
		ContractName: contract,
	})
}
