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
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc/codes"
)

// These tests lock down the DOWNLOAD digest-verification behavior. The download
// path is intentionally unchanged by the streaming-upload work (PFM-6923); this
// battery guards it against regressions — the server must stream the stored
// bytes back, compute their sha256 across however many chunks the backend
// produces, and reject any content whose digest does not match the requested
// resource name.

// fakeReadServer is a minimal bytestream.ByteStream_ReadServer that records the
// data chunks the streamWriter sends. Only Send is exercised by streamWriter.
type fakeReadServer struct {
	bytestream.ByteStream_ReadServer
	sent [][]byte
}

func (f *fakeReadServer) Send(r *bytestream.ReadResponse) error {
	// copy: the underlying buffer may be reused by the caller
	cp := make([]byte, len(r.Data))
	copy(cp, r.Data)
	f.sent = append(f.sent, cp)
	return nil
}

// TestStreamWriter_ChecksumAcrossWrites verifies the download writer computes
// the sha256 over the full stream regardless of how the content is chunked, and
// forwards every byte to the client in order.
func TestStreamWriter_ChecksumAcrossWrites(t *testing.T) {
	content := []byte("chainloop download payload delivered in several writes")
	digest := sha256Hex(content)

	fake := &fakeReadServer{}
	sw := &streamWriter{
		stream:       fake,
		log:          servicelogger.EmptyLogger(),
		wantChecksum: digest,
		gotChecksum:  sha256.New(),
	}

	// Feed the content in uneven pieces to mimic a backend emitting arbitrary
	// chunk boundaries.
	for off := 0; off < len(content); off += 11 {
		end := off + 11
		if end > len(content) {
			end = len(content)
		}
		n, err := sw.Write(content[off:end])
		require.NoError(t, err)
		assert.Equal(t, end-off, n)
	}

	assert.Equal(t, digest, sw.GetChecksum(), "checksum must be computed over the whole stream")
	assert.Equal(t, int64(len(content)), sw.size)

	// The bytes forwarded to the client must reassemble to the exact content.
	forwarded := make([]byte, 0, len(content))
	for _, c := range fake.sent {
		forwarded = append(forwarded, c...)
	}
	assert.Equal(t, content, forwarded)
}

// TestStreamWriter_EmptyStreamChecksum: with no writes the computed checksum is
// the sha256 of the empty input.
func TestStreamWriter_EmptyStreamChecksum(t *testing.T) {
	sw := &streamWriter{stream: &fakeReadServer{}, log: servicelogger.EmptyLogger(), gotChecksum: sha256.New()}
	assert.Equal(t, sha256Hex([]byte{}), sw.GetChecksum())
	assert.Equal(t, int64(0), sw.size)
}

// recvAllDownload drains a Read stream, returning the concatenated payload and
// the terminating error (io.EOF becomes nil).
func recvAllDownload(reader bytestream.ByteStream_ReadClient) ([]byte, error) {
	var buf []byte
	for {
		resp, err := reader.Recv()
		if resp != nil {
			buf = append(buf, resp.Data...)
		}
		if errors.Is(err, io.EOF) {
			return buf, nil
		}
		if err != nil {
			return buf, err
		}
	}
}

// TestDownloadMultiChunkChecksumOK: content the backend emits across several
// writes is streamed back intact and passes digest verification.
func (s *bytestreamSuite) TestDownloadMultiChunkChecksumOK() {
	content := deterministicBytes(200 * 1024)
	digest := sha256Hex(content)

	s.ociBackend.On("Download", mock.Anything, mock.Anything, digest).Return(nil).
		Run(func(args mock.Arguments) {
			w := args.Get(1).(io.Writer)
			for off := 0; off < len(content); off += 8192 {
				end := off + 8192
				if end > len(content) {
					end = len(content)
				}
				_, err := w.Write(content[off:end])
				s.NoError(err)
			}
		})

	reader, err := s.client.Read(s.downCtx, &bytestream.ReadRequest{ResourceName: digest})
	s.NoError(err)

	got, err := recvAllDownload(reader)
	s.NoError(err)
	s.Equal(content, got)
	s.Equal(digest, sha256Hex(got))

	s.Require().Len(s.audit.published, 1)
	info := decodeArtifactEvent(s.T(), s.audit.published[0])
	s.Equal(digest, info.Digest)
	s.Equal(int64(len(content)), info.SizeBytes)
	s.False(info.Skipped)
}

// TestDownloadEmptyContentChecksum: a zero-byte artifact whose requested digest
// is sha256("") verifies and streams back empty.
func (s *bytestreamSuite) TestDownloadEmptyContentChecksum() {
	digest := sha256Hex([]byte{})

	s.ociBackend.On("Download", mock.Anything, mock.Anything, digest).Return(nil).
		Run(func(_ mock.Arguments) {
			// write nothing
		})

	reader, err := s.client.Read(s.downCtx, &bytestream.ReadRequest{ResourceName: digest})
	s.NoError(err)

	got, err := recvAllDownload(reader)
	s.NoError(err)
	s.Empty(got)

	s.Require().Len(s.audit.published, 1)
	info := decodeArtifactEvent(s.T(), s.audit.published[0])
	s.Equal(int64(0), info.SizeBytes)
}

// TestDownloadTamperedAcrossChunksRejected: if the streamed bytes do not hash to
// the requested digest, the server reports a checksum mismatch and emits no
// audit event — the tamper-detection guarantee of a CAS.
func (s *bytestreamSuite) TestDownloadTamperedAcrossChunksRejected() {
	// The client asks for this digest, but the backend returns different bytes.
	requested := sha256Hex([]byte("the authentic artifact contents"))
	tampered := []byte("tampered artifact contents split across chunks!!")

	s.ociBackend.On("Download", mock.Anything, mock.Anything, requested).Return(nil).
		Run(func(args mock.Arguments) {
			w := args.Get(1).(io.Writer)
			_, _ = w.Write(tampered[:20])
			_, _ = w.Write(tampered[20:])
		})

	reader, err := s.client.Read(s.downCtx, &bytestream.ReadRequest{ResourceName: requested})
	s.NoError(err)

	_, err = recvAllDownload(reader)
	s.ErrorContains(err, "checksum mismatch")
	// tampered downloads emit no events
	s.Empty(s.audit.published)
}

// TestDownloadClientDisconnect: a backend Download failure that indicates the
// client went away is treated as a cancellation (no error to a gone client, no
// audit). This is pre-existing download behavior; the test pins it.
func (s *bytestreamSuite) TestDownloadClientDisconnect() {
	const digest = "deadbeef"
	s.ociBackend.On("Download", mock.Anything, mock.Anything, digest).
		Return(fmt.Errorf("stream send failed: %w", context.Canceled))

	reader, err := s.client.Read(s.downCtx, &bytestream.ReadRequest{ResourceName: digest})
	s.NoError(err)

	// Server returns nil (treats as disconnect); the client sees a clean EOF.
	_, err = recvAllDownload(reader)
	s.NoError(err)
	s.Empty(s.audit.published)
}

// TestDownloadBackendErrorMasked: a genuine backend Download failure is masked
// as Internal and emits no audit event.
func (s *bytestreamSuite) TestDownloadBackendErrorMasked() {
	const digest = "deadbeef"
	s.ociBackend.On("Download", mock.Anything, mock.Anything, digest).
		Return(fmt.Errorf("object store unreachable"))

	reader, err := s.client.Read(s.downCtx, &bytestream.ReadRequest{ResourceName: digest})
	s.NoError(err)

	_, err = recvAllDownload(reader)
	assertGRPCError(s.T(), err, codes.Internal, "server error")
	s.Empty(s.audit.published)
}
