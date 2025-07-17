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

// RBACEnabled returns whether an org-scoped role has RBAC enabled and needs resource-scoped enforcement.
func (r Role) RBACEnabled() bool {
	return r == RoleOrgMember || r == RoleOrgContributor
}

func (r Role) IsAdmin() bool {
	return r == RoleAdmin || r == RoleOwner
}

const (
	// Actions

	ActionRead   = "read"
	ActionList   = "list"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"

	// Resources

	ResourceWorkflowContract        = "workflow_contract"
	ResourceCASArtifact             = "cas_artifact"
	ResourceCASBackend              = "cas_backend"
	ResourceReferrer                = "referrer"
	ResourceAvailableIntegration    = "integration_available"
	ResourceRegisteredIntegration   = "integration_registered"
	ResourceAttachedIntegration     = "integration_attached"
	ResourceOrgMetric               = "metrics_org"
	ResourceRobotAccount            = "robot_account"
	ResourceWorkflowRun             = "workflow_run"
	ResourceWorkflow                = "workflow"
	ResourceProject                 = "project"
	Organization                    = "organization"
	OrganizationMemberships         = "organization_memberships"
	ResourceGroup                   = "group"
	ResourceGroupMembership         = "group_membership"
	ResourceAPIToken                = "api_token"
	ResourceProjectMembership       = "project_membership"
	ResourceOrganizationInvitations = "organization_invitations"
	ResourceGroupProjects           = "group_projects"

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

	// RoleOrgMember cannot see projects until they are invited. However, they are able to create their own projects,
	// so Casbin rules (role, resource-type, action) are NOT enough to check for permission, since we must check for ownership as well.
	// That last check will be done at the service level.
	RoleOrgMember Role = "role:org:member"

	// RoleOrgContributor can work on projects they are invited to with scoped role ProjectAdmin or ProjectViewer, but they cannot create their own projects.
	RoleOrgContributor Role = "role:org:contributor"

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
	ResourceProject,
	Organization,
	OrganizationMemberships,
	ResourceGroup,
	ResourceGroupMembership,
	ResourceAPIToken,
	ResourceProjectMembership,
	ResourceOrganizationInvitations,
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
	PolicyWorkflowContractDelete = &Policy{ResourceWorkflowContract, ActionDelete}
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
	// Projects
	PolicyProjectCreate = &Policy{ResourceProject, ActionCreate}
	// User Membership
	PolicyOrganizationRead            = &Policy{Organization, ActionRead}
	PolicyOrganizationListMemberships = &Policy{OrganizationMemberships, ActionList}

	// Groups
	PolicyGroupList = &Policy{ResourceGroup, ActionList}
	PolicyGroupRead = &Policy{ResourceGroup, ActionRead}

	// Group Memberships
	PolicyGroupListPendingInvitations = &Policy{ResourceGroup, ActionList}
	PolicyGroupListMemberships        = &Policy{ResourceGroupMembership, ActionList}
	PolicyGroupAddMemberships         = &Policy{ResourceGroupMembership, ActionCreate}
	PolicyGroupRemoveMemberships      = &Policy{ResourceGroupMembership, ActionDelete}
	PolicyGroupUpdateMemberships      = &Policy{ResourceGroupMembership, ActionUpdate}
	PolicyGroupListProjects           = &Policy{ResourceGroupProjects, ActionList}

	// API Token
	PolicyAPITokenList   = &Policy{ResourceAPIToken, ActionList}
	PolicyAPITokenCreate = &Policy{ResourceAPIToken, ActionCreate}
	PolicyAPITokenRevoke = &Policy{ResourceAPIToken, ActionDelete}
	// Project Memberships
	PolicyProjectListMemberships   = &Policy{ResourceProjectMembership, ActionList}
	PolicyProjectAddMemberships    = &Policy{ResourceProjectMembership, ActionCreate}
	PolicyProjectUpdateMemberships = &Policy{ResourceProjectMembership, ActionUpdate}
	PolicyProjectRemoveMemberships = &Policy{ResourceProjectMembership, ActionDelete}
	// Organization Invitations
	PolicyOrganizationInvitationsCreate = &Policy{ResourceOrganizationInvitations, ActionCreate}
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
		// API tokens
		PolicyAPITokenList,
	},
	// RoleAdmin is an org-scoped role that provides super admin privileges (it's the higher role)
	RoleAdmin: {
		// We do a manual check in the artifact upload endpoint
		// so we need the actual policy in place skipping it is not enough
		PolicyArtifactUpload,
		// We manually check this policy to be able to know if the user can invite users to the system
		PolicyOrganizationInvitationsCreate,
		// + all the policies from the viewer role inherited automatically
	},

	// RoleOrgMember is an org-scoped role that enables RBAC in the underlying resources. Users with this role at
	// the organization level will need specific project roles to access their contents
	RoleOrgContributor: {
		// Allowed endpoints. RBAC will be applied where needed
		PolicyWorkflowRead,
		PolicyWorkflowContractList,
		PolicyWorkflowContractRead,
		PolicyWorkflowContractCreate,
		PolicyWorkflowContractUpdate,
		PolicyWorkflowContractDelete,

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

		// Additional check for API tokens is done at the service level
		PolicyAPITokenList,
		PolicyAPITokenCreate,
		PolicyAPITokenRevoke,

		// Project Memberships available to contributors if they are project admins
		PolicyProjectListMemberships,
		PolicyProjectAddMemberships,
		PolicyProjectRemoveMemberships,
		PolicyProjectUpdateMemberships,
	},

	// RoleOrgMember inherits from RoleOrgContributor and can also create their own projects and see members
	RoleOrgMember: {
		PolicyProjectCreate,

		// Org memberships
		PolicyOrganizationListMemberships,
	},

	// RoleProjectViewer: has read-only permissions on a project
	RoleProjectViewer: {
		PolicyWorkflowRead,
		PolicyWorkflowRunRead,
		// workflow contracts
		PolicyWorkflowContractList,
		PolicyWorkflowContractRead,
		// Project API Token
		PolicyAPITokenList,
	},
	// RoleProjectAdmin: inherits from ProjectViewer and represents a project administrator.
	RoleProjectAdmin: {
		// workflow contracts
		PolicyWorkflowContractCreate,
		PolicyWorkflowContractUpdate,
		PolicyWorkflowContractDelete,

		// attestations
		PolicyWorkflowCreate,
		PolicyWorkflowRunCreate,
		PolicyWorkflowRunUpdate, // to reset attestations

		// workflow operations
		PolicyWorkflowUpdate,
		PolicyWorkflowDelete,

		// integrations
		PolicyAttachedIntegrationAttach,
		PolicyAttachedIntegrationDetach,

		// Project API Token
		PolicyAPITokenCreate,
		PolicyAPITokenRevoke,

		// Project Memberships
		PolicyProjectListMemberships,
		PolicyProjectAddMemberships,
		PolicyProjectRemoveMemberships,
		PolicyProjectUpdateMemberships,
	},
	// RoleGroupMaintainer: represents a group maintainer role.
	RoleGroupMaintainer: {
		// Group Memberships
		PolicyGroupListMemberships,
		PolicyGroupListPendingInvitations,
		PolicyGroupAddMemberships,
		PolicyGroupRemoveMemberships,
		PolicyGroupUpdateMemberships,
		PolicyGroupListProjects,
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
	"/controlplane.v1.WorkflowContractService/Delete":   {PolicyWorkflowContractDelete},
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

	// org memberships onnly available to admins
	// "/controlplane.v1.OrganizationService/ListMemberships"

	// Groups (everyone see groups)
	"/controlplane.v1.GroupService/List": {},
	"/controlplane.v1.GroupService/Get":  {},

	// For the following endpoints, we rely on the service layer to check the permissions
	// That's why we let everyone access them (empty policies).
	// Group Memberships are only available to org admins or maintainers
	"/controlplane.v1.GroupService/ListMembers":                  {},
	"/controlplane.v1.GroupService/ListProjects":                 {},
	"/controlplane.v1.GroupService/AddMember":                    {},
	"/controlplane.v1.GroupService/RemoveMember":                 {},
	"/controlplane.v1.GroupService/ListPendingInvitations":       {},
	"/controlplane.v1.GroupService/UpdateMemberMaintainerStatus": {},

	// Project Memberships
	"/controlplane.v1.ProjectService/ListMembers":            {PolicyProjectListMemberships},
	"/controlplane.v1.ProjectService/AddMember":              {PolicyProjectAddMemberships},
	"/controlplane.v1.ProjectService/RemoveMember":           {PolicyProjectRemoveMemberships},
	"/controlplane.v1.ProjectService/UpdateMemberRole":       {PolicyProjectUpdateMemberships},
	"/controlplane.v1.ProjectService/ListPendingInvitations": {PolicyProjectListMemberships},

	// API tokens RBAC are handled at the service level
	"/controlplane.v1.APITokenService/List":   {PolicyAPITokenList},
	"/controlplane.v1.APITokenService/Create": {PolicyAPITokenCreate},
	"/controlplane.v1.APITokenService/Revoke": {PolicyAPITokenRevoke},
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
		RoleOrgContributor,
		RoleProjectAdmin,
		RoleProjectViewer,
		RoleGroupMaintainer,
	} {
		roles = append(roles, string(s))
	}

	return
}
