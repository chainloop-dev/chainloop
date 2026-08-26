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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestUploadAndCraftContentOverride covers the seam redaction relies on: the
// stored bytes are the substituted ones, and every derived field (digest, size,
// uploaded body) describes them rather than the file on disk.
func TestUploadAndCraftContentOverride(t *testing.T) {
	const (
		onDisk   = `{"original":"content that is definitely longer"}`
		redacted = `{"original":"[REDACTED]"}`
	)

	testCases := []struct {
		name       string
		override   []byte
		skipUpload bool
		maxSize    int64
		wantErr    string
		wantStored string
		wantUpload bool
	}{
		{
			name:       "no override stores the file on disk",
			wantStored: onDisk,
			wantUpload: true,
		},
		{
			name:       "override stores the substituted bytes",
			override:   []byte(redacted),
			wantStored: redacted,
			wantUpload: true,
		},
		{
			name:     "an empty override is rejected rather than stored",
			override: []byte{},
			wantErr:  "file is empty",
		},
		{
			// The CAS stores the substituted content, so its size is the one that
			// has to fit the backend.
			name:     "the size limit applies to the override",
			override: []byte(`{"original":"padded out to be much larger than the limit"}`),
			maxSize:  20,
			wantErr:  "too big",
		},
		{
			name:       "skip upload still records the override digest",
			override:   []byte(redacted),
			skipUpload: true,
			wantStored: redacted,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := zerolog.Nop()
			path := filepath.Join(t.TempDir(), "session.json")
			require.NoError(t, os.WriteFile(path, []byte(onDisk), 0o600))

			schema := &schemaapi.CraftingSchema_Material{
				Name:       "test",
				Type:       schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
				SkipUpload: tc.skipUpload,
			}

			backend := &casclient.CASBackend{Name: "test", MaxSize: tc.maxSize}
			var uploaded []byte
			if tc.wantUpload {
				uploader := mUploader.NewUploader(t)
				uploader.On("Upload", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						var err error
						uploaded, err = io.ReadAll(args.Get(1).(io.Reader))
						require.NoError(t, err)
					}).
					Return(&casclient.UpDownStatus{Digest: "deadbeef"}, nil)
				backend.Uploader = uploader
			}

			var opts []uploadAndCraftOption
			if tc.override != nil {
				opts = append(opts, withContentOverride(tc.override))
			}

			got, err := uploadAndCraft(context.TODO(), schema, backend, path, &logger, opts...)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			// The recorded filename must be unaffected by the substitution.
			assert.Equal(t, "session.json", got.GetArtifact().Name)
			assert.Equal(t, strconv.Itoa(len(tc.wantStored)), got.Annotations[AnnotationMaterialSize])
			assert.Equal(t, sha256Digest(tc.wantStored), got.GetArtifact().Digest)

			if tc.wantUpload {
				assert.Equal(t, tc.wantStored, string(uploaded))
				assert.True(t, got.UploadedToCas)
			} else {
				assert.False(t, got.UploadedToCas)
			}
		})
	}
}

// The AWS credentials the session fixture carries as __AWS_ACCESS_KEY_ID__ and
// __AWS_SECRET_ACCESS_KEY__ placeholders, assembled from fragments so the literal
// appears in no source file: GitHub's push protection recognises the same AWS
// patterns betterleaks does, and rejects a realistic-looking key even in test
// data.
const (
	awsKey    = "AKIA" + "4G7TI63VCBIRS4GW"
	awsSecret = "kQ7zXn2VbW9pLm4RtY6" + "uHs3JdF8gA1cE5oPzQwXn"
)

// materializeFixture writes a copy of a session fixture with the AWS credential
// placeholders substituted, and returns its path. The crafter reads the artifact
// from disk, so the substitution has to land in a real file.
func materializeFixture(t *testing.T, src string) string {
	t.Helper()

	content, err := os.ReadFile(src)
	require.NoError(t, err)

	content = bytes.ReplaceAll(content, []byte("__AWS_ACCESS_KEY_ID__"), []byte(awsKey))
	content = bytes.ReplaceAll(content, []byte("__AWS_SECRET_ACCESS_KEY__"), []byte(awsSecret))

	path := filepath.Join(t.TempDir(), filepath.Base(src))
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

// TestChainloopAICodingSessionCrafterRedaction is the end-to-end guarantee: the
// bytes that leave the machine carry no secrets, while the file on disk — what
// policies are evaluated against afterwards — is left untouched.
func TestChainloopAICodingSessionCrafterRedaction(t *testing.T) {
	const (
		withSecrets = "./aicodingsession/testdata/session-with-secrets.json"
		clean       = "./testdata/ai-coding-session.json"
	)

	testCases := []struct {
		name          string
		filePath      string
		skipRedaction bool
		inlineBackend bool
		wantRedacted  bool
		wantCount     string
		wantRules     string
	}{
		{
			name:         "secrets are stripped before upload",
			filePath:     withSecrets,
			wantRedacted: true,
			wantCount:    "7",
			wantRules:    "anthropic-api-key,aws-access-token,aws-secret-access-key,github-pat",
		},
		{
			// An inline backend embeds the content into the attestation itself,
			// so it matters most there that the redacted copy is the stored one.
			name:          "an inline backend embeds the redacted copy",
			filePath:      withSecrets,
			inlineBackend: true,
			wantRedacted:  true,
			wantCount:     "7",
			wantRules:     "anthropic-api-key,aws-access-token,aws-secret-access-key,github-pat",
		},
		{
			name:          "the opt-out is recorded in the attestation",
			filePath:      withSecrets,
			skipRedaction: true,
		},
		{
			name:     "a clean session is not marked as redacted",
			filePath: clean,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := zerolog.Nop()
			schema := &schemaapi.CraftingSchema_Material{
				Name: "test",
				Type: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
			}

			path := materializeFixture(t, tc.filePath)
			original, err := os.ReadFile(path)
			require.NoError(t, err)

			// A nil Uploader is what the CLI builds for an inline CAS backend.
			backend := &casclient.CASBackend{Name: "not-set"}
			var stored []byte
			if !tc.inlineBackend {
				uploader := mUploader.NewUploader(t)
				uploader.On("Upload", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						var readErr error
						stored, readErr = io.ReadAll(args.Get(1).(io.Reader))
						require.NoError(t, readErr)
					}).
					Return(&casclient.UpDownStatus{Digest: "deadbeef"}, nil)
				backend.Uploader = uploader
			}

			crafter, err := NewChainloopAICodingSessionCrafter(schema, backend, &logger,
				WithAICodingSessionSkipRedaction(tc.skipRedaction))
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), path)
			require.NoError(t, err)

			if tc.inlineBackend {
				require.True(t, got.InlineCas)
				stored = got.GetArtifact().Content
			}

			assert.Equal(t, tc.wantCount, got.Annotations[api.AnnotationMaterialRedactionCount])
			assert.Equal(t, tc.wantRules, got.Annotations[api.AnnotationMaterialRedactionRules])

			switch {
			case tc.wantRedacted:
				assert.Equal(t, "true", got.Annotations[api.AnnotationMaterialRedacted])
				assert.NotContains(t, string(stored), awsKey)
				assert.Contains(t, string(stored), "[REDACTED:aws-access-token]")
				// The digest describes the redacted artifact, not the source file.
				assert.NotEqual(t, sha256Digest(string(original)), got.GetArtifact().Digest)
			case tc.skipRedaction:
				assert.Equal(t, "true", got.Annotations[api.AnnotationMaterialRedactionSkipped])
				assert.Contains(t, string(stored), awsKey)
				assert.Equal(t, sha256Digest(string(original)), got.GetArtifact().Digest)
			default:
				assert.NotContains(t, got.Annotations, api.AnnotationMaterialRedacted)
				assert.NotContains(t, got.Annotations, api.AnnotationMaterialRedactionSkipped)
				// Nothing to redact, so the digest stays reproducible from the file.
				assert.Equal(t, sha256Digest(string(original)), got.GetArtifact().Digest)
			}

			// Redaction must never touch the file the policies will read.
			stillOnDisk, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, string(original), string(stillOnDisk))
		})
	}
}

func sha256Digest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}
