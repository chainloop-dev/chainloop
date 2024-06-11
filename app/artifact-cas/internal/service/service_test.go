//
// Copyright 2023 The Chainloop Authors.
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
	"testing"

	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/mocks"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	jwtm "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
