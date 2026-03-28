//
// Copyright 2024-2026 The Chainloop Authors.
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
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

var updateGolden bool

// go test ./renderer/chainloop/... --update-golden
func TestMain(m *testing.M) {
	flag.BoolVar(&updateGolden, "update-golden", false, "update the expected golden files")
	// Parse the flags
	flag.Parse()
	os.Exit(m.Run())
}

func TestRenderV02(t *testing.T) {
	testCases := []struct {
		name       string
		sourcePath string
		outputPath string
	}{
		{
			name:       "default with policy violations",
			sourcePath: "testdata/attestation.source.json",
			outputPath: "testdata/attestation.output.v0.2.json",
		},
		{
			name:       "with multiple types of materials",
			sourcePath: "testdata/attestation.source-2.json",
			outputPath: "testdata/attestation.output-2.v0.2.json",
		},
		{
			name:       "V2 schema with policy violations",
			sourcePath: "testdata/attestation.source.v2.json",
			outputPath: "testdata/attestation.output.v2.json",
		},
		{
			name:       "V2 schema with multiple types of materials",
			sourcePath: "testdata/attestation.source-2.v2.json",
			outputPath: "testdata/attestation.output-2.v2.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize renderer
			state := &api.CraftingState{}
			stateRaw, err := os.ReadFile(tc.sourcePath)
			require.NoError(t, err)
			err = protojson.Unmarshal(stateRaw, state)
			require.NoError(t, err)
			renderer := NewChainloopRendererV02(state.Attestation, "dev", "sha256:59e14f1a9de709cdd0e91c36b33e54fcca95f7dba1dc7169a7f81986e02108e5", nil, nil)

			// Compare result
			statement, err := renderer.Statement(context.TODO())
			require.NoError(t, err)
			gotRawStatement, err := json.MarshalIndent(statement, "", "  ")
			require.NoError(t, err)

			// Update test files
			if updateGolden {
				err := os.WriteFile(filepath.Clean(tc.outputPath), gotRawStatement, 0600)
				require.NoError(t, err)
			}

			// Load expected resulting output
			raw, err := os.ReadFile(tc.outputPath)
			require.NoError(t, err)
			var want *intoto.Statement
			err = json.Unmarshal(raw, &want)
			require.NoError(t, err)
			wantRaw, err := json.MarshalIndent(want, "", "  ")
			require.NoError(t, err)

			assert.Equal(t, string(wantRaw), string(gotRawStatement))
		})
	}
}

func mapToStruct(t *testing.T, input map[string]interface{}) *structpb.Struct {
	res, err := structpb.NewStruct(input)
	require.NoError(t, err)
	return res
}

func TestNormalizeMaterial(t *testing.T) {
	testCases := []struct {
		name             string
		input            *intoto.ResourceDescriptor
		inputAnnotations map[string]interface{}
		want             *NormalizedMaterial
		wantErr          bool
	}{
		{
			name: "invalid material type",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "INVALID",
				}),
			},
			wantErr: true,
		},
		{
			name: "missing material type",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.name": "foo",
				}),
			},
			wantErr: true,
		},
		{
			name: "missing material name",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.type": "STRING",
				}),
			},
			wantErr: true,
		},
		{
			name: "valid string material",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "STRING",
				}),
				Content: []byte("bar"),
			},
			want: &NormalizedMaterial{
				Name:  "foo",
				Type:  "STRING",
				Value: "bar",
				Annotations: map[string]string{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "STRING",
				},
			},
		},
		{
			name: "empty string material",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "STRING",
				}),
			},
			wantErr: true,
		},
		{
			name: "valid artifact material",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "ARTIFACT",
					"chainloop.material.cas":  true,
				}),
				Digest: map[string]string{
					"sha256": "deadbeef",
				},
				Name: "artifact.tgz",
			},
			want: &NormalizedMaterial{
				Name:          "foo",
				Type:          "ARTIFACT",
				Filename:      "artifact.tgz",
				Hash:          &crv1.Hash{Algorithm: "sha256", Hex: "deadbeef"},
				UploadedToCAS: true,
				Annotations: map[string]string{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "ARTIFACT",
					"chainloop.material.cas":  "true",
				},
			},
		},
		{
			name: "valid artifact material with annotations",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "ARTIFACT",
					"chainloop.material.cas":  true,
					"foo":                     "bar",
					"bar":                     "baz",
				}),
				Digest: map[string]string{
					"sha256": "deadbeef",
				},
				Name: "artifact.tgz",
			},
			want: &NormalizedMaterial{
				Name:          "foo",
				Type:          "ARTIFACT",
				Filename:      "artifact.tgz",
				Hash:          &crv1.Hash{Algorithm: "sha256", Hex: "deadbeef"},
				UploadedToCAS: true,
				Annotations: map[string]string{
					"foo":                     "bar",
					"bar":                     "baz",
					"chainloop.material.name": "foo",
					"chainloop.material.type": "ARTIFACT",
					"chainloop.material.cas":  "true",
				},
			},
		},
		{
			name: "valid artifact material, inline content",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.name":       "foo",
					"chainloop.material.type":       "ARTIFACT",
					"chainloop.material.cas.inline": true,
				}),
				Digest: map[string]string{
					"sha256": "deadbeef",
				},
				Name:    "artifact.tgz",
				Content: []byte("this is an inline material"),
			},
			want: &NormalizedMaterial{
				Name:           "foo",
				Type:           "ARTIFACT",
				Filename:       "artifact.tgz",
				Value:          "this is an inline material",
				Hash:           &crv1.Hash{Algorithm: "sha256", Hex: "deadbeef"},
				EmbeddedInline: true,
				Annotations: map[string]string{
					"chainloop.material.name":       "foo",
					"chainloop.material.type":       "ARTIFACT",
					"chainloop.material.cas.inline": "true",
				},
			},
		},
		{
			name: "invalid artifact material, missing file name",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "ARTIFACT",
				}),
				Digest: map[string]string{
					"sha256": "deadbeef",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid artifact material, missing digest",
			input: &intoto.ResourceDescriptor{
				Annotations: mapToStruct(t, map[string]interface{}{
					"chainloop.material.name": "foo",
					"chainloop.material.type": "ARTIFACT",
				}),
				Name: "artifact.tgz",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeMaterial(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tc.want, got)
			}
		})
	}
}

func TestStructValueToString(t *testing.T) {
	testCases := []struct {
		name  string
		input *structpb.Value
		want  string
	}{
		{name: "string value", input: structpb.NewStringValue("hello"), want: "hello"},
		{name: "empty string", input: structpb.NewStringValue(""), want: ""},
		{name: "bool true", input: structpb.NewBoolValue(true), want: "true"},
		{name: "bool false", input: structpb.NewBoolValue(false), want: "false"},
		{name: "number int", input: structpb.NewNumberValue(42), want: "42"},
		{name: "number float", input: structpb.NewNumberValue(3.14), want: "3.14"},
		{name: "null value", input: structpb.NewNullValue(), want: ""},
		{name: "list value", input: mustListValue(t, []any{"a", "b"}), want: `["a","b"]`},
		{name: "struct value", input: mustStructValue(t, map[string]any{"k": "v"}), want: `{"k":"v"}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, structValueToString(tc.input))
		})
	}
}

func mustListValue(t *testing.T, items []any) *structpb.Value {
	t.Helper()
	list, err := structpb.NewList(items)
	require.NoError(t, err)
	return structpb.NewListValue(list)
}

func mustStructValue(t *testing.T, fields map[string]any) *structpb.Value {
	t.Helper()
	s, err := structpb.NewStruct(fields)
	require.NoError(t, err)
	return structpb.NewStructValue(s)
}

func TestPredicateWithPolicyEvaluationsRef(t *testing.T) {
	stateRaw, err := os.ReadFile("testdata/attestation.source.v2.json")
	require.NoError(t, err)

	state := &api.CraftingState{}
	err = protojson.Unmarshal(stateRaw, state)
	require.NoError(t, err)

	renderer := NewChainloopRendererV02(state.Attestation, "dev", "sha256:59e14f1a9de709cdd0e91c36b33e54fcca95f7dba1dc7169a7f81986e02108e5", nil, nil)

	ref := &intoto.ResourceDescriptor{
		Name:      "policy-evaluations",
		Digest:    map[string]string{"sha256": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
		MediaType: PolicyEvaluationsBundleMediaType,
	}
	renderer.SetPolicyEvaluationsRef(ref)

	statement, err := renderer.Statement(context.TODO())
	require.NoError(t, err)

	var predicate ProvenancePredicateV02
	err = extractPredicate(statement, &predicate)
	require.NoError(t, err)

	require.NotNil(t, predicate.PolicyEvaluationsRef)
	assert.Equal(t, "policy-evaluations", predicate.PolicyEvaluationsRef.Name)
	assert.Equal(t, PolicyEvaluationsBundleMediaType, predicate.PolicyEvaluationsRef.MediaType)
	assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", predicate.PolicyEvaluationsRef.Digest["sha256"])
}

func TestPredicateWithoutPolicyEvaluationsRef(t *testing.T) {
	stateRaw, err := os.ReadFile("testdata/attestation.source.v2.json")
	require.NoError(t, err)

	state := &api.CraftingState{}
	err = protojson.Unmarshal(stateRaw, state)
	require.NoError(t, err)

	renderer := NewChainloopRendererV02(state.Attestation, "dev", "sha256:59e14f1a9de709cdd0e91c36b33e54fcca95f7dba1dc7169a7f81986e02108e5", nil, nil)

	statement, err := renderer.Statement(context.TODO())
	require.NoError(t, err)

	var predicate ProvenancePredicateV02
	err = extractPredicate(statement, &predicate)
	require.NoError(t, err)

	assert.Nil(t, predicate.PolicyEvaluationsRef)
}

func TestPolicyEvaluationsField(t *testing.T) {
	raw, err := os.ReadFile("testdata/attestation-pe-snake.json")
	require.NoError(t, err)

	var st *intoto.Statement
	err = json.Unmarshal(raw, &st)
	require.NoError(t, err)

	var predicate ProvenancePredicateV02
	err = extractPredicate(st, &predicate)
	require.NoError(t, err)

	assert.Len(t, predicate.GetPolicyEvaluations(), 1)
	evs := predicate.GetPolicyEvaluations()["sbom"]
	assert.Len(t, evs, 1)
	ev := evs[0]
	assert.Equal(t, "sbom", ev.MaterialName)
	assert.NotNil(t, ev.PolicyReference)
}
