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
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc/codes"
)

// These tests lock down the DOWNLOAD digest-verification behavior — the server
// must stage the stored bytes on disk, compute their sha256 across however many
// chunks the backend produces, and reject any content whose digest does not
// match the requested resource name BEFORE the first byte reaches the client.

// fakeReadServer is a minimal bytestream.ByteStream_ReadServer that records the
// data chunks the sendWriter sends. Only Send is exercised by sendWriter.
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

// TestSendWriter_ForwardsChunksInOrder verifies the download adapter turns
// every Write into one ReadResponse carrying exactly those bytes, in order.
// Hashing is not its job: content is verified on disk before the first Send.
func TestSendWriter_ForwardsChunksInOrder(t *testing.T) {
	content := []byte("chainloop download payload delivered in several writes")

	fake := &fakeReadServer{}
	sw := sendWriter{stream: fake}

	// Feed the content in uneven pieces to mimic arbitrary chunk boundaries.
	for off := 0; off < len(content); off += 11 {
		end := min(off+11, len(content))
		n, err := sw.Write(content[off:end])
		require.NoError(t, err)
		assert.Equal(t, end-off, n)
	}

	// The bytes forwarded to the client must reassemble to the exact content.
	forwarded := make([]byte, 0, len(content))
	for _, c := range fake.sent {
		forwarded = append(forwarded, c...)
	}
	assert.Equal(t, content, forwarded)
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

// TestDownloadTamperedAcrossChunksRejected: if the stored bytes do not hash to
// the requested digest, the server reports the content as lost/corrupt, sends
// NOT A SINGLE BYTE to the client and emits no audit event — the
// tamper-detection guarantee of a CAS.
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

	got, err := recvAllDownload(reader)
	assertGRPCError(s.T(), err, codes.DataLoss, "does not match the requested digest")
	s.ErrorContains(err, "got="+sha256Hex(tampered))
	s.Empty(got, "no unverified byte may reach the client")
	// tampered downloads emit no events
	s.Empty(s.audit.published)
}

// TestDownloadStagingFailureMasked: when the download cannot be staged on disk
// (here: the staging directory vanished) the failure is masked as Internal,
// nothing is sent to the client and no audit event is emitted.
func (s *bytestreamSuite) TestDownloadStagingFailureMasked() {
	content := []byte("never leaves the server")
	digest := sha256Hex(content)
	s.Require().NoError(os.RemoveAll(s.stagingDir))

	reader, err := s.client.Read(s.downCtx, &bytestream.ReadRequest{ResourceName: digest})
	s.NoError(err)

	got, err := recvAllDownload(reader)
	assertGRPCError(s.T(), err, codes.Internal, "server error")
	s.Empty(got)
	s.Empty(s.audit.published)
	// the backend is never asked for content that cannot be staged
	s.ociBackend.AssertNotCalled(s.T(), "Download", mock.Anything, mock.Anything, mock.Anything)
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
