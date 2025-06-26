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
	"golang.org/x/exp/slices"
)

type Role string
type Resource string

const (
	// Actions

	ActionRead   = "read"
	ActionList   = "list"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"

	// Resources

	ResourceWorkflowContract      Resource = "workflow_contract"
	ResourceCASArtifact           Resource = "cas_artifact"
	ResourceCASBackend            Resource = "cas_backend"
	ResourceReferrer              Resource = "referrer"
	ResourceAvailableIntegration  Resource = "integration_available"
	ResourceRegisteredIntegration Resource = "integration_registered"
	ResourceAttachedIntegration   Resource = "integration_attached"
	ResourceOrgMetric             Resource = "metrics_org"
	ResourceRobotAccount          Resource = "robot_account"
	ResourceWorkflowRun           Resource = "workflow_run"
	ResourceWorkflow              Resource = "workflow"
	Organization                  Resource = "organization"
	ResourceGroup                 Resource = "group"
	ResourceGroupMembership       Resource = "group_membership"

	// We have for now three roles, viewer, admin and owner
	// The owner of an org
	// The administrator of an org
	// The read only viewer of an org
	// These roles are hierarchical
	// This means that the Owner role inherits all the policies from Admin so from the Viewer Role
	RoleOwner  Role = "role:org:owner"
	RoleAdmin  Role = "role:org:admin"
	RoleViewer Role = "role:org:viewer"

	// New RBAC roles

	// RoleOrgMember is the role that users get by default when they join an organization.
	// They cannot see projects until they are invited. However, they are able to create their own projects,
	// so Casbin rules (role, resource-type, action) are NOT enough to check for permission, since we must check for ownership as well.
	// That last check will be done at the service level.
	RoleOrgMember Role = "role:org:member"

	RoleProjectAdmin  Role = "role:project:admin"
	RoleProjectViewer Role = "role:project:viewer"
)

// AuthzManagedResources are the resources that are managed by Chainloop, considered during permissions sync
var AuthzManagedResources = []Resource{
	ResourceWorkflowContract,
	ResourceCASArtifact,
	ResourceCASBackend,
	ResourceReferrer,
	ResourceAvailableIntegration,
	ResourceRegisteredIntegration,
	ResourceAttachedIntegration,
	ResourceOrgMetric,
	ResourceRobotAccount,
	ResourceWorkflowRun,
	ResourceWorkflow,
	Organization,
	ResourceGroup,
	ResourceGroupMembership,
}

// resource, action tuple
type Policy struct {
	Resource string
	Action   string
}

func NewPolicy(r Resource, action string) *Policy {
	return &Policy{
		Resource: string(r),
		Action:   action,
	}
}

var (
	// Referrer
	PolicyReferrerRead = NewPolicy(ResourceReferrer, ActionRead)
	// Artifact
	PolicyArtifactDownload = NewPolicy(ResourceCASArtifact, ActionRead)
	PolicyArtifactUpload   = NewPolicy(ResourceCASArtifact, ActionCreate)
	// CAS backend
	PolicyCASBackendList = NewPolicy(ResourceCASBackend, ActionList)
	// Available integrations
	PolicyAvailableIntegrationList = NewPolicy(ResourceAvailableIntegration, ActionList)
	PolicyAvailableIntegrationRead = NewPolicy(ResourceAvailableIntegration, ActionRead)
	// Registered integrations
	PolicyRegisteredIntegrationList = NewPolicy(ResourceRegisteredIntegration, ActionList)
	PolicyRegisteredIntegrationRead = NewPolicy(ResourceRegisteredIntegration, ActionRead)
	PolicyRegisteredIntegrationAdd  = NewPolicy(ResourceRegisteredIntegration, ActionCreate)
	// Attached integrations
	PolicyAttachedIntegrationList   = NewPolicy(ResourceAttachedIntegration, ActionList)
	PolicyAttachedIntegrationAttach = NewPolicy(ResourceAttachedIntegration, ActionCreate)
	PolicyAttachedIntegrationDetach = NewPolicy(ResourceAttachedIntegration, ActionDelete)
	// Org Metrics
	PolicyOrgMetricsRead = NewPolicy(ResourceOrgMetric, ActionList)
	// Robot Account
	PolicyRobotAccountList   = NewPolicy(ResourceRobotAccount, ActionList)
	PolicyRobotAccountCreate = NewPolicy(ResourceRobotAccount, ActionCreate)
	// Workflow Contract
	PolicyWorkflowContractList   = NewPolicy(ResourceWorkflowContract, ActionList)
	PolicyWorkflowContractRead   = NewPolicy(ResourceWorkflowContract, ActionRead)
	PolicyWorkflowContractUpdate = NewPolicy(ResourceWorkflowContract, ActionUpdate)
	PolicyWorkflowContractCreate = NewPolicy(ResourceWorkflowContract, ActionCreate)
	// WorkflowRun
	PolicyWorkflowRunList   = NewPolicy(ResourceWorkflowRun, ActionList)
	PolicyWorkflowRunRead   = NewPolicy(ResourceWorkflowRun, ActionRead)
	PolicyWorkflowRunCreate = NewPolicy(ResourceWorkflowRun, ActionCreate)
	PolicyWorkflowRunUpdate = NewPolicy(ResourceWorkflowRun, ActionUpdate)
	// Workflow
	PolicyWorkflowList   = NewPolicy(ResourceWorkflow, ActionList)
	PolicyWorkflowRead   = NewPolicy(ResourceWorkflow, ActionRead)
	PolicyWorkflowCreate = NewPolicy(ResourceWorkflow, ActionCreate)
	PolicyWorkflowUpdate = NewPolicy(ResourceWorkflow, ActionUpdate)
	PolicyWorkflowDelete = NewPolicy(ResourceWorkflow, ActionDelete)
	// User Membership
	PolicyOrganizationRead            = NewPolicy(Organization, ActionRead)
	PolicyOrganizationListMemberships = NewPolicy(Organization, ActionRead)
	// Groups
	PolicyGroupCreate = NewPolicy(ResourceGroup, ActionCreate)
	PolicyGroupUpdate = NewPolicy(ResourceGroup, ActionUpdate)
	PolicyGroupDelete = NewPolicy(ResourceGroup, ActionDelete)
	PolicyGroupList   = NewPolicy(ResourceGroup, ActionList)
	PolicyGroupRead   = NewPolicy(ResourceGroup, ActionRead)
	// Group Memberships
	PolicyGroupListMemberships = NewPolicy(ResourceGroupMembership, ActionList)
)

// List of policies for each role
// NOTE: roles are hierarchical, this means that the Admin Role can inherit all the policies from the Viewer Role
// so we do not need to add them as well.
var rolesMap = map[Role][]*Policy{
	// RoleViewer is an org-scoped role that provides read-only access to all resources
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
	// RoleAdmin is an org-scoped role that provides super admin privileges (it's the higher role)
	RoleAdmin: {
		// We do a manual check in the artifact upload endpoint
		// so we need the actual policy in place skipping it is not enough
		PolicyArtifactUpload,
		// + all the policies from the viewer role inherited automatically
	},
	// RoleOrgMember is an org-scoped role that enables RBAC in the underlying resources. Users with this role at
	// the organization level will need specific project roles to access their contents
	RoleOrgMember: {
		// Allowed endpoints. RBAC will be applied where needed
		PolicyWorkflowRead,
		PolicyWorkflowContractList,
		PolicyWorkflowContractRead,
		PolicyWorkflowContractCreate,
		PolicyWorkflowContractUpdate,

		PolicyWorkflowList,
		PolicyWorkflowCreate,
		PolicyWorkflowUpdate,
		PolicyWorkflowDelete,

		PolicyWorkflowRunList,
		PolicyWorkflowRunRead,

		PolicyArtifactDownload,

		PolicyCASBackendList,

		PolicyOrganizationRead,

		// integrations
		PolicyAvailableIntegrationList,
		PolicyAvailableIntegrationRead,
		PolicyRegisteredIntegrationList,
		PolicyRegisteredIntegrationRead,
		// attachments (RBAC will be applied)
		PolicyAttachedIntegrationList,
		PolicyAttachedIntegrationAttach,
		PolicyAttachedIntegrationDetach,

		PolicyOrgMetricsRead,
		PolicyReferrerRead,

		// Groups
		PolicyGroupList,
		PolicyGroupRead,

		// Group Memberships
		PolicyGroupListMemberships,
	},
	// RoleProjectViewer: has read-only permissions on a project
	RoleProjectViewer: {
		PolicyWorkflowRead,
		PolicyWorkflowRunRead,
	},
	// RoleProjectAdmin: represents a project administrator. It's the higher role in project resources,
	// and it's only considered when the org-level role is `RoleOrgMember`
	RoleProjectAdmin: {
		// attestations

		PolicyWorkflowRead,
		PolicyWorkflowCreate,
		PolicyWorkflowRunCreate,
		PolicyWorkflowRunUpdate, // to reset attestations

		// workflow operations

		PolicyWorkflowUpdate,
		PolicyWorkflowDelete,

		// workflow runs
		PolicyWorkflowRunRead,

		// integrations
		PolicyAttachedIntegrationAttach,
		PolicyAttachedIntegrationDetach,
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
	"/controlplane.v1.IntegrationsService/Detach":          {PolicyAttachedIntegrationDetach},
	// Metrics
	"/controlplane.v1.OrgMetricsService/.*": {PolicyOrgMetricsRead},
	// Robot Account
	"/controlplane.v1.RobotAccountService/List":   {PolicyRobotAccountList},
	"/controlplane.v1.RobotAccountService/Create": {PolicyRobotAccountCreate},
	// Workflows
	"/controlplane.v1.WorkflowService/List":   {PolicyWorkflowList},
	"/controlplane.v1.WorkflowService/View":   {PolicyWorkflowRead},
	"/controlplane.v1.WorkflowService/Create": {PolicyWorkflowCreate},
	"/controlplane.v1.WorkflowService/Update": {PolicyWorkflowUpdate},
	"/controlplane.v1.WorkflowService/Delete": {PolicyWorkflowDelete},
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

	"/controlplane.v1.OrganizationService/ListMemberships": {PolicyOrganizationListMemberships},
	// Groups
	"/controlplane.v1.GroupService/List":   {PolicyGroupList},
	"/controlplane.v1.GroupService/Get":    {PolicyGroupRead},
	"/controlplane.v1.GroupService/Create": {PolicyGroupCreate},
	"/controlplane.v1.GroupService/Update": {PolicyGroupUpdate},
	"/controlplane.v1.GroupService/Delete": {PolicyGroupDelete},
	// Group Memberships
	"/controlplane.v1.GroupService/ListMembers": {PolicyGroupListMemberships},
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
func NewDatabaseEnforcer(c *config.DatabaseConfig, managedResources []Resource) (*Enforcer, error) {
	// policy storage in database
	a, err := entadapter.NewAdapter(c.Driver, c.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	e, err := newEnforcer(a, managedResources)
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
func NewFiletypeEnforcer(path string, managedResources []Resource) (*Enforcer, error) {
	// policy storage in filesystem
	a := fileadapter.NewAdapter(path)
	e, err := newEnforcer(a, managedResources)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	return e, nil
}

// NewEnforcer creates a new casbin authorization enforcer for the policies stored
// in the database and the model defined in model.conf
func newEnforcer(a persist.Adapter, managedResources []Resource) (*Enforcer, error) {
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
	if err := syncRBACRoles(&Enforcer{enforcer}, managedResources); err != nil {
		return nil, fmt.Errorf("failed to sync roles: %w", err)
	}

	return &Enforcer{enforcer}, nil
}

// Load the roles map into the enforcer
// This is done by adding all the policies defined in the roles map
// and removing all the policies that are not
func syncRBACRoles(e *Enforcer, managedResources []Resource) error {
	return doSync(e, rolesMap, managedResources)
}

func doSync(e *Enforcer, rolesMap map[Role][]*Policy, managedResources []Resource) error {
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
		resource := gotPolicies[1]
		action := gotPolicies[2]
		policy := &Policy{Resource: resource, Action: action}

		// if it's not a managed resource, skip deletion
		if !slices.Contains(managedResources, Resource(resource)) {
			continue
		}

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

		// RBAC roles
		RoleOrgMember,
		RoleProjectAdmin,
		RoleProjectViewer,
	} {
		roles = append(roles, string(s))
	}

	return
}
