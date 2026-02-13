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

package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OrgInvitationUseCase struct {
	logger *log.Helper
	// Repositories
	repo        OrgInvitationRepo
	mRepo       MembershipRepo
	userRepo    UserRepo
	groupRepo   GroupRepo
	projectRepo ProjectsRepo
	// Use cases
	auditor *AuditorUseCase
}

type OrgInvitation struct {
	ID            uuid.UUID
	Org           *Organization
	Sender        *User
	ReceiverEmail string
	CreatedAt     *time.Time
	Status        OrgInvitationStatus
	Role          authz.Role
	// Context is a JSON field that can be used to store additional information
	Context *OrgInvitationContext
}

// OrgInvitationContext is used to pass additional context when accepting an invitation
type OrgInvitationContext struct {
	// GroupIDToJoin is the ID of the group to join when accepting the invitation
	GroupIDToJoin *uuid.UUID `json:"group_id_to_join,omitempty"`
	// GroupMaintainer indicates if the user should be added as a maintainer of the group
	GroupMaintainer bool `json:"group_maintainer,omitempty"`
	// ProjectIDToJoin is the ID of the project to join when accepting the invitation
	ProjectIDToJoin *uuid.UUID `json:"project_id_to_join,omitempty"`
	// ProjectRole is the role to assign to the user in the project
	ProjectRole authz.Role `json:"project_role,omitempty"`
	// ExternalMetadata can be used to store additional information
	ExternalMetadata json.RawMessage `json:"external_metadata,omitempty"`
}

type OrgInvitationRepo interface {
	Create(ctx context.Context, orgID uuid.UUID, senderID *uuid.UUID, receiverEmail string, role authz.Role, invCtx *OrgInvitationContext) (*OrgInvitation, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*OrgInvitation, error)
	PendingInvitation(ctx context.Context, orgID uuid.UUID, receiverEmail string) (*OrgInvitation, error)
	PendingInvitations(ctx context.Context, receiverEmail string) ([]*OrgInvitation, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListByOrg(ctx context.Context, org uuid.UUID) ([]*OrgInvitation, error)
	ChangeStatus(ctx context.Context, ID uuid.UUID, status OrgInvitationStatus) error
}

func NewOrgInvitationUseCase(r OrgInvitationRepo, mRepo MembershipRepo, uRepo UserRepo, auditorUC *AuditorUseCase, groupRepo GroupRepo, projectRepo ProjectsRepo, l log.Logger) (*OrgInvitationUseCase, error) {
	return &OrgInvitationUseCase{
		logger: servicelogger.ScopedHelper(l, "biz/orgInvitation"),
		repo:   r, mRepo: mRepo, userRepo: uRepo, auditor: auditorUC, groupRepo: groupRepo, projectRepo: projectRepo,
	}, nil
}

type invitationCreateOpts struct {
	role     authz.Role
	ctx      *OrgInvitationContext
	senderID *uuid.UUID
}

type InvitationCreateOpt func(*invitationCreateOpts)

func WithInvitationRole(r authz.Role) InvitationCreateOpt {
	return func(o *invitationCreateOpts) {
		o.role = r
	}
}

// WithInvitationContext allows passing additional context when creating an invitation
// This context will be taken into account when accepting the invitation
func WithInvitationContext(ctx *OrgInvitationContext) InvitationCreateOpt {
	return func(o *invitationCreateOpts) {
		o.ctx = ctx
	}
}

func WithSender(senderID uuid.UUID) InvitationCreateOpt {
	return func(o *invitationCreateOpts) {
		o.senderID = &senderID
	}
}

func (uc *OrgInvitationUseCase) Create(ctx context.Context, orgID, receiverEmail string, createOpts ...InvitationCreateOpt) (*OrgInvitation, error) {
	receiverEmail = strings.ToLower(receiverEmail)

	// 1 - Static Validation
	if receiverEmail == "" {
		return nil, NewErrValidationStr("receiver email is required")
	}

	// Default to viewer role
	opts := &invitationCreateOpts{
		role: authz.RoleViewer,
	}

	for _, o := range createOpts {
		o(opts)
	}

	// If it has ben overrode by the user, validate it
	if opts.role == "" {
		return nil, NewErrValidationStr("role is required")
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// 2 - the sender exists and it's not the same than the receiver of the invitation
	if opts.senderID != nil {
		sender, err := uc.userRepo.FindByID(ctx, *opts.senderID)
		if err != nil {
			return nil, fmt.Errorf("error finding sender %s: %w", opts.senderID.String(), err)
		} else if sender == nil {
			return nil, NewErrNotFound("sender")
		}

		if sender.Email == receiverEmail {
			return nil, NewErrValidationStr("sender and receiver emails cannot be the same")
		}
	}

	// 4 - The receiver does exist in the org already
	_, membershipCount, err := uc.mRepo.FindByOrg(ctx, orgUUID, &ListByOrgOpts{
		Email: &receiverEmail,
	}, pagination.NewDefaultOffsetPaginationOpts())
	if err != nil {
		return nil, fmt.Errorf("error finding memberships for user %s: %w", receiverEmail, err)
	}

	if membershipCount > 0 {
		return nil, NewErrValidationStr("user already exists in the org")
	}

	// 5 - Check if there is already an invitation for this user for this org
	m, err := uc.repo.PendingInvitation(ctx, orgUUID, receiverEmail)
	if err != nil {
		return nil, fmt.Errorf("error finding invitation for org %s and receiver %s: %w", orgID, receiverEmail, err)
	}

	if m != nil {
		return nil, NewErrValidationStr("invitation already exists for this user and org")
	}

	// 6 - Create the invitation
	invitation, err := uc.repo.Create(ctx, orgUUID, opts.senderID, receiverEmail, opts.role, opts.ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating invitation: %w", err)
	}

	// 7 - Audit the event
	uc.auditor.Dispatch(ctx, &events.OrgUserInvited{
		OrgBase: &events.OrgBase{
			OrgID:   &orgUUID,
			OrgName: invitation.Org.Name,
		},
		ReceiverEmail: receiverEmail,
		Role:          string(opts.role),
	}, &orgUUID)

	return invitation, nil
}

func (uc *OrgInvitationUseCase) ListByOrg(ctx context.Context, orgID string) ([]*OrgInvitation, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.ListByOrg(ctx, orgUUID)
}

// Revoke an invitation by ID only if the user is the one who created it
func (uc *OrgInvitationUseCase) Revoke(ctx context.Context, orgID, invitationID string) error {
	invitationUUID, err := uuid.Parse(invitationID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// We care only about pending invitations in the given org
	m, err := uc.repo.FindByID(ctx, invitationUUID)
	if err != nil {
		return fmt.Errorf("error finding invitation %s: %w", invitationID, err)
	} else if m == nil || m.Org.ID != orgID {
		return NewErrNotFound("invitation")
	}

	if m.Status != OrgInvitationStatusPending {
		return NewErrValidationStr("invitation is not in pending state")
	}

	return uc.repo.SoftDelete(ctx, invitationUUID)
}

// AcceptPendingInvitations accepts all pending invitations for a given user email
func (uc *OrgInvitationUseCase) AcceptPendingInvitations(ctx context.Context, receiverEmail string) error {
	user, err := uc.userRepo.FindByEmail(ctx, receiverEmail)
	if err != nil {
		return fmt.Errorf("error finding user %s: %w", receiverEmail, err)
	} else if user == nil {
		return NewErrNotFound("user")
	}

	userUUID, err := uuid.Parse(user.ID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// Find all memberships for the user and all pending invitations
	memberships, err := uc.mRepo.FindByUser(ctx, userUUID)
	if err != nil {
		return fmt.Errorf("error finding memberships for user %s: %w", receiverEmail, err)
	}

	invitations, err := uc.repo.PendingInvitations(ctx, receiverEmail)
	if err != nil {
		return fmt.Errorf("error finding pending invitations for user %s: %w", receiverEmail, err)
	}

	uc.logger.Infow("msg", "Checking pending invitations", "user_id", user.ID, "invitations", len(invitations))

	// Iterate on the invitations and create the membership if it doesn't exist
	for _, invitation := range invitations {
		var alreadyMember bool
		for _, m := range memberships {
			if m.OrganizationID.String() == invitation.Org.ID {
				alreadyMember = true
			}
		}

		orgUUID, err := uuid.Parse(invitation.Org.ID)
		if err != nil {
			return NewErrInvalidUUID(err)
		}

		// user is not a member of the org, create the membership
		// role was defined during the invitation
		role := invitation.Role
		if !alreadyMember {
			uc.logger.Infow("msg", "Adding member", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", user.ID, "role", role)
			if _, err := uc.mRepo.Create(ctx, orgUUID, userUUID, false, role); err != nil {
				return fmt.Errorf("error creating membership for user %s: %w", receiverEmail, err)
			}

			// the user joins the org
			// set the context user so it can be used in the auditor
			ctx = entities.WithCurrentUser(ctx, &entities.User{Email: user.Email, ID: user.ID})
			uc.auditor.Dispatch(ctx, &events.OrgUserJoined{
				OrgBase: &events.OrgBase{
					OrgID:   &orgUUID,
					OrgName: invitation.Org.Name,
				},
				UserID:       userUUID,
				UserEmail:    user.Email,
				InvitationID: invitation.ID,
			}, &orgUUID)
		}

		// Process group membership if present in the invitation context
		if err := uc.processGroupMembership(ctx, invitation, orgUUID, userUUID); err != nil {
			return fmt.Errorf("error processing group membership to user %s: %w", receiverEmail, err)
		}

		// Process project membership if present in the invitation context
		if err := uc.processProjectMembership(ctx, invitation, orgUUID, userUUID); err != nil {
			return fmt.Errorf("error processing project membership to user %s: %w", receiverEmail, err)
		}

		uc.logger.Infow("msg", "Accepting invitation", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", user.ID, "role", role)
		// change the status of the invitation
		if err := uc.repo.ChangeStatus(ctx, invitation.ID, OrgInvitationStatusAccepted); err != nil {
			return fmt.Errorf("error changing status of invitation %s: %w", invitation.ID.String(), err)
		}
	}

	return nil
}

func (uc *OrgInvitationUseCase) AcceptInvitation(ctx context.Context, invitationID string) error {
	invitationUUID, err := uuid.Parse(invitationID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	return uc.repo.ChangeStatus(ctx, invitationUUID, OrgInvitationStatusAccepted)
}

func (uc *OrgInvitationUseCase) FindByID(ctx context.Context, invitationID string) (*OrgInvitation, error) {
	invitationUUID, err := uuid.Parse(invitationID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	invitation, err := uc.repo.FindByID(ctx, invitationUUID)
	if err != nil {
		return nil, fmt.Errorf("error finding invitation %s: %w", invitationID, err)
	} else if invitation == nil {
		return nil, NewErrNotFound("invitation")
	}

	return invitation, nil
}

// processGroupMembership adds a user to a group if the invitation context contains a group to join
func (uc *OrgInvitationUseCase) processGroupMembership(ctx context.Context, invitation *OrgInvitation, orgUUID uuid.UUID, userUUID uuid.UUID) error {
	// Skip if there's no group to join in the invitation context
	if invitation.Context == nil || invitation.Context.GroupIDToJoin == nil || *invitation.Context.GroupIDToJoin == uuid.Nil {
		return nil
	}

	groupID := invitation.Context.GroupIDToJoin
	uc.logger.Infow("msg", "Adding user to group", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", userUUID, "group_id", groupID)

	// Check if the group exists
	gr, err := uc.groupRepo.FindByOrgAndID(ctx, orgUUID, *groupID)
	if err != nil {
		return fmt.Errorf("error finding group %s: %w", groupID.String(), err)
	}

	if _, err := uc.groupRepo.AddMemberToGroup(ctx, orgUUID, *groupID, userUUID, invitation.Context.GroupMaintainer); err != nil {
		if IsErrAlreadyExists(err) {
			// User is already a member of the group, nothing to do
			uc.logger.Infow("msg", "User already in group", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", userUUID.String(), "group_id", groupID.String())
			return nil
		}

		return fmt.Errorf("error adding user %s to group %s: %w", userUUID, groupID.String(), err)
	}

	// Dispatch event to the audit log for group membership addition
	uc.auditor.Dispatch(ctx, &events.GroupMemberAdded{
		GroupBase: &events.GroupBase{
			GroupID:   groupID,
			GroupName: gr.Name,
		},
		UserID:     &userUUID,
		UserEmail:  invitation.ReceiverEmail,
		Maintainer: invitation.Context.GroupMaintainer,
	}, &orgUUID)

	uc.logger.Infow("msg", "User added to group successfully", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", userUUID.String(), "group_id", groupID.String())

	return nil
}

// processProjectMembership adds a user to a project if the invitation context contains a project to join
func (uc *OrgInvitationUseCase) processProjectMembership(ctx context.Context, invitation *OrgInvitation, orgUUID uuid.UUID, userUUID uuid.UUID) error {
	// Skip if there's no group to join in the invitation context
	if invitation.Context == nil || invitation.Context.ProjectIDToJoin == nil || *invitation.Context.ProjectIDToJoin == uuid.Nil {
		return nil
	}

	projectID := invitation.Context.ProjectIDToJoin
	uc.logger.Infow("msg", "Adding user to project", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", userUUID, "project_id", projectID)

	// Check if the project exists
	project, err := uc.projectRepo.FindProjectByOrgIDAndID(ctx, orgUUID, *projectID)
	if err != nil {
		return fmt.Errorf("error finding project %s: %w", projectID.String(), err)
	}

	if project == nil {
		uc.logger.Infow("msg", "Project no longer exists", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", userUUID.String(), "project_id", projectID.String())
		return nil
	}

	// Use the correct role from the invitation context
	role := invitation.Context.ProjectRole
	if role == "" {
		// Default to viewer if no role specified
		role = authz.RoleProjectViewer
	}

	// Check if the user is already a member of the project
	existingMembership, err := uc.projectRepo.FindProjectMembershipByProjectAndID(ctx, orgUUID, *projectID, userUUID, authz.MembershipTypeUser)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("error checking project membership for user %s: %w", userUUID, err)
	}

	if existingMembership != nil {
		// User is already a member of the project, nothing to do
		uc.logger.Infow("msg", "User already in project", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", userUUID.String(), "project_id", projectID.String())
		return nil
	}

	// Add the user to the project
	if _, err := uc.projectRepo.AddMemberToProject(ctx, orgUUID, *projectID, userUUID, authz.MembershipTypeUser, role); err != nil {
		return fmt.Errorf("error adding user %s to project %s: %w", userUUID, projectID.String(), err)
	}

	// Dispatch event to the audit log for project membership addition
	uc.auditor.Dispatch(ctx, &events.ProjectMembershipAdded{
		ProjectBase: &events.ProjectBase{
			ProjectID:   projectID,
			ProjectName: project.Name,
		},
		UserID:    &userUUID,
		UserEmail: invitation.ReceiverEmail,
		Role:      string(role),
	}, &orgUUID)

	uc.logger.Infow("msg", "User added to project successfully", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", userUUID.String(), "project_id", projectID.String())

	return nil
}

type OrgInvitationStatus string

var (
	OrgInvitationStatusPending  OrgInvitationStatus = "pending"
	OrgInvitationStatusAccepted OrgInvitationStatus = "accepted"
)

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (OrgInvitationStatus) Values() (kinds []string) {
	for _, s := range []OrgInvitationStatus{OrgInvitationStatusAccepted, OrgInvitationStatusPending} {
		kinds = append(kinds, string(s))
	}

	return
}
