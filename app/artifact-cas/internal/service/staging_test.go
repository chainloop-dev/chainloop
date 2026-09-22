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

package service

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests lock down the shared spill-and-verify primitive used by uploads
// and both download paths: whatever the producer writes is spilled to an
// unlinked file under the staging directory, hashed in the same pass and
// compared with the expected digest. Only a verified file is ever handed back.

func TestStageAndVerify(t *testing.T) {
	content := []byte("bytes that are spilled to disk, hashed and verified")
	digest := sha256Hex(content)
	tampered := []byte("tampered")
	errProducer := errors.New("producer failed")

	tests := []struct {
		name string
		// stagingDir overrides the per-test temp dir when set
		stagingDir func(t *testing.T) string
		want       string
		fill       func(w io.Writer) error
		// wantErr is matched with errors.Is, wantErrMsg with ErrorContains
		wantErr    error
		wantErrMsg string
	}{
		{
			name: "matching content is returned rewound with its size",
			want: digest,
			fill: func(w io.Writer) error {
				// uneven chunks: the hash must cover the whole stream
				_, _ = w.Write(content[:7])
				_, err := w.Write(content[7:])
				return err
			},
		},
		{
			name: "empty content verifies against sha256 of the empty input",
			want: sha256Hex([]byte{}),
			fill: func(_ io.Writer) error { return nil },
		},
		{
			name: "content that does not hash to the digest is rejected with both digests",
			want: digest,
			fill: func(w io.Writer) error {
				_, err := w.Write(tampered)
				return err
			},
			wantErrMsg: stagingUpload.mismatchMsg + ": got=" + sha256Hex(tampered) + ", want=" + digest,
		},
		{
			name:    "producer error is returned unwrapped for the caller to classify",
			want:    digest,
			fill:    func(_ io.Writer) error { return errProducer },
			wantErr: errProducer,
		},
		{
			name:       "an unconfigured staging directory is refused",
			stagingDir: func(*testing.T) string { return "" },
			want:       digest,
			fill:       func(_ io.Writer) error { return nil },
			wantErrMsg: "no staging directory configured",
		},
		{
			name:       "a missing staging directory fails to create the file",
			stagingDir: func(t *testing.T) string { return filepath.Join(t.TempDir(), "does-not-exist") },
			want:       digest,
			fill:       func(_ io.Writer) error { return nil },
			wantErrMsg: "creating staging file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			svc := newCommonService(nil, WithStagingDir(dir))
			if tc.stagingDir != nil {
				svc.stagingDir = tc.stagingDir(t)
			}

			f, size, err := svc.stageAndVerify(stagingUpload, tc.want, tc.fill)

			// whatever happened, nothing may be left behind in the staging dir
			entries, readErr := os.ReadDir(dir)
			require.NoError(t, readErr)
			assert.Empty(t, entries, "staging dir must be left clean")

			if tc.wantErr != nil || tc.wantErrMsg != "" {
				assert.Nil(t, f)
				if tc.wantErr != nil {
					require.ErrorIs(t, err, tc.wantErr)
				}
				if tc.wantErrMsg != "" {
					require.ErrorContains(t, err, tc.wantErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, f)
			defer f.Close()

			got, err := io.ReadAll(f)
			require.NoError(t, err)
			assert.Equal(t, int64(len(got)), size)
			assert.Equal(t, tc.want, sha256Hex(got), "file must be rewound and hold the verified bytes")
		})
	}
}

// TestStageAndVerify_FileIsUnlinked: the staged file is removed from the
// directory while it is still open, so a crash mid-stream cannot strand it.
func TestStageAndVerify_FileIsUnlinked(t *testing.T) {
	dir := t.TempDir()
	svc := newCommonService(nil, WithStagingDir(dir))

	var seenDuringFill []os.DirEntry
	f, _, err := svc.stageAndVerify(stagingUpload, sha256Hex([]byte("x")), func(w io.Writer) error {
		var readErr error
		seenDuringFill, readErr = os.ReadDir(dir)
		require.NoError(t, readErr)
		_, err := w.Write([]byte("x"))
		return err
	})
	require.NoError(t, err)
	defer f.Close()

	assert.Empty(t, seenDuringFill, "the staging file must be unlinked before the producer runs")
}
