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
	"os"

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
		var mismatch *digestMismatchError
		if errors.As(err, &mismatch) {
			s.log.Infow("msg", "upload rejected: digest mismatch", "digest", req.resource.Digest, "name", req.resource.FileName, "got", mismatch.got)
			return status.Error(codes.InvalidArgument, err.Error())
		}
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

// spillVerifyUpload streams the client's upload to a temporary file on the local
// staging disk while computing its SHA256, verifies the computed digest against
// the client-declared one, and only then hands the verified file to the backend
// for storage under the canonical key. Unverified bytes never reach the backend,
// so the canonical key can never hold content that does not hash to its
// digest. CAS memory stays bounded because the artifact lives on disk, and
// the *os.File handed to Upload lets object-store SDKs stream it in bounded parts
// via io.ReaderAt/io.Seeker rather than buffering it in memory.
//
// It returns the number of bytes committed. A digest mismatch is returned as a
// *digestMismatchError; a backend Upload failure as a *backendUploadError; spill
// errors (client disconnect, exceeded size cap, staging-disk write failure) are
// returned unwrapped for the caller to classify.
func (s *ByteStreamService) spillVerifyUpload(ctx context.Context, stream bytestream.ByteStream_WriteServer, storageBackend backend.Uploader, req *writeRequest, maxBytes int64) (int64, error) {
	f, err := os.CreateTemp(s.stagingDir, stagingFilePrefix+"*")
	if err != nil {
		return 0, fmt.Errorf("creating staging file: %w", err)
	}
	// Clean up the staging file on every exit path: nothing is left on disk
	// whether the upload is rejected, fails, or succeeds. Close before remove so
	// the handle is released; on Linux either order unlinks the file regardless.
	defer func() {
		_ = f.Close()
		if err := os.Remove(f.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warnw("msg", "failed to remove staging file", "path", f.Name(), "error", err.Error())
		}
	}()

	// Tee the stream into the file and a SHA256 hasher in one pass.
	hasher := sha256.New()
	size, err := spillStream(ctx, stream, io.MultiWriter(f, hasher), req.GetData(), maxBytes, s.log, req.resource.Digest)
	if err != nil {
		return 0, err
	}

	// Fail closed: if the streamed bytes do not hash to the declared digest,
	// reject the upload and send nothing to the backend.
	if got := hex.EncodeToString(hasher.Sum(nil)); got != req.resource.Digest {
		return 0, &digestMismatchError{got: got, want: req.resource.Digest}
	}

	// Rewind so the backend reads from the start. A seekable body also lets the
	// AWS SDK learn the exact length and take its zero-copy SectionReader fast
	// path instead of buffering parts in memory.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("rewinding staging file: %w", err)
	}

	s.log.Infow("msg", "artifact verified, uploading now to backend", "name", req.resource.FileName, "digest", req.resource.Digest, "size", size)
	// IMPORTANT: hand the *os.File to Upload unwrapped. Wrapping it (io.TeeReader,
	// io.LimitReader, a progress reader) hides io.ReaderAt/io.Seeker and silently
	// forces the object-store SDK back onto in-memory multipart buffering.
	if err := storageBackend.Upload(ctx, f, req.resource); err != nil {
		return 0, &backendUploadError{err}
	}

	return size, nil
}

// stagingFilePrefix names the temporary upload files so the boot-time sweep can
// distinguish CAS's own leftovers from anything else that might share the dir.
const stagingFilePrefix = "cas-upload-"

// backendUploadError marks a failure returned by the storage backend's Upload,
// as opposed to an error reading the client stream. Backend failures are always
// masked as internal errors and are never interpreted as a client disconnect or
// a size-cap violation, both of which only originate on the stream-read side.
type backendUploadError struct{ err error }

func (e *backendUploadError) Error() string { return e.err.Error() }
func (e *backendUploadError) Unwrap() error { return e.err }

// digestMismatchError marks an upload whose streamed bytes do not hash to the
// client-declared digest. It is surfaced to the client as InvalidArgument: the
// request is malformed (the declared key does not describe the content), and no
// bytes are ever written to the backend.
type digestMismatchError struct{ got, want string }

func (e *digestMismatchError) Error() string {
	return fmt.Sprintf("uploaded content does not match the declared digest: got=%s, want=%s", e.got, e.want)
}

// spillStream forwards the artifact from the client stream into w (the staging
// file tee'd into a SHA256 hasher), enforcing the max upload size as it goes.
// firstData is the payload already read from the first request. It returns the
// total number of bytes written. It reads the client stream straight into w.
func spillStream(ctx context.Context, stream bytestream.ByteStream_WriteServer, w io.Writer, firstData []byte, maxSize int64, log *log.Helper, digest string) (int64, error) {
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

			// Write this request's data first: a spec-compliant client may set
			// finish_write=true on the same message that carries the final chunk,
			// so the data must be written before the finish check or it is lost.
			if err := write(req.GetData()); err != nil {
				return size, err
			}

			log.Debugw("msg", "upload chunk received", "digest", digest, "currentSize", size, "maxSize", maxSize, "chunkSize", len(req.GetData()))

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
