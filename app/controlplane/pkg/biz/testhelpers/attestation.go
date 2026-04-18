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

package testhelpers

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// BundleBytesFromEnvelope reads a DSSE envelope fixture from disk and returns the
// protojson-encoded bytes of an equivalent Sigstore bundle.
func BundleBytesFromEnvelope(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var env dsse.Envelope
	require.NoError(t, json.Unmarshal(raw, &env))
	b, err := attestation.BundleFromDSSEEnvelope(&env)
	require.NoError(t, err)
	out, err := protojson.Marshal(b)
	require.NoError(t, err)
	return out
}
