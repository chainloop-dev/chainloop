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

package main

import (
	"testing"

	kratosErrors "github.com/go-kratos/kratos/v2/errors"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toGRPCStatus converts a Kratos *errors.Error into a *status.Status the same
// way it would travel over the wire: Kratos Error -> GRPCStatus() -> gRPC Status.
func toGRPCStatus(err *kratosErrors.Error) *status.Status {
	return err.GRPCStatus()
}

func TestIsWrappedErr(t *testing.T) {
	tests := []struct {
		name   string
		actual *kratosErrors.Error // the error that "came over the wire"
		target *kratosErrors.Error // the sentinel we are checking against
		want   bool
	}{
		{
			name:   "ErrTokenExpired matches itself",
			actual: jwtMiddleware.ErrTokenExpired,
			target: jwtMiddleware.ErrTokenExpired,
			want:   true,
		},
		{
			name:   "ErrTokenInvalid matches itself",
			actual: jwtMiddleware.ErrTokenInvalid,
			target: jwtMiddleware.ErrTokenInvalid,
			want:   true,
		},
		{
			name:   "ErrTokenParseFail matches itself",
			actual: jwtMiddleware.ErrTokenParseFail,
			target: jwtMiddleware.ErrTokenParseFail,
			want:   true,
		},
		{
			name:   "ErrMissingJwtToken matches itself",
			actual: jwtMiddleware.ErrMissingJwtToken,
			target: jwtMiddleware.ErrMissingJwtToken,
			want:   true,
		},
		{
			name:   "ErrTokenExpired does NOT match ErrTokenInvalid",
			actual: jwtMiddleware.ErrTokenExpired,
			target: jwtMiddleware.ErrTokenInvalid,
			want:   false,
		},
		{
			name:   "ErrTokenInvalid does NOT match ErrTokenExpired",
			actual: jwtMiddleware.ErrTokenInvalid,
			target: jwtMiddleware.ErrTokenExpired,
			want:   false,
		},
		{
			name:   "ErrTokenParseFail does NOT match ErrTokenExpired",
			actual: jwtMiddleware.ErrTokenParseFail,
			target: jwtMiddleware.ErrTokenExpired,
			want:   false,
		},
		{
			name:   "ErrTokenExpired does NOT match ErrMissingJwtToken",
			actual: jwtMiddleware.ErrTokenExpired,
			target: jwtMiddleware.ErrMissingJwtToken,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := toGRPCStatus(tt.actual)
			got := isWrappedErr(st, tt.target)
			assert.Equal(t, tt.want, got, "isWrappedErr(%v, %v)", tt.actual.Message, tt.target.Message)
		})
	}
}

func TestIsUnmatchedAuthErr(t *testing.T) {
	tests := []struct {
		name string
		st   *status.Status
		want bool
	}{
		{
			name: "generic 401/Unauthenticated error is caught",
			st:   status.New(codes.Unauthenticated, "some auth error"),
			want: true,
		},
		{
			name: "JWT token expired (Unauthenticated) is caught",
			st:   toGRPCStatus(jwtMiddleware.ErrTokenExpired),
			want: true,
		},
		{
			name: "PermissionDenied is NOT caught",
			st:   status.New(codes.PermissionDenied, "forbidden"),
			want: false,
		},
		{
			name: "OK status is NOT caught",
			st:   status.New(codes.OK, ""),
			want: false,
		},
		{
			name: "Internal error is NOT caught",
			st:   status.New(codes.Internal, "internal server error"),
			want: false,
		},
		{
			name: "NotFound is NOT caught",
			st:   status.New(codes.NotFound, "not found"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnmatchedAuthErr(tt.st)
			assert.Equal(t, tt.want, got, "isUnmatchedAuthErr()")
		})
	}
}

// TestKratosErrorsIsMasksJWTErrors demonstrates the bug that motivates our
// Message-based comparison: Kratos errors.Is() only compares Code and Reason.
// Since all JWT errors share Code=401 and Reason="UNAUTHORIZED", errors.Is()
// incorrectly matches ANY JWT error against ANY other JWT sentinel.
func TestKratosErrorsIsMasksJWTErrors(t *testing.T) {
	// This test proves that the naive errors.Is approach is broken:
	// ErrTokenExpired would incorrectly match ErrTokenInvalid via Kratos errors.Is.
	if !kratosErrors.Is(jwtMiddleware.ErrTokenExpired, jwtMiddleware.ErrTokenInvalid) {
		t.Skip("Kratos errors.Is behavior has changed; this test documents the original masking bug")
	}

	// Now verify that our isWrappedErr correctly distinguishes them
	st := toGRPCStatus(jwtMiddleware.ErrTokenExpired)
	assert.False(t, isWrappedErr(st, jwtMiddleware.ErrTokenInvalid),
		"isWrappedErr should NOT match ErrTokenExpired against ErrTokenInvalid")
	assert.True(t, isWrappedErr(st, jwtMiddleware.ErrTokenExpired),
		"isWrappedErr should match ErrTokenExpired against itself")
}

// TestIsWrappedErrGRPCWireRoundTrip verifies that isWrappedErr works after a
// full gRPC wire round-trip: KratosError -> GRPCStatus -> proto bytes -> gRPC
// status.FromError -> isWrappedErr. This simulates the actual path an error
// takes from the server through a gRPC transport to the CLI client.
func TestIsWrappedErrGRPCWireRoundTrip(t *testing.T) {
	sentinels := []*kratosErrors.Error{
		jwtMiddleware.ErrTokenExpired,
		jwtMiddleware.ErrTokenInvalid,
		jwtMiddleware.ErrTokenParseFail,
		jwtMiddleware.ErrMissingJwtToken,
	}

	for _, sentinel := range sentinels {
		t.Run(sentinel.Message, func(t *testing.T) {
			// Step 1: Convert Kratos error to gRPC status (server side)
			grpcSt := sentinel.GRPCStatus()

			// Step 2: Serialize to the wire format (proto bytes)
			proto := grpcSt.Proto()
			require.NotNil(t, proto, "gRPC status proto should not be nil")

			// Step 3: Deserialize from proto back into a gRPC status (client side)
			roundTripped := status.FromProto(proto)
			require.NotNil(t, roundTripped, "round-tripped status should not be nil")

			// Step 4: Verify isWrappedErr still matches after the full round-trip
			assert.True(t, isWrappedErr(roundTripped, sentinel),
				"isWrappedErr should match %q after gRPC wire round-trip", sentinel.Message)

			// Step 5: Verify it does NOT match a different sentinel after round-trip
			for _, other := range sentinels {
				if other == sentinel {
					continue
				}
				assert.False(t, isWrappedErr(roundTripped, other),
					"isWrappedErr should NOT match %q against %q after round-trip",
					sentinel.Message, other.Message)
			}
		})
	}
}

// TestJWTSentinelMessageCanary asserts the exact Message strings of the JWT
// sentinel errors we depend on. If a Kratos update changes these strings
// (e.g. ErrTokenParseFail's trailing space), this test will fail and alert us
// that our Message-based matching in isWrappedErr needs updating.
func TestJWTSentinelMessageCanary(t *testing.T) {
	tests := []struct {
		name    string
		err     *kratosErrors.Error
		wantMsg string
	}{
		{
			name:    "ErrTokenExpired",
			err:     jwtMiddleware.ErrTokenExpired,
			wantMsg: "JWT token has expired",
		},
		{
			name:    "ErrTokenInvalid",
			err:     jwtMiddleware.ErrTokenInvalid,
			wantMsg: "Token is invalid",
		},
		{
			name:    "ErrTokenParseFail (note trailing space)",
			err:     jwtMiddleware.ErrTokenParseFail,
			wantMsg: "Fail to parse JWT token ", // trailing space is intentional
		},
		{
			name:    "ErrMissingJwtToken",
			err:     jwtMiddleware.ErrMissingJwtToken,
			wantMsg: "JWT token is missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantMsg, tt.err.Message,
				"Kratos sentinel %s Message has changed — update isWrappedErr matching and case branches in errorInfo", tt.name)
		})
	}
}
