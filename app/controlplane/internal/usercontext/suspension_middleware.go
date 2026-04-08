package usercontext

import (
	"context"
	"regexp"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	errorsAPI "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// readOnlyActions are policy actions considered safe during suspension.
var readOnlyActions = map[string]bool{
	authz.ActionRead: true,
	authz.ActionList: true,
}

// suspensionExemptRegexp matches operations that are allowed when suspended even though
// they cannot be classified as read-only via ServerOperationsMap policies.
// This covers two cases:
//   - Empty-policy reads: operations mapped with {} in ServerOperationsMap that are actually reads
//   - Self-service writes: operations the user needs to leave/delete even when suspended
var suspensionExemptRegexp = regexp.MustCompile(`controlplane.v1.CASCredentialsService/Get|controlplane.v1.UserService/(ListMemberships|SetCurrentMembership|DeleteMembership)|controlplane.v1.GroupService/(List|Get|ListMembers|ListProjects|ListPendingInvitations)|controlplane.v1.AuthService/DeleteAccount|controlplane.v1.OrganizationService/Delete$`)

// WithSuspensionMiddleware blocks write operations when the current organization is suspended.
//
// Classification logic:
//  1. If the operation has policies in ServerOperationsMap with only read/list actions -> allow
//  2. If the operation matches suspensionExemptRegexp -> allow
//  3. Everything else (writes, empty policies, unmapped operations) -> block
func WithSuspensionMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			org := entities.CurrentOrg(ctx)
			// No org context (e.g., user info, status endpoints), pass through
			if org == nil {
				return handler(ctx, req)
			}

			if !org.Suspended {
				return handler(ctx, req)
			}

			// Org is suspended, determine if the operation is read-only
			t, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, suspendedError()
			}

			apiOperation := t.Operation()

			// Check exemptions first
			if suspensionExemptRegexp.MatchString(apiOperation) {
				return handler(ctx, req)
			}

			// Look up policies in ServerOperationsMap
			policies, err := authz.PoliciesLookup(apiOperation)
			if err != nil {
				// Operation not in ServerOperationsMap, block by default
				return nil, suspendedError()
			}

			// Empty policies, no action metadata to classify as read-only, block
			if len(policies) == 0 {
				return nil, suspendedError()
			}

			// Allow only if ALL policy actions are read-only
			for _, p := range policies {
				if !readOnlyActions[p.Action] {
					return nil, suspendedError()
				}
			}

			return handler(ctx, req)
		}
	}
}

func suspendedError() error {
	return errorsAPI.Forbidden("suspended", "organization is suspended, read-only access only")
}
