//
// Copyright 2023-2026 The Chainloop Authors.
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
	"errors"
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maskedClientMsg is the generic message servicelogger.LogAndMaskErr returns to
// the client, hiding the real (potentially sensitive) server-side error.
const maskedClientMsg = "server error"

func TestHandleUseCaseErr(t *testing.T) {
	t.Parallel()

	immutableErr := status.Error(codes.FailedPrecondition, `version "v1.83.2+next" is released and immutable: attestations cannot be added`)

	testCases := []struct {
		name        string
		err         error
		wantCode    codes.Code
		wantMessage string
	}{
		{
			name:        "failed precondition status error is propagated",
			err:         immutableErr,
			wantCode:    codes.FailedPrecondition,
			wantMessage: `version "v1.83.2+next" is released and immutable: attestations cannot be added`,
		},
		{
			name:        "wrapped failed precondition keeps code and original message",
			err:         fmt.Errorf("saving attestation digest: %w", immutableErr),
			wantCode:    codes.FailedPrecondition,
			wantMessage: `version "v1.83.2+next" is released and immutable: attestations cannot be added`,
		},
		{
			name:        "released version immutable biz error maps to failed precondition",
			err:         fmt.Errorf("saving attestation digest: %w", biz.NewErrReleasedVersionImmutable("v1.83.2+next")),
			wantCode:    codes.FailedPrecondition,
			wantMessage: `saving attestation digest: version "v1.83.2+next" is released and immutable: attestations cannot be added`,
		},
		{
			name:        "already converted error is propagated unchanged when processed again",
			err:         handleUseCaseErr(fmt.Errorf("saving attestation digest: %w", biz.NewErrReleasedVersionImmutable("v1.83.2+next")), nil),
			wantCode:    codes.FailedPrecondition,
			wantMessage: `saving attestation digest: version "v1.83.2+next" is released and immutable: attestations cannot be added`,
		},
		{
			name:        "already converted not found error is propagated unchanged when processed again",
			err:         handleUseCaseErr(biz.NewErrNotFound("workflow"), nil),
			wantCode:    codes.NotFound,
			wantMessage: "workflow not found",
		},
		{
			name:        "already converted already exists error is propagated unchanged when processed again",
			err:         handleUseCaseErr(biz.NewErrAlreadyExists(errors.New("name taken")), nil),
			wantCode:    codes.AlreadyExists,
			wantMessage: "duplicated: name taken",
		},
		{
			name:        "server-side status error is still masked",
			err:         status.Error(codes.Unavailable, "connection to database lost"),
			wantCode:    codes.Internal,
			wantMessage: maskedClientMsg,
		},
		{
			name:        "validation error maps to bad request",
			err:         biz.NewErrValidationStr("invalid input"),
			wantCode:    codes.InvalidArgument,
			wantMessage: "validation error: invalid input",
		},
		{
			name:        "unknown error is masked as internal server error",
			err:         errors.New("sensitive details"),
			wantCode:    codes.Internal,
			wantMessage: maskedClientMsg,
		},
		{
			// PFM-6775: a transient org-lookup failure now surfaces the real DB
			// cause up the stack (wrapped with %w by the data and biz layers).
			// It must be masked as a generic internal error so the SQL detail
			// never reaches the client, while the real error remains available
			// for logging/Sentry.
			name: "transient org-lookup failure is masked, SQL detail not leaked to client",
			err: fmt.Errorf("failed to find membership: %w",
				fmt.Errorf("querying organization %q: %w", "chainloop",
					errors.New("pq: canceling statement due to statement timeout (SQLSTATE 57014)"))),
			wantCode:    codes.Internal,
			wantMessage: maskedClientMsg,
		},
		{
			// PFM-6775: a genuinely missing org still maps to NotFound (never
			// masked), so ContextService.Current can degrade gracefully.
			name:        "genuinely missing org maps to not found",
			err:         fmt.Errorf("failed to find membership: %w", biz.NewErrNotFound("organization chainloop")),
			wantCode:    codes.NotFound,
			wantMessage: "failed to find membership: organization chainloop not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := handleUseCaseErr(tc.err, nil)
			require.Error(t, got)
			assert.Equal(t, tc.wantCode, status.Code(got))
			assert.Equal(t, tc.wantMessage, kerrors.FromError(got).GetMessage())
		})
	}
}

func TestRequireCurrentUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("no user", func(t *testing.T) {
		_, err := requireCurrentUser(ctx)
		assert.Error(t, err)
	})

	t.Run("with user", func(t *testing.T) {
		want := &entities.User{}
		ctx = entities.WithCurrentUser(ctx, want)
		u, err := requireCurrentUser(ctx)
		assert.NoError(t, err)
		require.Equal(t, want, u)
	})
}

func TestRequireCurrentOrg(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("no org", func(t *testing.T) {
		_, err := requireCurrentOrg(ctx)
		assert.Error(t, err)
	})

	t.Run("with org", func(t *testing.T) {
		want := &entities.Org{}
		ctx = entities.WithCurrentOrg(ctx, want)
		o, err := requireCurrentOrg(ctx)
		assert.NoError(t, err)
		require.Equal(t, want, o)
	})
}

func TestRequireAPIToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("no token", func(t *testing.T) {
		_, err := requireAPIToken(ctx)
		assert.Error(t, err)
	})

	t.Run("with token", func(t *testing.T) {
		want := &entities.APIToken{}
		ctx = entities.WithCurrentAPIToken(ctx, want)
		got, err := requireAPIToken(ctx)
		assert.NoError(t, err)
		require.Equal(t, want, got)
	})
}

func TestRequireCurrentUserOrAPIToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tesCases := []struct {
		name     string
		hasUser  bool
		hasToken bool
		wantErr  bool
	}{
		{
			name:     "no user nor token",
			hasUser:  false,
			hasToken: false,
			wantErr:  true,
		},
		{
			name:     "with user",
			hasUser:  true,
			hasToken: false,
			wantErr:  false,
		},
		{
			name:     "with token",
			hasUser:  false,
			hasToken: true,
			wantErr:  false,
		},
	}

	for _, tc := range tesCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx = context.Background()
			wantUser := &entities.User{}
			wantToken := &entities.APIToken{}

			if tc.hasUser {
				ctx = entities.WithCurrentUser(ctx, wantUser)
			}

			if tc.hasToken {
				ctx = entities.WithCurrentAPIToken(ctx, wantToken)
			}

			gotUser, gotToken, err := requireCurrentUserOrAPIToken(ctx)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tc.hasUser {
				require.Equal(t, wantUser, gotUser)
			} else {
				assert.Nil(t, gotUser)
			}

			if tc.hasToken {
				require.Equal(t, wantToken, gotToken)
			} else {
				assert.Nil(t, gotToken)
			}
		})
	}
}
