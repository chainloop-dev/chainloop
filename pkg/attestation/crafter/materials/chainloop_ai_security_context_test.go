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

package materials

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewChainloopAISecurityContextCrafter_WrongType(t *testing.T) {
	logger := zerolog.Nop()

	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
	}

	_, err := NewChainloopAISecurityContextCrafter(schema, nil, &logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "material type is not chainloop_ai_security_context")
}

func TestNewChainloopAISecurityContextCrafter_CorrectType(t *testing.T) {
	logger := zerolog.Nop()

	schema := &schemaapi.CraftingSchema_Material{
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_SECURITY_CONTEXT,
	}

	crafter, err := NewChainloopAISecurityContextCrafter(schema, nil, &logger)
	require.NoError(t, err)
	assert.NotNil(t, crafter)
}

// newSecurityContextCrafter builds a crafter whose uploader always succeeds, so
// that a test failure is always about the security context and never about CAS.
func newSecurityContextCrafter(t *testing.T) *ChainloopAISecurityContextCrafter {
	t.Helper()

	logger := zerolog.Nop()
	schema := &schemaapi.CraftingSchema_Material{
		Name: "test",
		Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_SECURITY_CONTEXT,
	}

	uploader := mUploader.NewUploader(t)
	uploader.On("Upload", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&casclient.UpDownStatus{Digest: "deadbeef", Filename: "security-context.json"}, nil).
		Maybe()

	crafter, err := NewChainloopAISecurityContextCrafter(schema, &casclient.CASBackend{Uploader: uploader}, &logger)
	require.NoError(t, err)
	return crafter
}

func TestChainloopAISecurityContextCrafter_Craft(t *testing.T) {
	testCases := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name: "a full security context",
			path: "./testdata/ai-security-context.json",
		},
		{
			name: "a security context with a single fingerprint",
			path: "./testdata/ai-security-context-minimal.json",
		},
		{
			// The payload on its own. Rejected because the envelope is what makes
			// the blob self-identifying once it is in CAS.
			name:    "the payload without the evidence envelope",
			path:    "./testdata/ai-security-context-bare.json",
			wantErr: "must be wrapped in the Chainloop evidence envelope",
		},
		{
			// additionalProperties: false at every level is what makes an explicit
			// --kind fail loudly on the wrong file, since this kind is excluded
			// from auto-detection.
			name:    "an unknown field inside a fingerprint",
			path:    "./testdata/ai-security-context-extra-field.json",
			wantErr: "additionalProperties 'unexpected_field' not allowed",
		},
		{
			name:    "an unrelated JSON document",
			path:    "../../../../internal/schemavalidators/testdata/sbom-spdx.json",
			wantErr: "must be wrapped in the Chainloop evidence envelope",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newSecurityContextCrafter(t).Craft(context.TODO(), tc.path)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, schemaapi.CraftingSchema_Material_CHAINLOOP_AI_SECURITY_CONTEXT, got.MaterialType)
		})
	}
}

func TestChainloopAISecurityContextCrafter_Envelope(t *testing.T) {
	const (
		validID  = "CHAINLOOP_AI_SECURITY_CONTEXT"
		validURL = "https://schemas.chainloop.dev/aisecuritycontext/0.1/ai-security-context.schema.json"
	)

	testCases := []struct {
		name    string
		id      string
		schema  string
		wantErr string
	}{
		{
			name:   "the envelope the producer writes",
			id:     validID,
			schema: validURL,
		},
		{
			// The kind was renamed during design; an artifact from a producer that
			// predates the rename must not pass as this material.
			name:    "an evidence id from another material",
			id:      "CHAINLOOP_SECURITY_POSTURE",
			schema:  validURL,
			wantErr: `evidence id is "CHAINLOOP_SECURITY_POSTURE"`,
		},
		{
			name:    "a schema URL from another material",
			id:      validID,
			schema:  "https://schemas.chainloop.dev/aicodingsession/0.1/ai-coding-session.schema.json",
			wantErr: "schema is",
		},
		{
			name:    "an empty envelope",
			wantErr: "must be wrapped in the Chainloop evidence envelope",
		},
	}

	payload, err := os.ReadFile("./testdata/ai-security-context-bare.json")
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "security-context.json")
			require.NoError(t, os.WriteFile(path, envelopeWith(t, tc.id, tc.schema, payload), 0o600))

			_, err := newSecurityContextCrafter(t).Craft(context.TODO(), path)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// envelopeWith wraps a payload in an envelope carrying the given id and schema.
// An empty id and schema produce an envelope with neither, which is the shape a
// producer that forgot to wrap its output would write.
func envelopeWith(t *testing.T, id, schema string, payload []byte) []byte {
	t.Helper()

	if id == "" && schema == "" {
		return []byte(`{}`)
	}

	doc := map[string]any{
		"chainloop.material.evidence.id": id,
		"schema":                         schema,
		"data":                           json.RawMessage(payload),
	}
	b, err := json.Marshal(doc)
	require.NoError(t, err)
	return b
}

func TestChainloopAISecurityContextCrafter_Annotations(t *testing.T) {
	testCases := []struct {
		name         string
		path         string
		headSHA      string
		toolVersion  string
		fingerprints string
		reconciles   string
	}{
		{
			name:         "a full security context",
			path:         "./testdata/ai-security-context.json",
			headSHA:      "49d3dd9a8b53f4988cf43d1d824fb3b14f808fcb",
			toolVersion:  "dev",
			fingerprints: "12",
			reconciles:   "true",
		},
		{
			name:         "a security context with a single fingerprint",
			path:         "./testdata/ai-security-context-minimal.json",
			headSHA:      "8c948c742bdfc09c4aae6b3c386faeb98f925ff2",
			toolVersion:  "dev",
			fingerprints: "1",
			reconciles:   "true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newSecurityContextCrafter(t).Craft(context.TODO(), tc.path)
			require.NoError(t, err)

			assert.Equal(t, tc.headSHA, got.Annotations[annotationSecurityContextHeadSHA])
			assert.Equal(t, tc.toolVersion, got.Annotations[annotationSecurityContextToolVersion])
			assert.Equal(t, tc.fingerprints, got.Annotations[annotationSecurityContextFingerprints])
			assert.Equal(t, tc.reconciles, got.Annotations[annotationSecurityContextReconciles])
		})
	}
}

// TestChainloopAISecurityContextCrafter_ReconcilesIsAlwaysAnnotated pins the one
// annotation that must be present even when false: a funnel that does not
// reconcile means the scan is incomplete, and a policy can only reject that if
// the annotation is there to read.
func TestChainloopAISecurityContextCrafter_ReconcilesIsAlwaysAnnotated(t *testing.T) {
	raw, err := os.ReadFile("./testdata/ai-security-context-minimal.json")
	require.NoError(t, err)

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &doc))

	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["data"], &data))
	var scan map[string]any
	require.NoError(t, json.Unmarshal(data["scan"], &scan))
	scan["reconciles"] = false

	data["scan"] = mustMarshal(t, scan)
	doc["data"] = mustMarshal(t, data)

	path := filepath.Join(t.TempDir(), "security-context.json")
	require.NoError(t, os.WriteFile(path, mustMarshal(t, doc), 0o600))

	got, err := newSecurityContextCrafter(t).Craft(context.TODO(), path)
	require.NoError(t, err)
	assert.Equal(t, "false", got.Annotations[annotationSecurityContextReconciles])
}

func TestChainloopAISecurityContextCrafter_FileErrors(t *testing.T) {
	testCases := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "not JSON at all",
			content: "not json",
			wantErr: "invalid JSON format",
		},
		{
			name:    "a JSON array rather than an envelope",
			content: `[]`,
			wantErr: "invalid JSON format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "security-context.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.content), 0o600))

			_, err := newSecurityContextCrafter(t).Craft(context.TODO(), path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestChainloopAISecurityContextCrafter_FileNotFound(t *testing.T) {
	_, err := newSecurityContextCrafter(t).Craft(context.TODO(), "./testdata/does-not-exist.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't open the file")
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
