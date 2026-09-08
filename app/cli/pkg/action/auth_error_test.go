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

package action

import (
	"errors"
	"fmt"
	"testing"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The messages AuthErrorMessage is expected to render for each Kratos JWT
// sentinel. Spelled out here rather than read from authMessages, so a change to
// what the CLI tells the user has to be made in both places deliberately.
const (
	authMsgExpired   = `your authentication token has expired, please run "chainloop auth login" again`
	authMsgInvalid   = `your authentication token is invalid, please run "chainloop auth login" again`
	authMsgParseFail = `failed to parse authentication token, please run "chainloop auth login" again`
	authMsgMissing   = `authentication required, please run "chainloop auth login"`
)

func TestAuthErrorMessage(t *testing.T) {
	testCases := []struct {
		name string
		err  error

		wantMatch bool
		wantMsg   string
	}{
		{
			name:      "a missing token asks the user to log in",
			err:       jwtMiddleware.ErrMissingJwtToken.GRPCStatus().Err(),
			wantMatch: true,
			wantMsg:   authMsgMissing,
		},
		{
			name:      "an expired token says so",
			err:       jwtMiddleware.ErrTokenExpired.GRPCStatus().Err(),
			wantMatch: true,
			wantMsg:   authMsgExpired,
		},
		{
			name:      "an invalid token says so",
			err:       jwtMiddleware.ErrTokenInvalid.GRPCStatus().Err(),
			wantMatch: true,
			wantMsg:   authMsgInvalid,
		},
		{
			name:      "an unparseable token says so",
			err:       jwtMiddleware.ErrTokenParseFail.GRPCStatus().Err(),
			wantMatch: true,
			wantMsg:   authMsgParseFail,
		},
		{
			// The whole point of matching on the message: every JWT sentinel
			// shares Code 401 and Reason "UNAUTHORIZED", so a Reason-only
			// comparison (kratos errors.Is) would report all of them as the
			// first one. See TestKratosErrorsIsMasksJWTErrors.
			name:      "any other 401 keeps the control plane's own message",
			err:       status.Error(codes.Unauthenticated, "token belongs to a deleted user"),
			wantMatch: true,
			wantMsg:   "authentication error: token belongs to a deleted user",
		},
		{
			// Commands add context on the way up, and grpc's status.FromError
			// would replace the status message with the whole chain, so the
			// status is dug out with errors.As instead.
			name:      "a wrapped authentication error is still matched",
			err:       fmt.Errorf("looking up workflow %q: %w", "ai-coding-session", jwtMiddleware.ErrMissingJwtToken.GRPCStatus().Err()),
			wantMatch: true,
			wantMsg:   authMsgMissing,
		},
		{
			name: "a permission error is not an authentication error",
			err:  status.Error(codes.PermissionDenied, "forbidden"),
		},
		{
			name: "a not-found error is not an authentication error",
			err:  status.Error(codes.NotFound, "not found"),
		},
		{
			name: "a plain error is not an authentication error",
			err:  errors.New("boom"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg, matched := AuthErrorMessage(tc.err)
			assert.Equal(t, tc.wantMatch, matched)
			assert.Equal(t, tc.wantMsg, msg)
		})
	}
}

// TestAuthErrorMessageGRPCWireRoundTrip verifies the mapping survives a full
// gRPC wire round-trip: kratos error -> GRPCStatus -> proto bytes -> status ->
// AuthErrorMessage. This is the path an error actually takes from the server
// through the transport to the CLI.
func TestAuthErrorMessageGRPCWireRoundTrip(t *testing.T) {
	for sentinel, want := range map[*kerrors.Error]string{
		jwtMiddleware.ErrTokenExpired:    authMsgExpired,
		jwtMiddleware.ErrTokenInvalid:    authMsgInvalid,
		jwtMiddleware.ErrTokenParseFail:  authMsgParseFail,
		jwtMiddleware.ErrMissingJwtToken: authMsgMissing,
	} {
		t.Run(sentinel.Message, func(t *testing.T) {
			proto := sentinel.GRPCStatus().Proto()
			require.NotNil(t, proto)

			msg, matched := AuthErrorMessage(status.FromProto(proto).Err())
			require.True(t, matched)
			assert.Equal(t, want, msg, "each sentinel must keep its own message after a wire round-trip")
		})
	}
}

// TestKratosErrorsIsMasksJWTErrors documents the bug that motivates comparing
// the message: kratos errors.Is() only compares Code and Reason, which every
// JWT error shares, so it matches any JWT error against any other sentinel.
func TestKratosErrorsIsMasksJWTErrors(t *testing.T) {
	if !kerrors.Is(jwtMiddleware.ErrTokenExpired, jwtMiddleware.ErrTokenInvalid) {
		t.Skip("Kratos errors.Is behavior has changed; this test documents the original masking bug")
	}

	msg, matched := AuthErrorMessage(jwtMiddleware.ErrTokenExpired.GRPCStatus().Err())
	require.True(t, matched)
	assert.Equal(t, authMsgExpired, msg, "an expired token must not be reported as an invalid one")
}

// TestJWTSentinelMessageCanary asserts the exact Message strings of the JWT
// sentinels the mapping depends on. If a Kratos update changes them, this fails
// and points at authMessages.
func TestJWTSentinelMessageCanary(t *testing.T) {
	for name, tc := range map[string]struct {
		err     *kerrors.Error
		wantMsg string
	}{
		"ErrTokenExpired":                    {jwtMiddleware.ErrTokenExpired, "JWT token has expired"},
		"ErrTokenInvalid":                    {jwtMiddleware.ErrTokenInvalid, "Token is invalid"},
		"ErrTokenParseFail (trailing space)": {jwtMiddleware.ErrTokenParseFail, "Fail to parse JWT token "},
		"ErrMissingJwtToken":                 {jwtMiddleware.ErrMissingJwtToken, "JWT token is missing"},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wantMsg, tc.err.Message,
				"Kratos sentinel %s Message has changed — update authMessages", name)
		})
	}
}
