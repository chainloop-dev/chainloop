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

package service_test

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/stretchr/testify/assert"
)

var enabledProviders *backend.Providers = &backend.Providers{
	"OCI": nil,
	"GCS": nil,
}

func TestInfoz(t *testing.T) {
	want := "v1.0.0"
	got, err := service.NewStatusService(want, enabledProviders).Infoz(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, want, got.Version)
	assert.Equal(t, []string{"OCI", "GCS"}, got.Backends)
}

func TestStatusz(t *testing.T) {
	_, err := service.NewStatusService("1.1.1", enabledProviders).Statusz(context.Background(), nil)
	assert.NoError(t, err)
}
