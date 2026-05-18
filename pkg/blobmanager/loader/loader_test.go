//
// Copyright 2026 The Chainloop Authors.
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

package loader

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/chainloop-dev/chainloop/pkg/blobmanager/azureblob"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/s3"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/s3accesspoint"
)

type stubReader struct{}

func (stubReader) ReadCredentials(_ context.Context, _ string, _ any) error { return nil }

func TestLoadProviders_AllRegistered(t *testing.T) {
	t.Parallel()

	ps := LoadProviders(stubReader{})
	assert.Contains(t, ps, oci.ProviderID)
	assert.Contains(t, ps, azureblob.ProviderID)
	assert.Contains(t, ps, s3.ProviderID)
	assert.Contains(t, ps, s3accesspoint.ProviderID)
}
