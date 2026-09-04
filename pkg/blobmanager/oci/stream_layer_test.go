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

package oci

import (
	"bytes"
	"io"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRawStreamLayer_Metadata(t *testing.T) {
	content := "hello world"
	hash, _, err := v1.SHA256(strings.NewReader(content))
	require.NoError(t, err)

	l := &rawStreamLayer{
		hash:      hash,
		size:      int64(len(content)),
		mediaType: types.MediaType("text/plain"),
		body:      io.NopCloser(strings.NewReader(content)),
	}

	gotDigest, err := l.Digest()
	require.NoError(t, err)
	assert.Equal(t, hash, gotDigest)

	// Content is stored uncompressed, so DiffID must equal Digest; this is what
	// keeps the config rootfs.diff_ids consistent for the download path.
	gotDiffID, err := l.DiffID()
	require.NoError(t, err)
	assert.Equal(t, hash, gotDiffID)

	gotSize, err := l.Size()
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), gotSize)

	gotMT, err := l.MediaType()
	require.NoError(t, err)
	assert.Equal(t, types.MediaType("text/plain"), gotMT)
}

func TestRawStreamLayer_BodyStreamedRaw(t *testing.T) {
	content := "the quick brown fox"
	l := &rawStreamLayer{body: io.NopCloser(strings.NewReader(content))}

	// Compressed returns the raw bytes verbatim (no gzip).
	rc, err := l.Compressed()
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, string(got))
}

func TestVerifyingReader(t *testing.T) {
	content := []byte("the quick brown fox")
	digest := func(b []byte) string {
		h, _, err := v1.SHA256(bytes.NewReader(b))
		require.NoError(t, err)
		return h.Hex
	}

	t.Run("matching digest streams through cleanly", func(t *testing.T) {
		vr := newVerifyingReader(bytes.NewReader(content), digest(content), int64(len(content)))
		got, err := io.ReadAll(vr)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("mismatching digest errors", func(t *testing.T) {
		// Declare the digest of different content than what actually streams.
		vr := newVerifyingReader(bytes.NewReader(content), digest([]byte("something else")), int64(len(content)))
		_, err := io.ReadAll(vr)
		require.Error(t, err)
		assert.ErrorContains(t, err, "content digest mismatch")
	})

	t.Run("verifies at the declared byte count without a trailing EOF", func(t *testing.T) {
		// Mimic an HTTP transport bounded by Content-Length: copy exactly
		// len(content) bytes through a LimitReader, which stops there and never
		// pulls the underlying EOF. The mismatch must still be caught.
		vr := newVerifyingReader(bytes.NewReader(content), digest([]byte("other content")), int64(len(content)))
		_, err := io.Copy(io.Discard, io.LimitReader(vr, int64(len(content))))
		require.Error(t, err)
		assert.ErrorContains(t, err, "content digest mismatch")
	})
}

func TestRawStreamLayer_ConsumableOnce(t *testing.T) {
	l := &rawStreamLayer{body: io.NopCloser(strings.NewReader("data"))}

	_, err := l.Compressed()
	require.NoError(t, err)

	// A second read (retry / HTTP body replay) must fail cleanly rather than
	// return a drained reader that would upload truncated content.
	_, err = l.Compressed()
	require.Error(t, err)
	assert.ErrorContains(t, err, "already consumed")

	_, err = l.Uncompressed()
	require.Error(t, err)
}
