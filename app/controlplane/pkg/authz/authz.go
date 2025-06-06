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
	_ "embed"
	"errors"
	"fmt"

	psqlwatcher "github.com/IguteChung/casbin-psql-watcher"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	entadapter "github.com/casbin/ent-adapter"
	config "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
)

type Role string

const (
	// Actions
	ActionRead   = "read"
	ActionList   = "list"
	ActionCreate = "create"
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
	UserMembership                = "membership_user"
	Organization                  = "organization"

	// We have for now three roles, viewer, admin and owner
	// The owner of an org
	// The administrator of an org
	// The read only viewer of an org
	// These roles are hierarchical
	// This means that the Owner role inherits all the policies from Admin so from the Viewer Role
	RoleOwner  Role = "role:org:owner"
	RoleAdmin  Role = "role:org:admin"
	RoleViewer Role = "role:org:viewer"
)

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
	PolicyArtifactUpload   = &Policy{ResourceCASArtifact, ActionCreate}
	// CAS backend
	PolicyCASBackendList = &Policy{ResourceCASBackend, ActionList}
	// Available integrations
	PolicyAvailableIntegrationList = &Policy{ResourceAvailableIntegration, ActionList}
	PolicyAvailableIntegrationRead = &Policy{ResourceAvailableIntegration, ActionRead}
	// Registered integrations
	PolicyRegisteredIntegrationList = &Policy{ResourceRegisteredIntegration, ActionList}
	PolicyRegisteredIntegrationRead = &Policy{ResourceRegisteredIntegration, ActionRead}
	PolicyRegisteredIntegrationAdd  = &Policy{ResourceRegisteredIntegration, ActionCreate}
	// Attached integrations
	PolicyAttachedIntegrationList   = &Policy{ResourceAttachedIntegration, ActionList}
	PolicyAttachedIntegrationAttach = &Policy{ResourceAttachedIntegration, ActionCreate}
	// Org Metrics
	PolicyOrgMetricsRead = &Policy{ResourceOrgMetric, ActionList}
	// Robot Account
	PolicyRobotAccountList   = &Policy{ResourceRobotAccount, ActionList}
	PolicyRobotAccountCreate = &Policy{ResourceRobotAccount, ActionCreate}
	// Workflow Contract
	PolicyWorkflowContractList   = &Policy{ResourceWorkflowContract, ActionList}
	PolicyWorkflowContractRead   = &Policy{ResourceWorkflowContract, ActionRead}
	PolicyWorkflowContractUpdate = &Policy{ResourceWorkflowContract, ActionUpdate}
	PolicyWorkflowContractCreate = &Policy{ResourceWorkflowContract, ActionCreate}
	// WorkflowRun
	PolicyWorkflowRunList = &Policy{ResourceWorkflowRun, ActionList}
	PolicyWorkflowRunRead = &Policy{ResourceWorkflowRun, ActionRead}
	// Workflow
	PolicyWorkflowList   = &Policy{ResourceWorkflow, ActionList}
	PolicyWorkflowRead   = &Policy{ResourceWorkflow, ActionRead}
	PolicyWorkflowCreate = &Policy{ResourceWorkflow, ActionCreate}

	// User Membership
	PolicyOrganizationRead = &Policy{Organization, ActionRead}
)

// List of policies for each role
// NOTE: roles are hierarchical, this means that the Admin Role can inherit all the policies from the Viewer Role
// so we do not need to add them as well.
var rolesMap = map[Role][]*Policy{
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
		PolicyWorkflowRead,
		// Organization
		PolicyOrganizationRead,
	},
	RoleAdmin: {
		// We do a manual check in the artifact upload endpoint
		// so we need the actual policy in place skipping it is not enough
		PolicyArtifactUpload,
		// + all the policies from the viewer role inherited automatically
	},
}

// ServerOperationsMap is a map of server operations to the ResourceAction tuples that are
// required to perform the operation
// If it contains more than one policy, all of them need to be true
var ServerOperationsMap = map[string][]*Policy{
	// Discover endpoint
	"/controlplane.v1.ReferrerService/DiscoverPrivate": {PolicyReferrerRead},
	// Download/Uploading artifacts
	// There are no policies for the download endpoint, we do a manual check in the service layer
	// to differentiate between upload and download requests
	"/controlplane.v1.CASCredentialsService/Get": {},
	// We have an endpoint to generate a download URL
	"/controlplane.v1.CASRedirectService/DownloadRedirect": {PolicyArtifactDownload},
	// Or to retrieve a download url
	"/controlplane.v1.CASRedirectService/GetDownloadURL": {PolicyArtifactDownload},
	// CAS Backend listing
	"/controlplane.v1.CASBackendService/List": {PolicyCASBackendList},
	// Available integrations
	"/controlplane.v1.IntegrationsService/ListAvailable": {PolicyAvailableIntegrationList, PolicyAvailableIntegrationRead},
	// Registered integrations
	"/controlplane.v1.IntegrationsService/ListRegistrations":    {PolicyRegisteredIntegrationList},
	"/controlplane.v1.IntegrationsService/DescribeRegistration": {PolicyRegisteredIntegrationRead},
	"/controlplane.v1.IntegrationsService/Register":             {PolicyRegisteredIntegrationAdd},
	// Attached integrations
	"/controlplane.v1.IntegrationsService/ListAttachments": {PolicyAttachedIntegrationList},
	"/controlplane.v1.IntegrationsService/Attach":          {PolicyAttachedIntegrationAttach},
	// Metrics
	"/controlplane.v1.OrgMetricsService/.*": {PolicyOrgMetricsRead},
	// Robot Account
	"/controlplane.v1.RobotAccountService/List":   {PolicyRobotAccountList},
	"/controlplane.v1.RobotAccountService/Create": {PolicyRobotAccountCreate},
	// Workflows
	"/controlplane.v1.WorkflowService/List":   {PolicyWorkflowList},
	"/controlplane.v1.WorkflowService/View":   {PolicyWorkflowRead},
	"/controlplane.v1.WorkflowService/Create": {PolicyWorkflowCreate},
	// WorkflowRun
	"/controlplane.v1.WorkflowRunService/List": {PolicyWorkflowRunList},
	"/controlplane.v1.WorkflowRunService/View": {PolicyWorkflowRunRead},
	// Workflow Contracts
	"/controlplane.v1.WorkflowContractService/List":     {PolicyWorkflowContractList},
	"/controlplane.v1.WorkflowContractService/Describe": {PolicyWorkflowContractRead},
	"/controlplane.v1.WorkflowContractService/Update":   {PolicyWorkflowContractUpdate},
	"/controlplane.v1.WorkflowContractService/Create":   {PolicyWorkflowContractCreate},
	// Get current information about an organization
	"/controlplane.v1.ContextService/Current": {PolicyOrganizationRead},
	// Listing, create or selecting an organization does not have any required permissions,
	// since all the permissions here are in the context of an organization
	// Create new organization
	"/controlplane.v1.OrganizationService/Create": {},
	// NOTE: this is about listing my own memberships, not about listing all the memberships in the organization
	"/controlplane.v1.UserService/ListMemberships": {},
	// Set the current organization for the current user
	"/controlplane.v1.UserService/SetCurrentMembership": {},
	// Leave the organization or delete your account
	"/controlplane.v1.UserService/DeleteMembership": {},
	"/controlplane.v1.AuthService/DeleteAccount":    {},
}

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

func (e *Enforcer) Enforce(sub string, p *Policy) (bool, error) {
	return e.Enforcer.Enforce(sub, p.Resource, p.Action)
}

// Remove all the policies for the given subject
func (e *Enforcer) ClearPolicies(sub *SubjectAPIToken) error {
	if sub == nil {
		return errors.New("no subject provided")
	}

	// Get all the policies for the subject
	policies, err := e.GetFilteredPolicy(0, sub.String())
	if err != nil {
		return fmt.Errorf("failed to get policies: %w", err)
	}

	if _, err := e.Enforcer.RemovePolicies(policies); err != nil {
		return fmt.Errorf("failed to remove policies: %w", err)
	}

	return nil
}

// NewDatabaseEnforcer creates a new casbin authorization enforcer
// based on a database backend as policies storage backend
func NewDatabaseEnforcer(c *config.DatabaseConfig) (*Enforcer, error) {
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
	return doSync(e, rolesMap)
}

func doSync(e *Enforcer, rolesMap map[Role][]*Policy) error {
	// Add all the defined policies if they don't exist
	for role, policies := range rolesMap {
		for _, p := range policies {
			// Add policies one by one to skip existing ones.
			// This is because the bulk method AddPoliciesEx does not work well with the ent adapter
			casbinPolicy := []string{string(role), p.Resource, p.Action}
			_, err := e.AddPolicy(casbinPolicy)
			if err != nil {
				return fmt.Errorf("failed to add policy: %w", err)
			}
		}
	}

	// Delete all the policies that are not in the roles map
	// 1 - load the policies from the enforcer DB
	policies, err := e.GetPolicy()
	if err != nil {
		return fmt.Errorf("failed to get policies: %w", err)
	}

	for _, gotPolicies := range policies {
		role := gotPolicies[0]
		policy := &Policy{Resource: gotPolicies[1], Action: gotPolicies[2]}

		wantPolicies, ok := rolesMap[Role(role)]
		// if the role does not exist in the map, we can delete the policy
		if !ok {
			_, err := e.RemovePolicy(role, policy.Resource, policy.Action)
			if err != nil {
				return fmt.Errorf("failed to remove policy: %w", err)
			}
			continue
		}

		// We have the role in the map, so we now compare the policies
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

	// To finish we make sure that the admin role inherit all the policies from the viewer role
	_, err = e.AddGroupingPolicy(string(RoleAdmin), string(RoleViewer))
	if err != nil {
		return fmt.Errorf("failed to add grouping policy: %w", err)
	}

	// same for the owner
	_, err = e.AddGroupingPolicy(string(RoleOwner), string(RoleAdmin))
	if err != nil {
		return fmt.Errorf("failed to add grouping policy: %w", err)
	}

	return nil
}

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
// so they can be added to the database schema
func (Role) Values() (roles []string) {
	for _, s := range []Role{
		RoleOwner,
		RoleAdmin,
		RoleViewer,
	} {
		roles = append(roles, string(s))
	}

	return
}
