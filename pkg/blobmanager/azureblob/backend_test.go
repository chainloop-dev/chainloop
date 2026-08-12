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

package azureblob

import (
	"testing"

	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackend_SupportsStreaming asserts the azureblob backend opts into
// streaming uploads so the CAS service feeds it directly from the client stream
// instead of buffering the whole artifact in memory (PFM-6923).
func TestBackend_SupportsStreaming(t *testing.T) {
	var b backend.UploaderDownloader = &Backend{}
	su, ok := b.(backend.StreamingUploader)
	require.True(t, ok, "azureblob backend must implement backend.StreamingUploader")
	assert.True(t, su.SupportsStreaming())
}
