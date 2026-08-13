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
	"context"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// digestOf returns the sha256 hex digest of content, as stored by chainloop
// (no algorithm prefix).
func digestOf(content []byte) string {
	h, _, err := v1.SHA256(bytes.NewReader(content))
	if err != nil {
		panic(err)
	}
	return h.Hex
}

// TestStreamingRoundTrip uploads content through the streaming path (size known)
// and downloads it back, asserting the bytes and metadata survive intact and the
// existing (untouched) download path keeps working.
func (s *testSuite) TestStreamingRoundTrip() {
	testCases := []struct {
		name    string
		content []byte
	}{
		{name: "small text below sniff length", content: []byte("hello world")},
		{name: "empty-ish single byte", content: []byte("x")},
		// Larger than sniffLen (512) to exercise the peek + streamed remainder.
		{name: "binary larger than sniff length", content: bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03}, 4096)},
		{name: "text larger than sniff length", content: bytes.Repeat([]byte("chainloop "), 1000)},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			digest := digestOf(tc.content)
			resource := &pb.CASResource{
				Digest:   digest,
				FileName: "artifact.bin",
				Size:     int64(len(tc.content)),
			}

			// Streamed upload (size > 0).
			err := s.validBackend.Upload(context.Background(), bytes.NewReader(tc.content), resource)
			require.NoError(t, err)

			// Download back through the existing path and compare.
			var buf bytes.Buffer
			err = s.validBackend.Download(context.Background(), &buf, digest)
			require.NoError(t, err)
			assert.Equal(t, tc.content, buf.Bytes())

			// Describe must report the right size and filename.
			desc, err := s.validBackend.Describe(context.Background(), digest)
			require.NoError(t, err)
			assert.Equal(t, digest, desc.Digest)
			assert.Equal(t, "artifact.bin", desc.FileName)
			assert.Equal(t, int64(len(tc.content)), desc.Size)

			exists, err := s.validBackend.Exists(context.Background(), digest)
			require.NoError(t, err)
			assert.True(t, exists)
		})
	}
}

// TestStreamingDigestMismatchRejected ensures the streaming path keeps the
// content-integrity guarantee the buffered path had: if the declared digest
// does not match the streamed bytes, the upload fails and nothing is committed.
func (s *testSuite) TestStreamingDigestMismatchRejected() {
	content := bytes.Repeat([]byte("streamed content "), 100)
	wrongDigest := digestOf([]byte("totally different content"))
	resource := &pb.CASResource{
		Digest:   wrongDigest,
		FileName: "mismatch.bin",
		Size:     int64(len(content)),
	}

	err := s.validBackend.Upload(context.Background(), bytes.NewReader(content), resource)
	require.Error(s.T(), err)
	assert.ErrorContains(s.T(), err, "content digest mismatch")

	// Nothing should have been committed under the (wrong) digest.
	exists, err := s.validBackend.Exists(context.Background(), wrongDigest)
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

// TestLegacyBufferedRoundTrip covers old clients that do not report the size:
// the upload falls back to buffering and still round-trips.
func (s *testSuite) TestLegacyBufferedRoundTrip() {
	content := bytes.Repeat([]byte("legacy "), 500)
	digest := digestOf(content)
	resource := &pb.CASResource{
		Digest:   digest,
		FileName: "legacy.bin",
		// Size intentionally left as 0 to emulate a legacy client.
	}

	err := s.validBackend.Upload(context.Background(), bytes.NewReader(content), resource)
	require.NoError(s.T(), err)

	var buf bytes.Buffer
	err = s.validBackend.Download(context.Background(), &buf, digest)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), content, buf.Bytes())
}
