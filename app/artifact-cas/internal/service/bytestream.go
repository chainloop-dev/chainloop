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
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"io"

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
// Iterate on the stream of file chunks, spill them to the staging disk, verify
// them against the declared digest, hand the verified file to the backend and
// return a response with the committedSize
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

	// Spill the upload to local disk, verify its SHA256 against the declared
	// digest, and only then hand the verified file to the backend. The canonical
	// key can therefore never hold content that does not hash to its digest, and
	// CAS memory stays bounded because the artifact lives on disk.
	committedSize, err := s.spillVerifyUpload(ctx, stream, storageBackend, req, info.MaxBytes)

	// Classify the outcome. The error may come from several distinct stages,
	// which must be treated differently:
	//   - A digest mismatch (digestMismatchError) is the client's fault: the
	//     bytes do not hash to the key they declared, so the request is invalid
	//     and no bytes were ever sent to the backend.
	//   - A backend Upload failure (backendUploadError) is always masked as an
	//     internal error. It must NOT be interpreted as a client disconnect even
	//     when it wraps a network reset/cancellation originating backend-side —
	//     doing so would falsely report success and silently drop the artifact.
	//   - A stream-read (spill) error is classified: a client disconnect is not a
	//     failure, an exceeded size cap maps to ResourceExhausted, anything else
	//     (e.g. a staging-disk write failure) is masked.
	if err != nil {
		if mismatch, ok := errors.AsType[*digestMismatchError](err); ok {
			s.log.Infow("msg", "upload rejected: digest mismatch", "digest", req.resource.Digest, "name", req.resource.FileName, "got", mismatch.got)
			return status.Error(codes.InvalidArgument, err.Error())
		}
		if backendErr, ok := errors.AsType[*backendUploadError](err); ok {
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

// spillVerifyUpload stages an upload on local disk, verifies its digest, and
// stores it. The client's bytes are streamed into a temporary file and hashed in
// the same pass; the file is handed to the backend only once the hash matches the
// client-declared digest, so unverified bytes never reach the canonical key.
// Staging on disk keeps this service's memory use independent of artifact size.
//
// It returns the number of bytes committed. A digest mismatch is returned as a
// *digestMismatchError; a backend Upload failure as a *backendUploadError; spill
// errors (client disconnect, exceeded size cap, staging-disk write failure) are
// returned unwrapped for the caller to classify.
func (s *ByteStreamService) spillVerifyUpload(ctx context.Context, stream bytestream.ByteStream_WriteServer, storageBackend backend.Uploader, req *writeRequest, maxBytes int64) (int64, error) {
	f, size, err := s.stageAndVerify(stagingUpload, req.resource.Digest, func(w io.Writer) error {
		return spillStream(ctx, stream, w, req.GetData(), maxBytes, s.log, req.resource.Digest)
	})
	if err != nil {
		return 0, err
	}
	// Closing the unlinked staging file is the whole cleanup: it frees the space.
	defer s.closeStagingFile(f)

	s.log.Infow("msg", "artifact verified, uploading now to backend", "name", req.resource.FileName, "digest", req.resource.Digest, "size", size)
	// IMPORTANT: hand the *os.File to Upload unwrapped. Wrapping it (io.TeeReader,
	// io.LimitReader, a progress reader) hides io.ReaderAt/io.Seeker and silently
	// forces the object-store SDK back onto in-memory multipart buffering.
	if err := storageBackend.Upload(ctx, f, req.resource); err != nil {
		return 0, &backendUploadError{err}
	}

	return size, nil
}

// backendUploadError marks a failure returned by the storage backend's Upload,
// as opposed to an error reading the client stream. Backend failures are always
// masked as internal errors and are never interpreted as a client disconnect or
// a size-cap violation, both of which only originate on the stream-read side.
type backendUploadError struct{ err error }

func (e *backendUploadError) Error() string { return e.err.Error() }
func (e *backendUploadError) Unwrap() error { return e.err }

// spillStream forwards the artifact from the client stream into w (the staging
// file tee'd into a SHA256 hasher), enforcing the max upload size as it goes.
// firstData is the payload already read from the first request.
func spillStream(ctx context.Context, stream bytestream.ByteStream_WriteServer, w io.Writer, firstData []byte, maxSize int64, log *log.Helper, digest string) error {
	// running total, needed to enforce the cap before each write
	var size int64
	write := func(data []byte) error {
		if len(data) == 0 {
			return nil
		}
		size += int64(len(data))
		if err := checkUploadSize(size, maxSize); err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("writing to staging file: %w", err)
		}
		return nil
	}

	// Write the data from the first request.
	if err := write(firstData); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			// DeadlineExceeded, or Canceled
			return ctx.Err()
		default:
			// Extract the next chunk of data from the stream request
			req, err := getWriteRequest(stream)
			if err != nil {
				// Finished reading the stream is not a real error
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}

			// Write this request's data first: a spec-compliant client may set
			// finish_write=true on the same message that carries the final chunk,
			// so the data must be written before the finish check or it is lost.
			if err := write(req.GetData()); err != nil {
				return err
			}

			log.Debugw("msg", "upload chunk received", "digest", digest, "currentSize", size, "maxSize", maxSize, "chunkSize", len(req.GetData()))

			// Check if the client has finished sending data
			if req.GetFinishWrite() {
				return nil
			}
		}
	}
}

// Server-side streaming RPC for reading blobs, implements the bytestream interface
// NOTE: the content is not piped straight from the backend to the client. It is
// staged on disk and verified first (see stageDownload), so the first byte only
// leaves once the whole artifact is known to hash to the requested digest. The
// client therefore sees no data until the backend fetch has completed.
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

	// Stage the backend egress on local disk and verify it against the requested
	// digest BEFORE the first byte is sent: the client never receives content
	// that does not hash to the key it asked for, and CAS memory stays bounded
	// because the artifact lives on disk.
	f, size, err := s.stageDownload(ctx, backend, req.ResourceName)
	if err != nil {
		return s.downloadError(err, req.ResourceName)
	}
	defer s.closeStagingFile(f)

	if err := copyStaged(sendWriter{stream}, f); err != nil {
		return s.downloadError(err, req.ResourceName)
	}

	s.log.Infow("msg", "download finished", "digest", req.ResourceName, "size", size)
	s.audit.Dispatch(&events.CASArtifactDownloaded{
		CASArtifactBase: &events.CASArtifactBase{
			Digest:      req.ResourceName,
			SizeBytes:   size,
			BackendType: info.BackendType,
		},
	}, info)

	return nil
}

// downloadError maps a staging or streaming failure of a download to its gRPC
// status. A mismatch means the backend holds content that does not hash to its
// key, corrupt or tampered, so it is reported as DataLoss with both digests
// rather than blamed on the caller. A client that went away is not an error.
// Anything else is masked.
func (s *ByteStreamService) downloadError(err error, digest string) error {
	if _, ok := errors.AsType[*digestMismatchError](err); ok {
		return status.Error(codes.DataLoss, err.Error())
	}
	if isClientDisconnect(err) {
		s.log.Infow("msg", "download canceled", "digest", digest)
		return nil
	}

	return sl.LogAndMaskErr(err, s.log)
}

// checkUploadSize returns an ErrUploadSizeExceeded when total exceeds maxSize.
// maxSize == 0 means no limit.
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

// sendWriter adapts a bytestream Read stream to io.Writer: each Write becomes
// one ReadResponse. The content has already been verified on disk and the
// chunking is decided by the caller (copyStaged), so this is a pure adapter.
type sendWriter struct {
	stream bytestream.ByteStream_ReadServer
}

func (sw sendWriter) Write(data []byte) (int, error) {
	return len(data), sw.stream.Send(&bytestream.ReadResponse{Data: data})
}
