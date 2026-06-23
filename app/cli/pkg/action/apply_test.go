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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitYAMLDocuments(t *testing.T) {
	t.Run("single document is preserved verbatim", func(t *testing.T) {
		// Hand-written contract with 2-space indentation and comments. The batch
		// apply path must send these bytes unchanged so the server sees exactly
		// what `wf contract apply` (verbatim os.ReadFile) sends.
		in := []byte(`apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: my-contract
spec:
  materials:
    - name: sbom   # keep this comment
      type: SBOM_CYCLONEDX_JSON
`)
		docs, err := SplitYAMLDocuments(in)
		require.NoError(t, err)
		require.Len(t, docs, 1)
		assert.Equal(t, "Contract", docs[0].Kind)
		assert.Equal(t, "my-contract", docs[0].Name)
		// The document bytes must be byte-identical to the input: no re-indenting,
		// no comment loss, no reflow.
		assert.Equal(t, string(in), string(docs[0].RawData))
	})

	t.Run("multiple documents keep their own verbatim bytes", func(t *testing.T) {
		doc1 := `apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: first
spec:
  materials:
    - name: sbom
      type: SBOM_CYCLONEDX_JSON
`
		doc2 := `apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: second   # second contract
spec: {}
`
		in := []byte(doc1 + "---\n" + doc2)

		docs, err := SplitYAMLDocuments(in)
		require.NoError(t, err)
		require.Len(t, docs, 2)
		assert.Equal(t, "first", docs[0].Name)
		assert.Equal(t, "second", docs[1].Name)
		assert.Equal(t, doc1, string(docs[0].RawData))
		assert.Equal(t, doc2, string(docs[1].RawData))
	})

	t.Run("empty and comment-only documents are skipped", func(t *testing.T) {
		in := []byte(`# just a comment, no document here
---
apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: only-one
spec: {}
`)
		docs, err := SplitYAMLDocuments(in)
		require.NoError(t, err)
		require.Len(t, docs, 1)
		assert.Equal(t, "only-one", docs[0].Name)
	})

	t.Run("document without kind returns an error", func(t *testing.T) {
		in := []byte(`apiVersion: chainloop.dev/v1
metadata:
  name: no-kind
`)
		_, err := SplitYAMLDocuments(in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "kind")
	})

	t.Run("document without metadata.name returns an error", func(t *testing.T) {
		in := []byte(`apiVersion: chainloop.dev/v1
kind: Contract
spec: {}
`)
		_, err := SplitYAMLDocuments(in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})
}
