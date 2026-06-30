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

package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	// Blank import so every controlplane.v1 file descriptor registers in the global
	// registry before wireProcedures() enumerates them.
	_ "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

// TestServerOperationsMapKeysMatchWireProcedures asserts that every ServerOperationsMap key matches
// a real controlplane.v1 wire procedure ("/controlplane.v1.Service/Method"). A typo'd or stale key
// is silently unenforced: the authz middleware derives the same operation string from the request,
// so the procedure the key was meant to gate never matches and falls through to policiesLookup's
// "operation not allowed" deny. That denies a legitimate endpoint to every non-admin role while the
// stale key protects nothing — exactly how CASRedirectService/DownloadRedirect (renamed to
// GetDownloadURL) lingered as dead weight.
func TestServerOperationsMapKeysMatchWireProcedures(t *testing.T) {
	valid := wireProcedures(t)

	for key := range ServerOperationsMap {
		assert.Containsf(t, valid, key,
			"ServerOperationsMap key %q does not match any controlplane.v1 Service/Method — fix the key to the "+
				"real wire procedure or remove it. An unmatched key is silently unenforced, and the procedure it "+
				"was meant to gate falls through to the policiesLookup deny.", key)
	}
}

// authzExemptProcedures lists controlplane.v1 procedures that never reach the authorization
// middleware (WithAuthzMiddleware) because the selector matchers in internal/server/grpc.go skip
// them: currentUserSkipRegexp exempts the attestation/robot-account and public surfaces, which
// carry their own auth. These procedures are therefore neither role-gated via ServerOperationsMap
// nor admin-only. Keep this in sync with currentUserSkipRegexp in
// app/controlplane/internal/server/grpc.go.
var authzExemptProcedures = map[string]struct{}{
	// AttestationService — robot-account / API-token authenticated, gated by requireRobotAccountMatcher.
	"/controlplane.v1.AttestationService/Cancel":               {},
	"/controlplane.v1.AttestationService/FindOrCreateWorkflow": {},
	"/controlplane.v1.AttestationService/GetContract":          {},
	"/controlplane.v1.AttestationService/GetPolicy":            {},
	"/controlplane.v1.AttestationService/GetPolicyGroup":       {},
	"/controlplane.v1.AttestationService/GetUploadCreds":       {},
	"/controlplane.v1.AttestationService/Init":                 {},
	"/controlplane.v1.AttestationService/Store":                {},
	"/controlplane.v1.AttestationStateService/Initialized":     {},
	"/controlplane.v1.AttestationStateService/Read":            {},
	"/controlplane.v1.AttestationStateService/Reset":           {},
	"/controlplane.v1.AttestationStateService/Save":            {},
	"/controlplane.v1.SigningService/GenerateSigningCert":      {},
	"/controlplane.v1.SigningService/GetTrustedRoot":           {},
	"/controlplane.v1.StatusService/Infoz":                     {},
	"/controlplane.v1.StatusService/Statusz":                   {},
	"/controlplane.v1.ReferrerService/DiscoverPublicShared":    {},
}

// adminOnlyProcedures lists controlplane.v1 procedures that reach the authorization middleware but
// are intentionally absent from ServerOperationsMap. An unmapped operation fails policiesLookup and
// is denied for every non-admin role, so these are reachable only by org admins via the global
// IsAdmin() bypass in middleware.go (the TODO there). Listing a procedure here is an explicit
// acknowledgement that it is admin-only today; if a procedure should be reachable by a non-admin
// role, move it into ServerOperationsMap instead of adding it here.
var adminOnlyProcedures = map[string]struct{}{
	// CAS backend mutations (Create/List/Revalidate are role-gated in ServerOperationsMap).
	"/controlplane.v1.CASBackendService/Delete": {},
	"/controlplane.v1.CASBackendService/Update": {},

	// Group catalog mutations (List/Get and membership ops are role-open in ServerOperationsMap).
	"/controlplane.v1.GroupService/Create": {},
	"/controlplane.v1.GroupService/Delete": {},
	"/controlplane.v1.GroupService/Update": {},

	// Integration deregistration (Register/Attach/Detach are role-gated in ServerOperationsMap).
	"/controlplane.v1.IntegrationsService/Deregister": {},

	// Org invitation management (Create is role-gated in ServerOperationsMap).
	"/controlplane.v1.OrgInvitationService/ListSent": {},
	"/controlplane.v1.OrgInvitationService/Revoke":   {},

	// Organization and membership mutations (Create/Delete/ListMemberships are handled separately).
	"/controlplane.v1.OrganizationService/DeleteMembership": {},
	"/controlplane.v1.OrganizationService/Update":           {},
	"/controlplane.v1.OrganizationService/UpdateMembership": {},

	// Robot account revocation (List/Create are role-gated in ServerOperationsMap).
	"/controlplane.v1.RobotAccountService/Revoke": {},
}

// TestEveryProcedureIsClassified asserts that every controlplane.v1 wire procedure is explicitly
// classified — mapped in ServerOperationsMap (role-gated), listed in adminOnlyProcedures
// (intentionally admin-only by omission), or listed in authzExemptProcedures (skips the
// authorization middleware). An unclassified procedure silently defaults to admin-only (non-admins
// denied by policiesLookup), so a newly added RPC cannot quietly become inaccessible to the roles
// that should reach it without someone making — and a reviewer seeing — an explicit decision here.
func TestEveryProcedureIsClassified(t *testing.T) {
	valid := wireProcedures(t)

	for proc := range valid {
		_, mapped := ServerOperationsMap[proc]
		_, adminOnly := adminOnlyProcedures[proc]
		_, exempt := authzExemptProcedures[proc]
		assert.Truef(t, mapped || adminOnly || exempt,
			"procedure %q is unclassified: add it to ServerOperationsMap to make it reachable by non-admin "+
				"roles, to adminOnlyProcedures to confirm it is intentionally admin-only, or to "+
				"authzExemptProcedures if it bypasses the authorization middleware. Unclassified procedures "+
				"default to admin-only (non-admins denied).", proc)
	}

	// Guard against stale list entries and against a procedure being classified in more than one bucket.
	for proc := range adminOnlyProcedures {
		assert.Containsf(t, valid, proc, "adminOnlyProcedures entry %q matches no controlplane.v1 procedure; remove it", proc)
		_, mapped := ServerOperationsMap[proc]
		assert.Falsef(t, mapped, "procedure %q is in both ServerOperationsMap and adminOnlyProcedures; keep it in one bucket", proc)
	}

	for proc := range authzExemptProcedures {
		assert.Containsf(t, valid, proc, "authzExemptProcedures entry %q matches no controlplane.v1 procedure; remove it", proc)
		_, mapped := ServerOperationsMap[proc]
		assert.Falsef(t, mapped, "procedure %q is in both ServerOperationsMap and authzExemptProcedures; keep it in one bucket", proc)
		_, adminOnly := adminOnlyProcedures[proc]
		assert.Falsef(t, adminOnly, "procedure %q is in both authzExemptProcedures and adminOnlyProcedures; keep it in one bucket", proc)
	}
}

// wireProcedures returns the set of "/controlplane.v1.Service/Method" identifiers for every service
// in the controlplane.v1 proto package, mirroring both the operation string the authz middleware
// derives at runtime and the key shape used in ServerOperationsMap.
func wireProcedures(t *testing.T) map[string]struct{} {
	t.Helper()

	procedures := make(map[string]struct{})
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if fd.Package() != "controlplane.v1" {
			return true
		}
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				procedures["/controlplane.v1."+string(svc.Name())+"/"+string(methods.Get(j).Name())] = struct{}{}
			}
		}
		return true
	})

	require.NotEmpty(t, procedures, "no controlplane.v1 wire procedures registered — is the generated package imported?")
	return procedures
}
