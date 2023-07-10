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
	"io"
	"path/filepath"
	"testing"

	loaderMocks "github.com/chainloop-dev/chainloop/app/controlplane/plugins/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	pluginC := mocks.NewFanOut(t)
	pluginCFactory := func(l log.Logger) (sdk.FanOut, error) {
		pluginB.On("Describe").Return(&sdk.IntegrationInfo{ID: "c"})
		pluginB.On("String").Return("c").Maybe()
		return pluginB, nil
	}

	testCases := []struct {
		name              string
		pluginsFromMemory []sdk.FanOutFactory
		pluginsFromDir    []sdk.FanOutFactory
		wantErr           bool
		want              sdk.AvailablePlugins
	}{
		{
			name:              "no plugins in dir and no duplicates",
			pluginsFromMemory: []sdk.FanOutFactory{pluginAFactory, pluginBFactory},
			wantErr:           false,
			want: []*sdk.FanOutP{
				{FanOut: pluginA}, {FanOut: pluginB},
			},
		},
		{
			name:              "no plugins in dir and duplicates skipped",
			pluginsFromMemory: []sdk.FanOutFactory{pluginAFactory, pluginBFactory, pluginAFactory},
			wantErr:           false,
			want: []*sdk.FanOutP{
				{FanOut: pluginA}, {FanOut: pluginB},
			},
		},
		{
			name:              "plugins in dir and memory and no duplicates",
			pluginsFromMemory: []sdk.FanOutFactory{pluginAFactory, pluginBFactory, pluginAFactory},
			pluginsFromDir:    []sdk.FanOutFactory{pluginCFactory},
			wantErr:           false,
			want: []*sdk.FanOutP{
				{FanOut: pluginA}, {FanOut: pluginB}, {FanOut: pluginC},
			},
		},
		{
			name:              "plugins in dir and memory but duplicates",
			pluginsFromMemory: []sdk.FanOutFactory{pluginAFactory, pluginBFactory, pluginCFactory},
			pluginsFromDir:    []sdk.FanOutFactory{pluginCFactory},
			wantErr:           false,
			want: []*sdk.FanOutP{
				{FanOut: pluginA}, {FanOut: pluginB}, {FanOut: pluginC},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			memLoader := &memoryLoader{plugins: tc.pluginsFromMemory}
			dirLoader := &memoryLoader{plugins: tc.pluginsFromDir}

			got, err := doLoad(memLoader, dirLoader, log.NewHelper(log.NewStdLogger(io.Discard)))
			defer got.Cleanup()
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.want, got)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDirectoryLoader(t *testing.T) {
	pluginA := mocks.NewFanOut(t)
	pluginA.On("Describe").Return(&sdk.IntegrationInfo{ID: "a"})
	pluginA.On("String").Return("a").Maybe()

	pluginB := mocks.NewFanOut(t)
	pluginB.On("Describe").Return(&sdk.IntegrationInfo{ID: "b"})
	pluginB.On("String").Return("b").Maybe()

	initializer := loaderMocks.NewPluginInitializer(t)

	dirLoader := &directoryLoader{
		pluginsDir:  "./testdata/plugins",
		initializer: initializer,
		logger:      log.NewHelper(log.NewStdLogger(io.Discard)),
	}

	pluginDir, err := filepath.Abs(dirLoader.pluginsDir)
	require.NoError(t, err)

	initializer.On("Init", filepath.Join(pluginDir, "chainloop-plugin-a")).Return(
		&sdk.FanOutP{FanOut: pluginA, DisposeFunc: func() {}}, nil,
	)

	initializer.On("Init", filepath.Join(pluginDir, "chainloop-plugin-a-duplicated")).Return(
		&sdk.FanOutP{FanOut: pluginA, DisposeFunc: func() {}}, nil,
	)

	initializer.On("Init", filepath.Join(pluginDir, "chainloop-plugin-b")).Return(
		&sdk.FanOutP{FanOut: pluginB, DisposeFunc: func() {}}, nil,
	)

	plugins, err := dirLoader.load()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(plugins))
}
