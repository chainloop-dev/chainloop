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

package chainloop

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	api "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestRenderV01(t *testing.T) {
	testCases := []struct {
		name       string
		sourcePath string
		outputPath string
	}{
		{
			name:       "render v0.1",
			sourcePath: "testdata/attestation.source.json",
			outputPath: "testdata/attestation.output.v0.1.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load expected resulting output
			wantRaw, err := os.ReadFile(tc.outputPath)
			require.NoError(t, err)

			var want *in_toto.Statement
			err = json.Unmarshal(wantRaw, &want)
			require.NoError(t, err)

			// Initialize renderer
			state := &api.CraftingState{}
			stateRaw, err := os.ReadFile(tc.sourcePath)
			require.NoError(t, err)

			err = protojson.Unmarshal(stateRaw, state)
			require.NoError(t, err)

			renderer := NewChainloopRendererV01(state.Attestation, "dev", "sha256:59e14f1a9de709cdd0e91c36b33e54fcca95f7dba1dc7169a7f81986e02108e5")

			// Compare header
			gotHeader, err := renderer.Header()
			assert.NoError(t, err)
			assert.Equal(t, want.Type, gotHeader.Type)
			assert.Equal(t, want.Subject, gotHeader.Subject)
			assert.Equal(t, want.PredicateType, gotHeader.PredicateType)

			// Compare predicate
			gotPredicateI, err := renderer.Predicate()
			assert.NoError(t, err)
			gotPredicate := gotPredicateI.(ProvenancePredicateV01)

			wantPredicate := ProvenancePredicateV01{}
			err = extractPredicate(want, &wantPredicate)
			assert.NoError(t, err)
			wantPredicate.Metadata.FinishedAt = gotPredicate.Metadata.FinishedAt
			assert.EqualValues(t, wantPredicate, gotPredicate)
		})
	}
}

func TestExtractPredicate(t *testing.T) {
	testCases := []struct {
		name          string
		envelopePath  string
		predicatePath string
		wantErr       bool
	}{
		{
			name:          "valid envelope",
			envelopePath:  "testdata/valid.envelope.json",
			predicatePath: "testdata/valid.predicate.json",
			wantErr:       false,
		},
		{
			name:         "unknown source attestation",
			envelopePath: "testdata/unknown.envelope.json",
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			envelope, err := testEnvelope(tc.envelopePath)
			require.NoError(t, err)

			versions, err := ExtractPredicate(envelope)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			want, err := testPredicate(tc.predicatePath)
			require.NoError(t, err)

			assert.NoError(t, err)
			assert.Equal(t, want, versions.V01)
		})
	}
}

func testEnvelope(filePath string) (*dsse.Envelope, error) {
	var envelope dsse.Envelope
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(content, &envelope)
	if err != nil {
		return nil, err
	}

	return &envelope, nil
}

func testPredicate(path string) (*ProvenancePredicateV01, error) {
	var predicate ProvenancePredicateV01
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(content, &predicate); err != nil {
		return nil, fmt.Errorf("un-marshaling predicate: %w", err)
	}

	return &predicate, nil
}
