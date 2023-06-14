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

package extensions

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/sdk/v1/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
)

func TestDoLoad(t *testing.T) {
	extensionA := mocks.NewFanOut(t)
	extensionAFactory := func(l log.Logger) (sdk.FanOut, error) {
		extensionA.On("Describe").Return(&sdk.IntegrationInfo{ID: "a"})
		extensionA.On("String").Return("a").Maybe()
		return extensionA, nil
	}

	extensionB := mocks.NewFanOut(t)
	extensionBFactory := func(l log.Logger) (sdk.FanOut, error) {
		extensionB.On("Describe").Return(&sdk.IntegrationInfo{ID: "b"})
		extensionB.On("String").Return("b").Maybe()
		return extensionB, nil
	}

	extensionAA := mocks.NewFanOut(t)
	extensionAAFactory := func(l log.Logger) (sdk.FanOut, error) {
		extensionAA.On("Describe").Return(&sdk.IntegrationInfo{ID: "a"})
		extensionAA.On("String").Return("c").Maybe()
		return extensionAA, nil
	}

	testCases := []struct {
		name       string
		extensions []sdk.FanOutFactory
		wantErr    bool
		want       sdk.AvailableExtensions
	}{
		{
			name:       "no duplicates",
			extensions: []sdk.FanOutFactory{extensionAFactory, extensionBFactory},
			wantErr:    false,
			want:       []sdk.FanOut{extensionA, extensionB},
		},
		{
			name:       "duplicates",
			extensions: []sdk.FanOutFactory{extensionAFactory, extensionAAFactory},
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := doLoad(tc.extensions, nil)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.want, got)
			} else {
				assert.NoError(t, err)

			}
		})
	}
}
