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
	"net"
	"os"
	"syscall"
	"testing"

	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/mocks"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	jwtm "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestInfoFromAuth(t *testing.T) {
	testCases := []struct {
		name string
		// input
		claims  jwt.Claims
		wantErr bool
	}{
		{
			name: "valid claims downloader",
			claims: &casJWT.Claims{
				Role:           casJWT.Downloader,
				StoredSecretID: "test",
				BackendType:    "backend-type",
			},
		},
		{
			name: "valid claims uploader",
			claims: &casJWT.Claims{
				Role:           casJWT.Uploader,
				StoredSecretID: "test",
				BackendType:    "backend-type",
			},
		},
		{
			name: "invalid role",
			claims: &casJWT.Claims{
				Role:           "invalid",
				StoredSecretID: "test",
				BackendType:    "backend-type",
			},
			wantErr: true,
		},
		{
			name: "missing secretID",
			claims: &casJWT.Claims{
				Role:        "test",
				BackendType: "backend-type",
			},
			wantErr: true,
		},
		{
			name: "missing role",
			claims: &casJWT.Claims{
				StoredSecretID: "test",
				BackendType:    "backend-type",
			},
			wantErr: true,
		},
		{
			name: "missing backend type",
			claims: &casJWT.Claims{
				StoredSecretID: "test",
				Role:           "test",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := infoFromAuth(jwtm.NewContext(context.Background(), tc.claims))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.claims, info)
		})
	}
}

func TestLoadBackend(t *testing.T) {
	testCases := []struct {
		name string
		// input
		providerType string
		secretID     string
		wantErr      bool
		is404Err     bool
	}{
		{
			name:         "valid",
			providerType: "test",
			secretID:     "test",
		},
		{
			name:         "invalid provider type",
			providerType: "invalid",
			wantErr:      true,
			is404Err:     true,
		},
		{
			name:         "invalid secretID",
			providerType: "test",
			secretID:     "invalid",
			wantErr:      true,
		},
	}

	backendProvider := mocks.NewProvider(t)
	b := mocks.NewUploaderDownloader(t)
	backendProvider.On("FromCredentials", mock.Anything, "test").Maybe().Return(b, nil)
	backendProvider.On("FromCredentials", mock.Anything, "invalid").Maybe().Return(nil, errors.New("backend not found"))
	// Initialize common service with backends
	providers := backend.Providers{
		"test": backendProvider,
	}

	s := newCommonService(providers)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.loadBackend(context.Background(), tc.providerType, tc.secretID)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.is404Err, kerrors.IsNotFound(err))
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, b, got)
		})
	}
}

func TestIsClientDisconnect(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "context canceled",
			err:  context.Canceled,
			want: true,
		},
		{
			name: "wrapped context canceled",
			err:  fmt.Errorf("download failed: %w", context.Canceled),
			want: true,
		},
		{
			name: "grpc canceled status",
			err:  status.Error(codes.Canceled, "canceled"),
			want: true,
		},
		{
			name: "connection reset by peer (syscall)",
			err: &net.OpError{
				Op:  "write",
				Net: "tcp",
				Err: &os.SyscallError{
					Syscall: "write",
					Err:     syscall.ECONNRESET,
				},
			},
			want: true,
		},
		{
			name: "broken pipe (syscall)",
			err: &net.OpError{
				Op:  "write",
				Net: "tcp",
				Err: &os.SyscallError{
					Syscall: "write",
					Err:     syscall.EPIPE,
				},
			},
			want: true,
		},
		{
			name: "wrapped connection reset",
			err: fmt.Errorf("copying data: %w", &net.OpError{
				Op:  "write",
				Net: "tcp",
				Err: &os.SyscallError{
					Syscall: "write",
					Err:     syscall.ECONNRESET,
				},
			}),
			want: true,
		},
		{
			name: "generic error",
			err:  errors.New("something went wrong"),
			want: false,
		},
		{
			name: "grpc internal error",
			err:  status.Error(codes.Internal, "internal"),
			want: false,
		},
		{
			name: "grpc unavailable",
			err:  status.Error(codes.Unavailable, "unavailable"),
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isClientDisconnect(tc.err)
			assert.Equal(t, tc.want, got)
		})
	}
}
