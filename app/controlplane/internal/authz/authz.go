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

// Authorization package
package authz

import (
	"context"
	"errors"
	"fmt"

	_ "embed"

	psqlwatcher "github.com/IguteChung/casbin-psql-watcher"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"

	entadapter "github.com/casbin/ent-adapter"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
)

const (
	// Actions
	ActionRead   = "read"
	ActionList   = "list"
	ActionUpdate = "update"
	ActionDelete = "delete"

	// Resources
	ResourceWorkflowContract      = "workflow_contract"
	ResourceCASArtifact           = "cas_artifact"
	ResourceCASBackend            = "cas_backend"
	ResourceReferrer              = "referrer"
	ResourceAvailableIntegration  = "integration_available"
	ResourceRegisteredIntegration = "integration_registered"
	ResourceAttachedIntegration   = "integration_attached"
	ResourceOrgMetric             = "metrics_org"
	ResourceRobotAccount          = "robot_account"
	ResourceWorkflowRun           = "workflow_run"
	ResourceWorkflow              = "workflow"

	// Roles
	RoleViewer = "role:viewer"
)

// List of policies for each role
// NOTE: roles are hierarchical, this means that the Admin Role can inherit all the policies from the Viewer Role
// so we do not need to add them as well.
var rolesMap = map[string][]*Policy{
	RoleViewer: {
		// Referrer
		PolicyReferrerRead,
		// Artifact
		PolicyArtifactDownload,
		// CAS backend
		PolicyCASBackendList,
		// Available integrations
		PolicyAvailableIntegrationList,
		PolicyAvailableIntegrationRead,
		// Registered integrations
		PolicyRegisteredIntegrationList,
		// Attached integrations
		PolicyAttachedIntegrationList,
		// Metrics
		PolicyOrgMetricsRead,
		// Robot Account
		PolicyRobotAccountList,
		// Workflow Contract
		PolicyWorkflowContractList,
		PolicyWorkflowContractRead,
		// WorkflowRun
		PolicyWorkflowRunList,
		PolicyWorkflowRunRead,
		// Workflow
		PolicyWorkflowList,
	},
}

// resource, action tuple
type Policy struct {
	Resource string
	Action   string
}

var (
	// Referrer
	PolicyReferrerRead = &Policy{ResourceReferrer, ActionRead}
	// Artifact
	PolicyArtifactDownload = &Policy{ResourceCASArtifact, ActionRead}
	// CAS backend
	PolicyCASBackendList = &Policy{ResourceCASBackend, ActionList}
	// Available integrations
	PolicyAvailableIntegrationList = &Policy{ResourceAvailableIntegration, ActionList}
	PolicyAvailableIntegrationRead = &Policy{ResourceAvailableIntegration, ActionRead}
	// Registered integrations
	PolicyRegisteredIntegrationList = &Policy{ResourceRegisteredIntegration, ActionList}
	// Attached integrations
	PolicyAttachedIntegrationList = &Policy{ResourceAttachedIntegration, ActionList}
	// Org Metrics
	PolicyOrgMetricsRead = &Policy{ResourceOrgMetric, ActionList}
	// Robot Account
	PolicyRobotAccountList = &Policy{ResourceRobotAccount, ActionList}
	// Workflow Contract
	PolicyWorkflowContractList   = &Policy{ResourceWorkflowContract, ActionList}
	PolicyWorkflowContractRead   = &Policy{ResourceWorkflowContract, ActionRead}
	PolicyWorkflowContractUpdate = &Policy{ResourceWorkflowContract, ActionUpdate}
	// WorkflowRun
	PolicyWorkflowRunList = &Policy{ResourceWorkflowRun, ActionList}
	PolicyWorkflowRunRead = &Policy{ResourceWorkflowRun, ActionRead}
	// Workflow
	PolicyWorkflowList = &Policy{ResourceWorkflow, ActionList}
)

type SubjectAPIToken struct {
	ID string
}

func (t *SubjectAPIToken) String() string {
	return fmt.Sprintf("api-token:%s", t.ID)
}

//go:embed model.conf
var modelFile []byte

type Enforcer struct {
	*casbin.Enforcer
}

func (e *Enforcer) AddPolicies(sub *SubjectAPIToken, policies ...*Policy) error {
	if len(policies) == 0 {
		return errors.New("no policies to add")
	}

	if sub == nil {
		return errors.New("no subject provided")
	}

	for _, p := range policies {
		casbinPolicy := []string{sub.String(), p.Resource, p.Action}
		// Add policies one by one to skip existing ones.
		// This is because the bulk method AddPoliciesEx does not work well with the ent adapter
		if _, err := e.AddPolicy(casbinPolicy); err != nil {
			return fmt.Errorf("failed to add policy: %w", err)
		}
	}

	return nil
}

// Remove all the policies for the given subject
func (e *Enforcer) ClearPolicies(sub *SubjectAPIToken) error {
	if sub == nil {
		return errors.New("no subject provided")
	}

	// Get all the policies for the subject
	policies := e.GetFilteredPolicy(0, sub.String())

	if _, err := e.Enforcer.RemovePolicies(policies); err != nil {
		return fmt.Errorf("failed to remove policies: %w", err)
	}

	return nil
}

// NewDatabaseEnforcer creates a new casbin authorization enforcer
// based on a database backend as policies storage backend
func NewDatabaseEnforcer(c *conf.Data_Database) (*Enforcer, error) {
	// policy storage in database
	a, err := entadapter.NewAdapter(c.Driver, c.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	e, err := newEnforcer(a)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	// watch for policy changes in database and update enforcer
	w, err := psqlwatcher.NewWatcherWithConnString(context.Background(), c.Source, psqlwatcher.Option{})
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	if err = e.SetWatcher(w); err != nil {
		return nil, fmt.Errorf("failed to set watcher: %w", err)
	}

	if err = w.SetUpdateCallback(func(string) {
		// When there is a change in the policy, we load the in-memory policy for the current enforcer
		if err := e.LoadPolicy(); err != nil {
			fmt.Printf("failed to load policy: %v", err)
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to set update callback: %w", err)
	}

	return e, nil
}

// NewFileAdapter creates a new casbin authorization enforcer
// based on a CSV file as policies storage backend
func NewFiletypeEnforcer(path string) (*Enforcer, error) {
	// policy storage in filesystem
	a := fileadapter.NewAdapter(path)
	e, err := newEnforcer(a)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	return e, nil
}

// NewEnforcer creates a new casbin authorization enforcer for the policies stored
// in the database and the model defined in model.conf
func newEnforcer(a persist.Adapter) (*Enforcer, error) {
	// load model defined in model.conf
	m, err := model.NewModelFromString(string(modelFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	// create enforcer for authorization
	enforcer, err := casbin.NewEnforcer(m, a)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	// Initialize the enforcer with the roles map
	if err := syncRBACRoles(&Enforcer{enforcer}); err != nil {
		return nil, fmt.Errorf("failed to sync roles: %w", err)
	}

	return &Enforcer{enforcer}, nil
}

// Load the roles map into the enforcer
// This is done by adding all the policies defined in the roles map
// and removing all the policies that are not
func syncRBACRoles(e *Enforcer) error {
	// Add all the defined policies if they don't exist
	for role, policies := range rolesMap {
		for _, p := range policies {
			// Add policies one by one to skip existing ones.
			// This is because the bulk method AddPoliciesEx does not work well with the ent adapter
			casbinPolicy := []string{role, p.Resource, p.Action}
			_, err := e.AddPolicy(casbinPolicy)
			if err != nil {
				return fmt.Errorf("failed to add policy: %w", err)
			}
		}
	}

	// Delete all the policies that are not in the roles map
	// 1 - load the policies from the enforcer DB
	for _, gotPolicies := range e.GetPolicy() {
		role := gotPolicies[0]
		policy := &Policy{Resource: gotPolicies[1], Action: gotPolicies[2]}

		// Check if they exist in the map and if they don't, remove them
		wantPolicies, ok := rolesMap[role]
		if !ok {
			continue
		}

		// Check if the policy is in the map
		found := false
		for _, p := range wantPolicies {
			if p.Resource == policy.Resource && p.Action == policy.Action {
				found = true
				break
			}
		}

		// If the policy is not in the map, we remove it
		if !found {
			_, err := e.RemovePolicy(gotPolicies)
			if err != nil {
				return fmt.Errorf("failed to remove policy: %w", err)
			}
		}
	}

	return nil
}
