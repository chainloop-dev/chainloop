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

package biz_test

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/chainloop-dev/chainloop/internal/casclient/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
		t.Run(tc.name, func(t *testing.T) {
			clientProvider := func(conf *conf.Bootstrap_CASServer, token string) (casclient.DownloaderUploader, func() error, error) {
				c := mocks.NewDownloaderUploader(t)
				c.On("IsReady", mock.Anything).Return(tc.casReady, nil)
				return c, func() error { return nil }, nil
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
