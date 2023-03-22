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

package renderer

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

			got, err := ExtractPredicate(envelope)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			want, err := testPredicate(tc.predicatePath)
			require.NoError(t, err)

			assert.NoError(t, err)
			assert.Equal(t, want, got)
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

func testPredicate(path string) (*ChainloopProvenancePredicateV1, error) {
	var predicate ChainloopProvenancePredicateV1
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(content, &predicate); err != nil {
		return nil, fmt.Errorf("un-marshaling predicate: %w", err)
	}

	return &predicate, nil
}
