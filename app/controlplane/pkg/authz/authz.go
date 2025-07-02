//
// Copyright 2024-2025 The Chainloop Authors.
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

// resource, action tuple
type Policy struct {
	Resource string
	Action   string
}

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
	Organization                  = "organization"
	ResourceGroup                 = "group"
	ResourceGroupMembership       = "group_membership"
	ResourceProjectAPIToken       = "project_api_token"
	ResourceProjectMembership     = "project_membership"

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

	// RoleGroupMaintainer is a role that can manage groups in an organization.
	RoleGroupMaintainer Role = "role:group:maintainer"
)

// ManagedResources are the resources that are managed by Chainloop, considered during permissions sync
var ManagedResources = []string{
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
	PolicyAttachedIntegrationDetach = &Policy{ResourceAttachedIntegration, ActionDelete}
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
	PolicyWorkflowRunList   = &Policy{ResourceWorkflowRun, ActionList}
	PolicyWorkflowRunRead   = &Policy{ResourceWorkflowRun, ActionRead}
	PolicyWorkflowRunCreate = &Policy{ResourceWorkflowRun, ActionCreate}
	PolicyWorkflowRunUpdate = &Policy{ResourceWorkflowRun, ActionUpdate}
	// Workflow
	PolicyWorkflowList   = &Policy{ResourceWorkflow, ActionList}
	PolicyWorkflowRead   = &Policy{ResourceWorkflow, ActionRead}
	PolicyWorkflowCreate = &Policy{ResourceWorkflow, ActionCreate}
	PolicyWorkflowUpdate = &Policy{ResourceWorkflow, ActionUpdate}
	PolicyWorkflowDelete = &Policy{ResourceWorkflow, ActionDelete}
	// User Membership
	PolicyOrganizationRead            = &Policy{Organization, ActionRead}
	PolicyOrganizationListMemberships = &Policy{Organization, ActionRead}
	// Groups
	PolicyGroupList = &Policy{ResourceGroup, ActionList}
	PolicyGroupRead = &Policy{ResourceGroup, ActionRead}
	// Group Memberships
	PolicyGroupListMemberships   = &Policy{ResourceGroupMembership, ActionList}
	PolicyGroupAddMemberships    = &Policy{ResourceGroupMembership, ActionCreate}
	PolicyGroupRemoveMemberships = &Policy{ResourceGroupMembership, ActionDelete}
	// Project API Token
	PolicyProjectAPITokenList   = &Policy{ResourceProjectAPIToken, ActionList}
	PolicyProjectAPITokenCreate = &Policy{ResourceProjectAPIToken, ActionCreate}
	PolicyProjectAPITokenRevoke = &Policy{ResourceProjectAPIToken, ActionDelete}
	// Project Memberships
	PolicyProjectListMemberships   = &Policy{ResourceProjectMembership, ActionList}
	PolicyProjectAddMemberships    = &Policy{ResourceProjectMembership, ActionCreate}
	PolicyProjectRemoveMemberships = &Policy{ResourceProjectMembership, ActionDelete}
)

// RolesMap The default list of policies for each role
// NOTE: roles are not necessarily hierarchical, however the Admin Role inherits all the policies from the Viewer Role
// so we do not need to add them as well.
var RolesMap = map[Role][]*Policy{
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
		// Groups
		PolicyGroupList,
		PolicyGroupRead,
		// Group Memberships
		PolicyGroupListMemberships,
		// Project Memberships
		PolicyProjectListMemberships,
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
		PolicyArtifactUpload,

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

		// Project API Token
		PolicyProjectAPITokenList,
		PolicyProjectAPITokenCreate,
		PolicyProjectAPITokenRevoke,

		// Project Memberships
		PolicyProjectListMemberships,
		PolicyProjectAddMemberships,
		PolicyProjectRemoveMemberships,
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

		// Project API Token
		PolicyProjectAPITokenList,
		PolicyProjectAPITokenCreate,
		PolicyProjectAPITokenRevoke,

		// Project Memberships
		PolicyProjectListMemberships,
		PolicyProjectAddMemberships,
		PolicyProjectRemoveMemberships,
	},
	// RoleGroupMaintainer: represents a group maintainer role.
	RoleGroupMaintainer: {
		PolicyGroupAddMemberships,
		PolicyGroupRemoveMemberships,
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
	"/controlplane.v1.GroupService/List": {PolicyGroupList},
	// Group Memberships
	"/controlplane.v1.GroupService/ListMembers": {PolicyGroupListMemberships},
	// For the following endpoints, we rely on the service layer to check the permissions
	// That's why we let everyone access them (empty policies)
	"/controlplane.v1.GroupService/AddMember":    {},
	"/controlplane.v1.GroupService/RemoveMember": {},
	// Project API Token
	"/controlplane.v1.ProjectService/APITokenCreate": {PolicyProjectAPITokenCreate},
	"/controlplane.v1.ProjectService/APITokenList":   {PolicyProjectAPITokenList},
	"/controlplane.v1.ProjectService/APITokenRevoke": {PolicyProjectAPITokenRevoke},
	// Project Memberships
	"/controlplane.v1.ProjectService/ListMembers":  {PolicyProjectListMemberships},
	"/controlplane.v1.ProjectService/AddMember":    {PolicyProjectAddMemberships},
	"/controlplane.v1.ProjectService/RemoveMember": {PolicyProjectRemoveMemberships},
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
		RoleGroupMaintainer,
	} {
		roles = append(roles, string(s))
	}

	return
}
