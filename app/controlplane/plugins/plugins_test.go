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

package plugins

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
)

func TestDoLoad(t *testing.T) {
	pluginA := mocks.NewFanOut(t)
	pluginAFactory := func(l log.Logger) (sdk.FanOut, error) {
		pluginA.On("Describe").Return(&sdk.IntegrationInfo{ID: "a"})
		pluginA.On("String").Return("a").Maybe()
		return pluginA, nil
	}

	pluginB := mocks.NewFanOut(t)
	pluginBFactory := func(l log.Logger) (sdk.FanOut, error) {
		pluginB.On("Describe").Return(&sdk.IntegrationInfo{ID: "b"})
		pluginB.On("String").Return("b").Maybe()
		return pluginB, nil
	}

	pluginAA := mocks.NewFanOut(t)
	pluginAAFactory := func(l log.Logger) (sdk.FanOut, error) {
		pluginAA.On("Describe").Return(&sdk.IntegrationInfo{ID: "a"})
		pluginAA.On("String").Return("c").Maybe()
		return pluginAA, nil
	}

	testCases := []struct {
		name    string
		plugins []sdk.FanOutFactory
		wantErr bool
		want    sdk.AvailablePlugins
	}{
		{
			name:    "no duplicates",
			plugins: []sdk.FanOutFactory{pluginAFactory, pluginBFactory},
			wantErr: false,
			want:    []sdk.FanOut{pluginA, pluginB},
		},
		{
			name:    "duplicates",
			plugins: []sdk.FanOutFactory{pluginAFactory, pluginAAFactory},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := doLoad(tc.plugins, nil)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.want, got)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
