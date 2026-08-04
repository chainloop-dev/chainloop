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

package service

import (
	"context"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateIsPinnedToCurrentOrg is a regression test for CP-N1. The authz
// middleware evaluates the caller's role against the organization selected in
// the request headers, so Update must refuse to operate on any other
// organization. Otherwise an admin of org A could target org B by naming it in
// the request body.
func TestUpdateIsPinnedToCurrentOrg(t *testing.T) {
	// A nil use case is deliberate: a request that reaches the biz layer means
	// the guard did not run, and the test fails loudly instead of silently
	// passing.
	svc := NewOrganizationService(nil, nil)

	ctxWithOrg := func(orgName string) context.Context {
		ctx := entities.WithCurrentUser(context.Background(), &entities.User{ID: uuid.NewString(), Email: "user@test.com"})
		return entities.WithCurrentOrg(ctx, &entities.Org{ID: uuid.NewString(), Name: orgName})
	}

	testCases := []struct {
		name       string
		currentOrg string
		reqName    string
	}{
		{name: "different organization", currentOrg: "my-org", reqName: "victim-org"},
		{name: "empty name", currentOrg: "my-org", reqName: ""},
		{name: "case variation", currentOrg: "my-org", reqName: "My-Org"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.Update(ctxWithOrg(tc.currentOrg), &pb.OrganizationServiceUpdateRequest{
				Name:                   tc.reqName,
				BlockOnPolicyViolation: toPtrBool(false),
			})

			require.Error(t, err)
			assert.Nil(t, got)
			assert.True(t, errors.IsForbidden(err), "want forbidden, got %v", err)
		})
	}
}

func toPtrBool(b bool) *bool {
	return &b
}
