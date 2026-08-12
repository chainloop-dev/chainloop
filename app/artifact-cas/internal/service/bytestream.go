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

package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"hash"
	"io"

	"errors"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	sl "github.com/chainloop-dev/chainloop/pkg/servicelogger"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var byteStreamTracer = otelx.Tracer("chainloop-cas", "cas/service/bytestream")

// Implements the bytestream interface
// https://github.com/googleapis/googleapis/blob/master/google/bytestream/bytestream.proto#L49
// specifically both the write and the read methods
type ByteStreamService struct {
	*bytestream.UnimplementedByteStreamServer
	*commonService
}

func NewByteStreamService(bp backend.Providers, opts ...NewOpt) *ByteStreamService {
	return &ByteStreamService{
		commonService: newCommonService(bp, opts...),
	}
}

// Client-side streaming RPC for writing blobs.
// Iterate on the stream of file chunks, aggregate them in a buffer,
// send them to the backend and return a response with the commitedSize
func (s *ByteStreamService) Write(stream bytestream.ByteStream_WriteServer) error {
	ctx := stream.Context()
	ctx, span := otelx.Start(ctx, byteStreamTracer, "ByteStreamService.Write")
	defer span.End()

	// Get auth info and check that it's an uploader token
	info, err := casJWT.InfoFromAuth(ctx)
	if err != nil {
		return err
	}

	if err := info.CheckRole(casJWT.Uploader); err != nil {
		return kerrors.Unauthorized("service", err.Error())
	}

	// Get the digest of the artifact that we want to upload from the first chunk
	// so we can check if it already exists before accepting any other chunks in the background
	req, err := getWriteRequest(stream)
	if err != nil {
		return kerrors.BadRequest("resource name", err.Error())
	}

	storageBackend, err := s.loadBackend(ctx, info.BackendType, info.StoredSecretID)
	if err != nil && kerrors.IsNotFound(err) {
		return err
	} else if err != nil {
		return sl.LogAndMaskErr(err, s.log)
	}

	// We check if the file already exists even before we wait for the whole buffer to be filled
	if exists, err := storageBackend.Exists(ctx, req.resource.Digest); err != nil {
		return sl.LogAndMaskErr(err, s.log)
	} else if exists {
		s.log.Infow("msg", "artifact already exists", "digest", req.resource.Digest)
		if s.audit.shouldEmit(info) {
			// the stored size is not known at the dedup point, look it up best-effort
			var size int64
			if r, err := storageBackend.Describe(ctx, req.resource.Digest); err == nil {
				size = r.Size
			}

			s.audit.Dispatch(&events.CASArtifactUploaded{
				CASArtifactBase: &events.CASArtifactBase{
					Digest:      req.resource.Digest,
					SizeBytes:   size,
					FileName:    req.resource.FileName,
					BackendType: info.BackendType,
				},
				Skipped: true,
			}, info)
		}
		return stream.SendAndClose(&bytestream.WriteResponse{})
	}

	s.log.Infow("msg", "artifact does not exist, uploading", "digest", req.resource.Digest, "name", req.resource.FileName)

	// Streaming-capable backends (object stores such as S3/Azure) are fed
	// directly from the client stream through an io.Pipe, so CAS memory stays
	// bounded by the chunk/pipe size regardless of artifact size (PFM-6923).
	// The OCI backend, whose push path needs the whole layer content up front,
	// does not advertise streaming and keeps the fully-buffered path.
	var committedSize int64
	if su, ok := storageBackend.(backend.StreamingUploader); ok && su.SupportsStreaming() {
		committedSize, err = s.streamUpload(ctx, stream, storageBackend, req, info.MaxBytes)
	} else {
		committedSize, err = s.bufferedUpload(ctx, stream, storageBackend, req, info.MaxBytes)
	}

	// Classify the outcome. The error may come from two distinct stages, which
	// must be treated differently:
	//   - A backend Upload failure (backendUploadError) is always masked as an
	//     internal error. It must NOT be interpreted as a client disconnect even
	//     when it wraps a network reset/cancellation originating backend-side —
	//     doing so would falsely report success and silently drop the artifact.
	//   - A stream-read (feed) error is classified: a client disconnect is not a
	//     failure, an exceeded size cap maps to ResourceExhausted, anything else
	//     is masked.
	if err != nil {
		var backendErr *backendUploadError
		if errors.As(err, &backendErr) {
			return sl.LogAndMaskErr(backendErr.err, s.log)
		}
		if isClientDisconnect(err) {
			s.log.Infow("msg", "upload canceled", "digest", req.resource.Digest, "name", req.resource.FileName)
			return nil
		}
		if backend.IsUploadSizeExceeded(err) {
			return status.Error(codes.ResourceExhausted, err.Error())
		}
		return sl.LogAndMaskErr(err, s.log)
	}

	s.log.Infow("msg", "upload finished", "name", req.resource.FileName, "digest", req.resource.Digest, "size", committedSize)
	s.audit.Dispatch(&events.CASArtifactUploaded{
		CASArtifactBase: &events.CASArtifactBase{
			Digest:      req.resource.Digest,
			SizeBytes:   committedSize,
			FileName:    req.resource.FileName,
			BackendType: info.BackendType,
		},
	}, info)

	return stream.SendAndClose(&bytestream.WriteResponse{CommittedSize: committedSize})
}

// bufferedUpload accumulates the whole artifact in memory before handing it to
// the backend. This is required by the OCI backend: its push implementation
// does not support streaming/chunked uploads for uncompressed layers (we can not
// use stream.Layer since it only supports compressed layers, and we want to
// store raw data with custom mimetypes), so it needs the full content up front.
// https://github.com/google/go-containerregistry/blob/main/pkg/v1/stream/README.md
// It returns the total number of bytes committed to the backend. Feed errors are
// returned unwrapped (classified by the caller); backend Upload failures are
// wrapped in backendUploadError so the caller always masks them.
func (s *ByteStreamService) bufferedUpload(ctx context.Context, stream bytestream.ByteStream_WriteServer, storageBackend backend.Uploader, req *writeRequest, maxBytes int64) (int64, error) {
	// Create a buffer that will be filled in the background before sending its content to the backend
	buffer := newStreamReader(maxBytes)
	// Add data from the first request
	if err := buffer.Write(req.GetData()); err != nil {
		return 0, err
	}

	// Start a goroutine that will fill the buffer in the background
	go bufferStream(ctx, stream, buffer, s.log)

	// Block until the buffer has been filled or the upload process has been canceled
	if err := <-buffer.errorChan; err != nil {
		return 0, err
	}

	s.log.Infow("msg", "artifact received, uploading now to backend", "name", req.resource.FileName, "digest", req.resource.Digest, "size", buffer.size)
	if err := storageBackend.Upload(ctx, buffer, req.resource); err != nil {
		return 0, &backendUploadError{err}
	}

	return buffer.size, nil
}

// streamUpload pipes the client stream straight into the backend's Upload
// without buffering the whole artifact in memory. A background goroutine feeds
// received chunks into an io.Pipe while Upload consumes the other end, so the
// two run concurrently and peak memory stays bounded (PFM-6923). It returns the
// total number of bytes committed to the backend.
//
// Tradeoff vs. the buffered path: because bytes are forwarded as they arrive, an
// over-cap upload has already streamed up to maxBytes to the backend by the time
// the size guard trips (the client is still correctly rejected with
// ResourceExhausted). For multipart backends the SDK aborts the in-flight upload
// best-effort; operators should keep a bucket lifecycle rule to reap any
// incomplete multipart uploads a failed abort could leave behind. This is
// inherent to streaming — the alternative (buffer-then-validate) is exactly the
// OOM this change removes.
func (s *ByteStreamService) streamUpload(ctx context.Context, stream bytestream.ByteStream_WriteServer, storageBackend backend.Uploader, req *writeRequest, maxBytes int64) (int64, error) {
	pr, pw := io.Pipe()

	var (
		uploadedSize int64
		feedErr      error
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		uploadedSize, feedErr = feedPipe(ctx, stream, pw, req.GetData(), maxBytes, s.log, req.resource.Digest)
		// Closing with feedErr signals EOF to the reader when nil, or propagates
		// the failure so Upload stops reading.
		_ = pw.CloseWithError(feedErr)
	}()

	uploadErr := storageBackend.Upload(ctx, streamingReader{pr}, req.resource)
	// If Upload returned without draining the pipe (a backend failure, or a
	// backend that reports success without reading to EOF), the feeding goroutine
	// may still be blocked on Write; closing the read end unblocks it. Then wait
	// for it so uploadedSize/feedErr are safe to read.
	_ = pr.CloseWithError(uploadErr)
	<-done

	// errPipeConsumerGone means the feed only failed because the reader (Upload)
	// stopped consuming — a consequence of the upload outcome, not a genuine
	// stream-read failure, so the backend's own result is authoritative.
	if errors.Is(feedErr, errPipeConsumerGone) {
		feedErr = nil
	}

	// A genuine feed-side error (client disconnect, exceeded size cap, stream
	// read failure) is the precise signal and takes precedence: when it occurs it
	// is what induced the backend error through the pipe. Returned unwrapped so
	// the caller classifies it (disconnect / ResourceExhausted / mask).
	if feedErr != nil {
		return 0, feedErr
	}
	// A backend failure is wrapped so the caller always masks it, never mistaking
	// a backend-side reset/cancellation for a client disconnect.
	if uploadErr != nil {
		return 0, &backendUploadError{uploadErr}
	}

	return uploadedSize, nil
}

// backendUploadError marks a failure returned by the storage backend's Upload,
// as opposed to an error reading the client stream. Backend failures are always
// masked as internal errors and are never interpreted as a client disconnect or
// a size-cap violation, both of which only originate on the stream-read side.
type backendUploadError struct{ err error }

func (e *backendUploadError) Error() string { return e.err.Error() }
func (e *backendUploadError) Unwrap() error { return e.err }

// errPipeConsumerGone is returned by feedPipe when a write to the pipe fails,
// which only happens once the reader (the backend Upload) has stopped consuming
// — because Upload returned and streamUpload closed the read end, or because it
// failed. It is not a genuine stream-read failure; streamUpload defers to the
// backend's own error in that case.
var errPipeConsumerGone = errors.New("pipe consumer stopped reading")

// streamingReader wraps the upload pipe reader with a stable string form. The
// pipe is written to concurrently while the backend reads it; exposing the bare
// *io.PipeReader lets a reflective consumer (a structured logger, a test's mock
// matcher, etc.) walk the pipe's internal state and race with the writer. The
// wrapper keeps io.Reader behaviour while presenting an opaque identity to fmt.
type streamingReader struct {
	io.Reader
}

func (streamingReader) String() string { return "cas-streaming-upload" }

// feedPipe forwards the artifact from the client stream into pw, enforcing the
// max upload size as it goes. firstData is the payload already read from the
// first request. It returns the total number of bytes forwarded.
func feedPipe(ctx context.Context, stream bytestream.ByteStream_WriteServer, pw *io.PipeWriter, firstData []byte, maxSize int64, log *log.Helper, digest string) (int64, error) {
	var size int64
	write := func(data []byte) error {
		if len(data) == 0 {
			return nil
		}
		size += int64(len(data))
		if err := checkUploadSize(size, maxSize); err != nil {
			return err
		}
		if _, err := pw.Write(data); err != nil {
			// A write only fails once the reader has gone away; surface it as the
			// consumer-gone sentinel so streamUpload defers to the backend result
			// rather than treating this as a client-side stream failure.
			return errPipeConsumerGone
		}
		return nil
	}

	// Forward the data from the first request.
	if err := write(firstData); err != nil {
		return size, err
	}

	for {
		select {
		case <-ctx.Done():
			// DeadlineExceeded, or Canceled
			return size, ctx.Err()
		default:
			// Extract the next chunk of data from the stream request
			req, err := getWriteRequest(stream)
			if err != nil {
				// Finished reading the stream is not a real error
				if errors.Is(err, io.EOF) {
					return size, nil
				}
				return size, err
			}

			// Forward this request's data first: a spec-compliant client may set
			// finish_write=true on the same message that carries the final chunk,
			// so the data must be written before the finish check or it is lost.
			if err := write(req.GetData()); err != nil {
				return size, err
			}

			log.Debugw("msg", "upload chunk received (streaming)", "digest", digest, "currentSize", size, "maxSize", maxSize, "chunkSize", len(req.GetData()))

			// Check if the client has finished sending data
			if req.GetFinishWrite() {
				return size, nil
			}
		}
	}
}

// Server-side streaming RPC for reading blobs, implements the bytestream interface
// NOTE: Due to the fact that we are using the OCI backend, we can not stream the content directly from the backend
// but instead we need to download the whole artifact and then stream it to the client
func (s *ByteStreamService) Read(req *bytestream.ReadRequest, stream bytestream.ByteStream_ReadServer) error {
	ctx := stream.Context()
	ctx, span := otelx.Start(ctx, byteStreamTracer, "ByteStreamService.Read")
	defer span.End()
	info, err := casJWT.InfoFromAuth(ctx)
	if err != nil {
		return err
	}

	s.log.Infow("msg", "download initialized", "digest", req.ResourceName)

	// Only downloader tokens are allowed
	if err := info.CheckRole(casJWT.Downloader); err != nil {
		return kerrors.Unauthorized("service", err.Error())
	}

	if req.ResourceName == "" {
		return kerrors.BadRequest("resource name", "empty resource name")
	}

	backend, err := s.loadBackend(ctx, info.BackendType, info.StoredSecretID)
	if err != nil && kerrors.IsNotFound(err) {
		return err
	} else if err != nil {
		return sl.LogAndMaskErr(err, s.log)
	}

	// streamwriter will stream chunks of data to the client
	sw := &streamWriter{stream: stream, log: s.log, wantChecksum: req.ResourceName, gotChecksum: sha256.New()}
	if err := backend.Download(ctx, sw, req.ResourceName); err != nil {
		if isClientDisconnect(err) {
			s.log.Infow("msg", "download canceled", "digest", req.ResourceName)
			return nil
		}

		return sl.LogAndMaskErr(err, s.log)
	}

	// check if the file has been tampered with and notify the client
	if sw.GetChecksum() != req.ResourceName {
		return kerrors.Unauthorized("checksum", fmt.Sprintf("checksum mismatch: got=%s, want=%s", sw.GetChecksum(), req.ResourceName))
	}

	s.log.Infow("msg", "download finished", "digest", req.ResourceName)
	s.audit.Dispatch(&events.CASArtifactDownloaded{
		CASArtifactBase: &events.CASArtifactBase{
			Digest:      req.ResourceName,
			SizeBytes:   sw.size,
			BackendType: info.BackendType,
		},
	}, info)

	return nil
}

// Store the data received from the stream in a buffer and send a signal when finished
// This is done in a separate goroutine to avoid blocking the stream
func bufferStream(ctx context.Context, stream bytestream.ByteStream_WriteServer, buffer *streamReader, log *log.Helper) {
	// Send termination signal when finished receiving data
	var bufferErr error
	defer func() {
		buffer.errorChan <- bufferErr
	}()

	for {
		select {
		case <-ctx.Done():
			// DeadlineExceeded, or Canceled
			bufferErr = ctx.Err()
			return
		default:
			// Extract the next chunk of data from the stream request
			req, err := getWriteRequest(stream)
			if err != nil {
				// If we have finished reading the stream we don't consider it a real error
				if !errors.Is(err, io.EOF) {
					bufferErr = err
				}
				return
			}

			// Write the data first: a spec-compliant client may set
			// finish_write=true on the same message that carries the final chunk,
			// so the data must be buffered before the finish check or it is lost.
			if err = buffer.Write(req.GetData()); err != nil {
				bufferErr = err
				return
			}

			log.Debugw("msg", "upload chunk received", "digest", req.resource.Digest, "currentSize", buffer.size, "maxSize", buffer.maxSize, "chunkSize", len(req.GetData()))

			// Check if the client has finished sending data
			if req.GetFinishWrite() {
				return
			}
		}
	}
}

type streamReader struct {
	*bytes.Buffer
	// total size of the in-memory buffer in bytes
	size int64
	// Max size allowed to be uploaded
	maxSize int64
	// there was an error during stream data filling
	errorChan chan error
}

// Wrapper around a buffer that adds
// the ability to record the total size of the data that went through it
// and a channel to be used by the clients to signal when the buffer has been filled
func newStreamReader(maxSize int64) *streamReader {
	return &streamReader{
		Buffer:    bytes.NewBuffer(nil),
		errorChan: make(chan error),
		maxSize:   maxSize,
	}
}

func (r *streamReader) Write(data []byte) error {
	r.size += int64(len(data))

	if err := checkUploadSize(r.size, r.maxSize); err != nil {
		return err
	}

	_, err := r.Buffer.Write(data)
	return err
}

// checkUploadSize returns an ErrUploadSizeExceeded when total exceeds maxSize.
// maxSize == 0 means no limit. It is shared by the buffered (streamReader) and
// streaming (feedPipe) paths so their cap semantics cannot drift.
func checkUploadSize(total, maxSize int64) error {
	if maxSize != 0 && total > maxSize {
		return backend.NewErrUploadSizeExceeded(total, maxSize)
	}
	return nil
}

type writeRequest struct {
	*bytestream.WriteRequest
	resource *v1.CASResource
}

// getWriteRequest returns the next write request from the stream
func getWriteRequest(stream bytestream.ByteStream_WriteServer) (*writeRequest, error) {
	req, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	resource, err := decodeResource(req.ResourceName)
	if err != nil {
		return nil, errors.New("resourceName must be set")
	}

	return &writeRequest{WriteRequest: req, resource: resource}, nil
}

// Extract the original filename and the digest from the resource string
// it comes in the form of base64(gob(resource))
func decodeResource(b64encoded string) (*v1.CASResource, error) {
	raw, err := base64.StdEncoding.DecodeString(b64encoded)
	if err != nil {
		return nil, err
	}

	resource := &v1.CASResource{}
	reader := bytes.NewReader(raw)
	dec := gob.NewDecoder(reader)
	if err := dec.Decode(resource); err != nil {
		return nil, err
	}

	return resource, err
}

// io.Writer wrapper for bytestreams.ReadResponses
type streamWriter struct {
	stream bytestream.ByteStream_ReadServer
	log    *log.Helper
	// expected wantChecksum of the data being sent
	wantChecksum string
	// calculated gotChecksum of the data sent
	gotChecksum hash.Hash
	// total number of bytes sent
	size int64
}

// Send the chunk of data through the bytestream
func (sw *streamWriter) Write(data []byte) (int, error) {
	sw.log.Debugw("msg", "sending download chunk", "digest", sw.wantChecksum, "chunkSize", len(data))

	// Update the checksum of the data being sent
	if _, err := sw.gotChecksum.Write(data); err != nil {
		return 0, err
	}

	sw.size += int64(len(data))
	return len(data), sw.stream.Send(&bytestream.ReadResponse{Data: data})
}

// GetChecksum retrieves the sha256 checksum of the read contents
func (sw *streamWriter) GetChecksum() string {
	return hex.EncodeToString(sw.gotChecksum.Sum(nil))
}
