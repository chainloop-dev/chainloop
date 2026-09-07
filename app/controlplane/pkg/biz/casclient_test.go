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

package biz_test

import (
	"context"
	"errors"
	"testing"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIsReady(t *testing.T) {
	validConf := &conf.Bootstrap_CASServer{
		Grpc: &conf.Server_GRPC{Addr: "localhost:1111"},
	}

	testCases := []struct {
		name     string
		config   *conf.Bootstrap_CASServer
		casReady bool
		want     bool
		wantErr  bool
	}{
		{
			name:    "missing configuration",
			config:  &conf.Bootstrap_CASServer{},
			wantErr: true,
		},
		{
			name:    "invalid configuration",
			config:  &conf.Bootstrap_CASServer{Grpc: &conf.Server_GRPC{}},
			wantErr: true,
		},
		{
			name:    "not ready configuration",
			config:  validConf,
			wantErr: false,
		},
		{
			name:     "ready configuration",
			config:   validConf,
			casReady: true,
			want:     true,
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			clientProvider := func(_ *conf.Bootstrap_CASServer, _ string) (casclient.DownloaderUploader, func(), error) {
				c := mocks.NewDownloaderUploader(t)
				c.On("IsReady", mock.Anything).Return(tc.casReady, nil)
				return c, func() {}, nil
			}
			uc := biz.NewCASClientUseCase(nil, tc.config, nil, biz.WithClientFactory(clientProvider))

			got, err := uc.IsReady(context.Background())
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDescribe(t *testing.T) {
	const digest = "sha256:cf4c9c8b7b1b4f4d0b4e3f4a5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d"

	validConf := &conf.Bootstrap_CASServer{
		Grpc: &conf.Server_GRPC{Addr: "localhost:1111"},
	}

	testCases := []struct {
		name     string
		orgID    uuid.UUID
		casInfo  *casclient.ResourceInfo
		casErr   error
		want     *casclient.ResourceInfo
		wantErr  bool
		wantCall bool
	}{
		{
			name:     "returns the resource metadata reported by the CAS",
			orgID:    uuid.New(),
			casInfo:  &casclient.ResourceInfo{Digest: digest, Filename: "policy-evaluations.json", Size: 2048},
			want:     &casclient.ResourceInfo{Digest: digest, Filename: "policy-evaluations.json", Size: 2048},
			wantCall: true,
		},
		{
			name:     "propagates the CAS error",
			orgID:    uuid.New(),
			casErr:   errors.New("not found"),
			wantErr:  true,
			wantCall: true,
		},
		{
			name:    "fails without reaching the CAS when the org is missing",
			orgID:   uuid.Nil,
			wantErr: true,
		},
	}

	credsProvider, err := biz.NewCASCredentialsUseCase(&conf.Auth{
		CasRobotAccountPrivateKeyPath: "./testdata/test-key.ec.pem",
	})
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := mocks.NewDownloaderUploader(t)
			if tc.wantCall {
				c.On("Describe", mock.Anything, digest).Return(tc.casInfo, tc.casErr)
			}

			clientProvider := func(_ *conf.Bootstrap_CASServer, _ string) (casclient.DownloaderUploader, func(), error) {
				return c, func() {}, nil
			}

			uc := biz.NewCASClientUseCase(credsProvider, validConf, nil, biz.WithClientFactory(clientProvider))

			got, err := uc.Describe(context.Background(), "OCI_REPOSITORY", "secret-name", tc.orgID, digest)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
