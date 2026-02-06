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

package usercontext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

// membershipsCache caches user memberships to save some database queries during intensive sessions
var membershipsCache = expirable.NewLRU[string, *entities.Membership](0, nil, time.Second*1)

func WithCurrentMembershipsMiddleware(membershipUC biz.MembershipsRBAC) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Get the current user and return if not found, meaning we are probably coming from an API Token
			u := entities.CurrentUser(ctx)
			if u == nil {
				return handler(ctx, req)
			}

			var err error
			// Let's store all memberships in the context.
			ctx, err = setCurrentMembershipsForUser(ctx, u, membershipUC)
			if err != nil {
				return nil, fmt.Errorf("error setting current org membership: %w", err)
			}

			return handler(ctx, req)
		}
	}
}

func WithCurrentOrganizationMiddleware(userUseCase biz.UserOrgFinder, orgUC *biz.OrganizationUseCase, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Get the current user and return if not found, meaning we are probably coming from an API Token
			u := entities.CurrentUser(ctx)
			if u == nil {
				// For API tokens, the organization is already set in WithCurrentAPITokenAndOrgMiddleware
				return handler(ctx, req)
			}

			orgName, err := entities.GetOrganizationNameFromHeader(ctx)
			if err != nil {
				return nil, fmt.Errorf("error getting organization name: %w", err)
			}

			// Extract organization from resource metadata, takes precedence over header
			if orgFromResource, err := getFromResource(req); err != nil {
				return nil, fmt.Errorf("organization from resource: %w", err)
			} else if orgFromResource != "" {
				orgName = orgFromResource
			}

			if orgName != "" {
				ctx, err = setCurrentMembershipFromOrgName(ctx, u, orgName, userUseCase, orgUC)
				if err != nil {
					return nil, v1.ErrorUserNotMemberOfOrgErrorNotInOrg("user is not a member of organization %s", orgName)
				}
			} else {
				// If no organization name is provided, we use the DB to find the current organization
				// DEPRECATED: in favor of header based org selection
				ctx, err = setCurrentOrganizationFromDB(ctx, u, userUseCase, logger)
				if err != nil {
					return nil, fmt.Errorf("error setting current org: %w", err)
				}
			}

			org := entities.CurrentOrg(ctx)
			if org == nil {
				return nil, errors.New("org not found")
			}

			logger.Infow("msg", "[authN] processed organization", "org-id", org.ID, "credentials type", "user")

			return handler(ctx, req)
		}
	}
}

// setCurrentMembershipsForUser retrieves all user memberships for RBAC
func setCurrentMembershipsForUser(ctx context.Context, u *entities.User, membershipUC biz.MembershipsRBAC) (context.Context, error) {
	var membership *entities.Membership
	var ok bool

	if membership, ok = membershipsCache.Get(u.ID); !ok {
		uid, err := uuid.Parse(u.ID)
		if err != nil {
			return nil, err
		}

		mm, err := membershipUC.ListAllMembershipsForUser(ctx, uid)
		if err != nil {
			return nil, fmt.Errorf("error getting membership list: %w", err)
		}

		resourceMemberships := make([]*entities.ResourceMembership, 0, len(mm))
		for _, m := range mm {
			resourceMemberships = append(resourceMemberships, &entities.ResourceMembership{
				Role:         m.Role,
				ResourceType: m.ResourceType,
				ResourceID:   m.ResourceID,
				MembershipID: m.ID,
			})
		}

		membership = &entities.Membership{UserID: uuid.MustParse(u.ID), Resources: resourceMemberships}
		membershipsCache.Add(u.ID, membership)
	}

	return entities.WithMembership(ctx, membership), nil
}

func ResetMembershipsCache() {
	membershipsCache.Purge()
}

func setCurrentMembershipFromOrgName(ctx context.Context, user *entities.User, orgName string, userUC biz.UserOrgFinder, orgUC *biz.OrganizationUseCase) (context.Context, error) {
	membership, err := userUC.MembershipInOrg(ctx, user.ID, orgName)
	if err != nil && !biz.IsNotFound(err) {
		return nil, fmt.Errorf("failed to find membership: %w", err)
	}

	var role authz.Role
	if membership == nil {
		// if not found, check if the user is instance admin
		ctx, err = setMembershipIfInstanceAdmin(ctx, orgName, orgUC)
		if err != nil {
			return nil, err
		}
		role = authz.RoleInstanceAdmin
	} else {
		role = membership.Role
		ctx = entities.WithCurrentOrg(ctx, &entities.Org{Name: membership.Org.Name, ID: membership.Org.ID, CreatedAt: membership.CreatedAt})
	}

	// Set the authorization subject that will be used to check the policies
	return WithAuthzSubject(ctx, string(role)), nil
}

// sets membership to any organization if the user is an instance admin
func setMembershipIfInstanceAdmin(ctx context.Context, orgName string, orgUC *biz.OrganizationUseCase) (context.Context, error) {
	// look for user membership with instance admin role
	m := entities.CurrentMembership(ctx)
	if m != nil {
		if slices.ContainsFunc(m.Resources, func(r *entities.ResourceMembership) bool {
			return r.Role == authz.RoleInstanceAdmin && r.ResourceType == authz.ResourceTypeInstance
		}) {
			org, err := orgUC.FindByName(ctx, orgName)
			if err != nil {
				return nil, fmt.Errorf("failed to find organization: %w", err)
			}
			ctx = entities.WithCurrentOrg(ctx, &entities.Org{Name: org.Name, ID: org.ID, CreatedAt: org.CreatedAt})
		}
	} else {
		// if no membership and no instance admin, return error
		return nil, errors.New("user membership not found")
	}

	return ctx, nil
}

// Find the current membership of the user and sets it on the context
func setCurrentOrganizationFromDB(ctx context.Context, user *entities.User, userUC biz.UserOrgFinder, logger *log.Helper) (context.Context, error) {
	// We load the current organization
	membership, err := userUC.CurrentMembership(ctx, user.ID)
	if err != nil {
		if biz.IsNotFound(err) {
			return nil, v1.ErrorUserWithNoMembershipErrorNotInOrg("user with id %s has no current organization", user.ID)
		}

		return nil, err
	}

	if membership == nil {
		logger.Warnf("user with id %s has no current organization", user.ID)
		return nil, errors.New("org not found")
	}

	ctx = entities.WithCurrentOrg(ctx, &entities.Org{Name: membership.Org.Name, ID: membership.Org.ID, CreatedAt: membership.CreatedAt})

	// Set the authorization subject that will be used to check the policies
	ctx = WithAuthzSubject(ctx, string(membership.Role))

	return ctx, nil
}

// Gets organization from resource metadata
// The metadata organization field acts as a namespace for organization resources
func getFromResource(req interface{}) (string, error) {
	if req == nil {
		return "", nil
	}

	switch v := req.(type) {
	case *v1.WorkflowContractServiceCreateRequest, *v1.WorkflowContractServiceUpdateRequest:
		return extractOrg(v)
	}

	return "", nil
}

type ResourceBase struct {
	Metadata struct {
		Organization string `json:"organization"`
	} `json:"metadata"`
}

// Extracts organization from request with raw contract data
func extractOrg(req interface{}) (string, error) {
	// Get raw data
	rawData, err := getRawData(req)
	if err != nil {
		return "", err
	}

	if len(rawData) == 0 {
		return "", nil
	}

	// Identify format
	format, err := unmarshal.IdentifyFormat(rawData)
	if err != nil {
		return "", err
	}

	jsonData, err := unmarshal.LoadJSONBytes(rawData, "."+string(format))
	if err != nil {
		return "", err
	}

	// Unmarshal to extract organization
	var resourceBase ResourceBase
	if err := json.Unmarshal(jsonData, &resourceBase); err != nil {
		// If unmarshaling fails, return empty string (no error)
		// This allows old format schemas to work without the metadata field
		return "", nil
	}

	return resourceBase.Metadata.Organization, nil
}

type RequestWithRawContract interface {
	GetRawContract() []byte
}

// Extracts raw data
func getRawData(req interface{}) ([]byte, error) {
	// Check if the request implements RequestWithRawContract
	if rawContractReq, ok := req.(RequestWithRawContract); ok {
		return rawContractReq.GetRawContract(), nil
	}

	return nil, fmt.Errorf("request does not have raw contract")
}
