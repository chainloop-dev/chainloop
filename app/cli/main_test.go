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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsWrappedErr(t *testing.T) {
	tests := []struct {
		name     string
		grpcErr  error
		target   *kratosErrors.Error
		expected bool
	}{
		{
			name:     "expired token matches ErrTokenExpired",
			grpcErr:  jwtMiddleware.ErrTokenExpired,
			target:   jwtMiddleware.ErrTokenExpired,
			expected: true,
		},
		{
			name:     "missing token matches ErrMissingJwtToken",
			grpcErr:  jwtMiddleware.ErrMissingJwtToken,
			target:   jwtMiddleware.ErrMissingJwtToken,
			expected: true,
		},
		{
			name:     "invalid token does NOT match ErrTokenExpired",
			grpcErr:  jwtMiddleware.ErrTokenInvalid,
			target:   jwtMiddleware.ErrTokenExpired,
			expected: false,
		},
		{
			name:     "missing token does NOT match ErrTokenExpired",
			grpcErr:  jwtMiddleware.ErrMissingJwtToken,
			target:   jwtMiddleware.ErrTokenExpired,
			expected: false,
		},
		{
			name:     "expired token does NOT match ErrMissingJwtToken",
			grpcErr:  jwtMiddleware.ErrTokenExpired,
			target:   jwtMiddleware.ErrMissingJwtToken,
			expected: false,
		},
		{
			name:     "generic unauthorized does NOT match ErrTokenExpired",
			grpcErr:  kratosErrors.Unauthorized("UNAUTHORIZED", "some other error"),
			target:   jwtMiddleware.ErrTokenExpired,
			expected: false,
		},
		{
			name:     "non-auth gRPC error does NOT match ErrTokenExpired",
			grpcErr:  status.Error(codes.Internal, "internal error"),
			target:   jwtMiddleware.ErrTokenExpired,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st, _ := status.FromError(tc.grpcErr)
			got := isWrappedErr(st, tc.target)
			assert.Equal(t, tc.expected, got)
		})
	}
}
