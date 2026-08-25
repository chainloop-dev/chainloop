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
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

const streamingBackendType = "streaming-backend-type"

// --- test helpers ---------------------------------------------------------

// streamingUpCtx returns an uploader context routed to the streaming backend.
// maxBytes == "" leaves the per-upload cap unset (unlimited).
func streamingUpCtx(maxBytes string) context.Context {
	pairs := []string{"role", "uploader", "backend-streaming", "true"}
	if maxBytes != "" {
		pairs = append(pairs, "max-bytes", maxBytes)
	}
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs(pairs...))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// resourceWithDigest builds a CASResource whose Digest is the real sha256 of
// content, so integrity assertions can compare the bytes the backend received
// against the declared digest.
func resourceWithDigest(content []byte, fileName string) *v1.CASResource {
	return &v1.CASResource{Digest: sha256Hex(content), FileName: fileName}
}

// deterministicBytes returns n reproducible, non-trivial bytes (a rolling
// pattern) so multi-chunk integrity tests exercise real content, not zeros.
func deterministicBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte((i*31 + 7) % 251)
	}
	return b
}

// sendInChunks writes content to the stream in chunkSize slices, re-sending the
// resource name on every message (as the Chainloop CLI does). A zero-length
// content still sends one message carrying the resource name.
func sendInChunks(t *testing.T, stream bytestream.ByteStream_WriteClient, resourceB64 string, content []byte, chunkSize int) {
	t.Helper()
	if len(content) == 0 {
		require.NoError(t, stream.Send(&bytestream.WriteRequest{ResourceName: resourceB64}))
		return
	}
	for off := 0; off < len(content); off += chunkSize {
		end := off + chunkSize
		if end > len(content) {
			end = len(content)
		}
		require.NoError(t, stream.Send(&bytestream.WriteRequest{
			ResourceName: resourceB64,
			Data:         content[off:end],
		}))
	}
}

// expectStreamingUpload stubs Exists=false and an Upload that fully drains the
// reader, returning the captured bytes on the returned channel and reporting
// uploadErr to the client.
func (s *bytestreamSuite) expectStreamingUpload(resource *v1.CASResource, uploadErr error) <-chan []byte {
	received := make(chan []byte, 1)
	s.streamingBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)
	s.streamingBackend.On("Upload", mock.Anything, mock.Anything, resource).
		Return(uploadErr).Run(func(args mock.Arguments) {
		got, err := io.ReadAll(args.Get(1).(io.Reader))
		s.NoError(err)
		received <- got
	})
	return received
}

// --- upload integrity verification -----------------------------------------

// TestWriteDigestMismatchRejected is the core upload-integrity guarantee: bytes
// that do not hash to the client-declared digest are rejected with
// InvalidArgument, nothing is ever sent to the backend, and no audit event is
// emitted. Without verification an attacker could store arbitrary content under
// an arbitrary digest key.
func (s *bytestreamSuite) TestWriteDigestMismatchRejected() {
	content := []byte("this is the real content that was actually streamed")
	// The declared digest belongs to DIFFERENT content than what is sent.
	resource := &v1.CASResource{Digest: sha256Hex([]byte("something else entirely")), FileName: "artifact.bin"}
	s.streamingBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)
	// Upload must NOT be called for a mismatching digest.

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, 7)

	_, err = stream.CloseAndRecv()
	assertGRPCError(s.T(), err, codes.InvalidArgument, "does not match the declared digest")
	s.streamingBackend.AssertNotCalled(s.T(), "Upload", mock.Anything, mock.Anything, mock.Anything)
	s.Empty(s.audit.published)
}

// TestWriteBackendReceivesSeekableFile asserts the backend's Upload is handed a
// value satisfying io.ReaderAt+io.Seeker (an *os.File), so the AWS SDK's
// zero-buffer fast path is taken. Wrapping the file on the way to Upload (a
// TeeReader, progress reader, LimitReader) would silently reinstate part
// buffering, so this guards against that regression.
func (s *bytestreamSuite) TestWriteBackendReceivesSeekableFile() {
	content := deterministicBytes(64 * 1024)
	resource := resourceWithDigest(content, "artifact.bin")
	var isReaderAt, isSeeker bool
	s.streamingBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)
	s.streamingBackend.On("Upload", mock.Anything, mock.Anything, resource).Return(nil).Run(func(args mock.Arguments) {
		r := args.Get(1)
		_, isReaderAt = r.(io.ReaderAt)
		_, isSeeker = r.(io.Seeker)
		_, _ = io.ReadAll(r.(io.Reader))
	})

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, 8192)

	_, err = stream.CloseAndRecv()
	s.NoError(err)
	s.True(isReaderAt, "backend must receive an io.ReaderAt for the SDK zero-buffer fast path")
	s.True(isSeeker, "backend must receive an io.Seeker for the SDK zero-buffer fast path")
}

// requireStagingEmpty asserts the per-test staging directory holds no files, so
// verified-and-uploaded or rejected content never accumulates on disk.
func (s *bytestreamSuite) requireStagingEmpty() {
	entries, err := os.ReadDir(s.stagingDir)
	s.Require().NoError(err)
	s.Emptyf(entries, "staging dir must be left empty, found: %v", entries)
}

// TestWriteStagingCleanupOnSuccess: after a successful upload the staging file
// is removed.
func (s *bytestreamSuite) TestWriteStagingCleanupOnSuccess() {
	content := []byte("staged, verified, then uploaded")
	resource := resourceWithDigest(content, "ok.bin")
	received := s.expectStreamingUpload(resource, nil)

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, 8)

	_, err = stream.CloseAndRecv()
	s.NoError(err)
	<-received
	s.requireStagingEmpty()
}

// TestWriteStagingCleanupOnMismatch: after a rejected digest mismatch the
// staging file is removed too — unverified bytes never linger.
func (s *bytestreamSuite) TestWriteStagingCleanupOnMismatch() {
	content := []byte("content that will not match the declared digest")
	resource := &v1.CASResource{Digest: sha256Hex([]byte("different")), FileName: "bad.bin"}
	s.streamingBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, 8)

	_, err = stream.CloseAndRecv()
	assertGRPCError(s.T(), err, codes.InvalidArgument, "does not match the declared digest")
	s.requireStagingEmpty()
}

// --- integrity / normal cases --------------------------------------------

// TestWriteStreamingSingleChunkOK: a single-chunk streaming upload stores the
// exact bytes, reports the committed size, and emits an audit event.
func (s *bytestreamSuite) TestWriteStreamingSingleChunkOK() {
	content := []byte("hello streaming world")
	resource := resourceWithDigest(content, "artifact.bin")
	received := s.expectStreamingUpload(resource, nil)

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, len(content))

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(len(content)), got.CommittedSize)
	s.Equal(content, <-received)

	s.Require().Len(s.audit.published, 1)
	info := decodeArtifactEvent(s.T(), s.audit.published[0])
	s.False(info.Skipped)
	s.Equal(resource.Digest, info.Digest)
	s.Equal(int64(len(content)), info.SizeBytes)
}

// TestWriteStreamingMultiChunkIntegrity: content split across several small
// chunks is reassembled byte-for-byte, and the sha256 of what the backend
// received matches the declared digest — no corruption/reordering/truncation.
func (s *bytestreamSuite) TestWriteStreamingMultiChunkIntegrity() {
	content := []byte("the quick brown fox jumps over the lazy dog, twice over for good measure")
	resource := resourceWithDigest(content, "artifact.bin")
	received := s.expectStreamingUpload(resource, nil)

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, 7)

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(len(content)), got.CommittedSize)

	gotBytes := <-received
	s.Equal(content, gotBytes)
	s.Equal(resource.Digest, sha256Hex(gotBytes), "backend content digest must match the declared digest")
}

// TestWriteStreamingManyChunksIntegrity exercises the pipe under many
// iterations with a larger payload and verifies integrity end-to-end.
func (s *bytestreamSuite) TestWriteStreamingManyChunksIntegrity() {
	content := deterministicBytes(512 * 1024) // 512 KiB
	resource := resourceWithDigest(content, "big.bin")
	received := s.expectStreamingUpload(resource, nil)

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, 4096)

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(len(content)), got.CommittedSize)

	gotBytes := <-received
	s.Equal(resource.Digest, sha256Hex(gotBytes))
	s.Len(gotBytes, len(content))
}

// TestWriteStreamingEmptyArtifact: a zero-byte artifact streams cleanly and is
// committed with size 0.
func (s *bytestreamSuite) TestWriteStreamingEmptyArtifact() {
	content := []byte{}
	resource := resourceWithDigest(content, "empty.bin")
	received := s.expectStreamingUpload(resource, nil)

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, 1024)

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(0), got.CommittedSize)
	s.Empty(<-received)

	s.Require().Len(s.audit.published, 1)
	info := decodeArtifactEvent(s.T(), s.audit.published[0])
	s.Equal(int64(0), info.SizeBytes)
}

// TestWriteStreamingFinishWriteWithData: a spec-compliant client may set
// finish_write=true on the final message while it still carries Data; that
// trailing chunk must be stored, not dropped/truncated (google.bytestream
// allows the terminal message to carry payload).
func (s *bytestreamSuite) TestWriteStreamingFinishWriteWithData() {
	content := []byte("hello world")
	resource := resourceWithDigest(content, "artifact.bin")
	received := s.expectStreamingUpload(resource, nil)

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	rb := encodeResource(s.T(), resource)
	// First chunk with no finish, then the final chunk carrying data AND finish.
	s.NoError(stream.Send(&bytestream.WriteRequest{ResourceName: rb, Data: content[:5]}))
	s.NoError(stream.Send(&bytestream.WriteRequest{ResourceName: rb, Data: content[5:], FinishWrite: true}))

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(len(content)), got.CommittedSize)

	gotBytes := <-received
	s.Equal(content, gotBytes)
	s.Equal(resource.Digest, sha256Hex(gotBytes), "the finish_write chunk's data must be stored, not dropped")
}

// --- size-cap boundaries --------------------------------------------------

// TestWriteStreamingMaxSizeExceededMultiChunk: the cap trips across chunks and
// surfaces as ResourceExhausted, emitting no audit event.
func (s *bytestreamSuite) TestWriteStreamingMaxSizeExceededMultiChunk() {
	s.streamingBackend.On("Exists", mock.Anything, s.resource.Digest).Return(false, nil)
	// The backend may or may not observe the truncated stream; drain if called.
	s.streamingBackend.On("Upload", mock.Anything, mock.Anything, s.resource).Maybe().
		Return(nil).Run(func(args mock.Arguments) {
		_, _ = io.ReadAll(args.Get(1).(io.Reader))
	})

	stream, err := s.client.Write(streamingUpCtx("8"))
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
		Data:         []byte("hello"),
	}))
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
		Data:         []byte("chainloop"),
	}))

	_, err = stream.CloseAndRecv()
	assertGRPCError(s.T(), err, codes.ResourceExhausted, "max size of upload exceeded")
	s.Empty(s.audit.published)
}

// TestWriteStreamingMaxSizeExceededFirstChunk: a first chunk that alone exceeds
// the cap is rejected with ResourceExhausted.
func (s *bytestreamSuite) TestWriteStreamingMaxSizeExceededFirstChunk() {
	s.streamingBackend.On("Exists", mock.Anything, s.resource.Digest).Return(false, nil)
	s.streamingBackend.On("Upload", mock.Anything, mock.Anything, s.resource).Maybe().
		Return(nil).Run(func(args mock.Arguments) {
		_, _ = io.ReadAll(args.Get(1).(io.Reader))
	})

	stream, err := s.client.Write(streamingUpCtx("4"))
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
		Data:         []byte("too-long"),
	}))

	_, err = stream.CloseAndRecv()
	assertGRPCError(s.T(), err, codes.ResourceExhausted, "max size of upload exceeded")
	s.Empty(s.audit.published)
}

// TestWriteStreamingMaxSizeBoundaryExact: content exactly equal to the cap is
// accepted (the cap is exceeded only when size > max).
func (s *bytestreamSuite) TestWriteStreamingMaxSizeBoundaryExact() {
	content := []byte("12345678") // 8 bytes
	resource := resourceWithDigest(content, "artifact.bin")
	received := s.expectStreamingUpload(resource, nil)

	stream, err := s.client.Write(streamingUpCtx("8"))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, 3)

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(8), got.CommittedSize)
	s.Equal(content, <-received)
}

// TestWriteStreamingMaxSizeUnlimited: max-bytes==0 imposes no cap.
func (s *bytestreamSuite) TestWriteStreamingMaxSizeUnlimited() {
	content := deterministicBytes(200 * 1024)
	resource := resourceWithDigest(content, "artifact.bin")
	received := s.expectStreamingUpload(resource, nil)

	// note: streamingUpCtx("") leaves max-bytes unset → claims.MaxBytes == 0
	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	sendInChunks(s.T(), stream, encodeResource(s.T(), resource), content, 8192)

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(len(content)), got.CommittedSize)
	s.Equal(resource.Digest, sha256Hex(<-received))
}

// --- error / disconnect classification ------------------------------------

// TestWriteStreamingBackendError: a generic backend Upload failure surfaces as
// Internal and emits no audit event.
func (s *bytestreamSuite) TestWriteStreamingBackendError() {
	data := []byte("hello world")
	resource := resourceWithDigest(data, "artifact.bin")
	s.streamingBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)
	s.streamingBackend.On("Upload", mock.Anything, mock.Anything, resource).
		Return(fmt.Errorf("object store rejected the upload")).Run(func(args mock.Arguments) {
		_, _ = io.ReadAll(args.Get(1).(io.Reader))
	})

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), resource),
		Data:         data,
	}))

	_, err = stream.CloseAndRecv()
	assertGRPCError(s.T(), err, codes.Internal, "server error")
	s.Empty(s.audit.published)
}

// TestWriteStreamingBackendResetNotTreatedAsDisconnect: a backend-side failure
// wrapping a network reset must be masked as Internal, NOT mistaken for a client
// disconnect (which would falsely report success and silently drop the blob).
func (s *bytestreamSuite) TestWriteStreamingBackendResetNotTreatedAsDisconnect() {
	data := []byte("hello world")
	resource := resourceWithDigest(data, "artifact.bin")
	s.streamingBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)
	s.streamingBackend.On("Upload", mock.Anything, mock.Anything, resource).
		Return(fmt.Errorf("connection to object store failed: %w", syscall.ECONNRESET)).
		Run(func(args mock.Arguments) { _, _ = io.ReadAll(args.Get(1).(io.Reader)) })

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), resource),
		Data:         data,
	}))

	_, err = stream.CloseAndRecv()
	// "server error" is the masked-error message; the buggy disconnect path
	// would instead return nil and surface a gRPC "cardinality violation".
	assertGRPCError(s.T(), err, codes.Internal, "server error")
	s.Empty(s.audit.published)
}

// TestWriteStreamingBackendCanceledNotTreatedAsDisconnect: same guard for an
// Upload error wrapping context.Canceled originating backend-side.
func (s *bytestreamSuite) TestWriteStreamingBackendCanceledNotTreatedAsDisconnect() {
	data := []byte("hello world")
	resource := resourceWithDigest(data, "artifact.bin")
	s.streamingBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)
	s.streamingBackend.On("Upload", mock.Anything, mock.Anything, resource).
		Return(fmt.Errorf("backend deadline: %w", context.Canceled)).
		Run(func(args mock.Arguments) { _, _ = io.ReadAll(args.Get(1).(io.Reader)) })

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), resource),
		Data:         data,
	}))

	_, err = stream.CloseAndRecv()
	assertGRPCError(s.T(), err, codes.Internal, "server error")
	s.Empty(s.audit.published)
}

// TestWriteStreamingBackendSuccessWithoutDrain: a backend that returns success
// without reading the reader to EOF must be reported as a successful upload; the
// service's own reader-close must not surface as an Internal error.
func (s *bytestreamSuite) TestWriteStreamingBackendSuccessWithoutDrain() {
	data := []byte("hello world")
	resource := resourceWithDigest(data, "artifact.bin")
	s.streamingBackend.On("Exists", mock.Anything, resource.Digest).Return(false, nil)
	s.streamingBackend.On("Upload", mock.Anything, mock.Anything, resource).Return(nil) // no drain

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), resource),
		Data:         data,
	}))

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(len(data)), got.CommittedSize)

	s.Require().Len(s.audit.published, 1)
	info := decodeArtifactEvent(s.T(), s.audit.published[0])
	s.False(info.Skipped)
}

// TestWriteStreamingClientDisconnect: when the client cancels mid-upload, the
// server aborts before verification completes, so nothing is ever sent to the
// backend and no audit event is emitted.
func (s *bytestreamSuite) TestWriteStreamingClientDisconnect() {
	// Exists may or may not be reached depending on how fast the cancellation
	// races the handler; the invariant under test is that Upload never is.
	s.streamingBackend.On("Exists", mock.Anything, s.resource.Digest).Maybe().Return(false, nil)

	ctx, cancel := context.WithCancel(streamingUpCtx(""))
	stream, err := s.client.Write(ctx)
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
		Data:         []byte("partial upload"),
	}))
	cancel()

	_, err = stream.CloseAndRecv()
	// The client canceled: the RPC ends in error and nothing is stored.
	s.Error(err)
	s.Empty(s.audit.published)
	s.streamingBackend.AssertNotCalled(s.T(), "Upload", mock.Anything, mock.Anything, mock.Anything)
}

// --- dedup ----------------------------------------------------------------

// TestWriteStreamingExist: an already-present artifact is not re-uploaded.
func (s *bytestreamSuite) TestWriteStreamingExist() {
	s.streamingBackend.On("Exists", mock.Anything, s.resource.Digest).Return(true, nil)
	s.streamingBackend.On("Describe", mock.Anything, s.resource.Digest).Return(&v1.CASResource{
		FileName: s.resource.FileName, Digest: s.resource.Digest, Size: 2048,
	}, nil)

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
	}))

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(0), got.CommittedSize)
	s.streamingBackend.AssertNotCalled(s.T(), "Upload", mock.Anything, mock.Anything, mock.Anything)

	s.Require().Len(s.audit.published, 1)
	info := decodeArtifactEvent(s.T(), s.audit.published[0])
	s.True(info.Skipped)
	s.Equal(int64(2048), info.SizeBytes)
}

// TestWriteStreamingExistError: an Exists lookup failure is masked as Internal.
func (s *bytestreamSuite) TestWriteStreamingExistError() {
	s.streamingBackend.On("Exists", mock.Anything, s.resource.Digest).
		Return(false, fmt.Errorf("backend unreachable"))

	stream, err := s.client.Write(streamingUpCtx(""))
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
		Data:         []byte("hello world"),
	}))

	_, err = stream.CloseAndRecv()
	assertGRPCError(s.T(), err, codes.Internal, "server error")
	s.Empty(s.audit.published)
}
