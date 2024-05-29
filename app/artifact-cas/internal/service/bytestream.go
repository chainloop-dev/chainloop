//
// Copyright 2023 The Chainloop Authors.
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
	"fmt"
	"hash"
	"io"

	"errors"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	sl "github.com/chainloop-dev/chainloop/pkg/servicelogger"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

	// Get auth info and check that it's an uploader token
	info, err := infoFromAuth(ctx)
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

	backend, err := s.loadBackend(ctx, info.BackendType, info.StoredSecretID)
	if err != nil && kerrors.IsNotFound(err) {
		return err
	} else if err != nil {
		return sl.LogAndMaskErr(err, s.log)
	}

	// We check if the file already exists even before we wait for the whole buffer to be filled
	if exists, err := backend.Exists(ctx, req.resource.Digest); err != nil {
		return sl.LogAndMaskErr(err, s.log)
	} else if exists {
		s.log.Infow("msg", "artifact already exists", "digest", req.resource.Digest)
		return stream.SendAndClose(&bytestream.WriteResponse{})
	}

	s.log.Infow("msg", "artifact does not exist, uploading", "digest", req.resource.Digest, "name", req.resource.FileName)
	// Create a buffer that will be filled in the background before sending its content to the backend
	buffer := newStreamReader()
	// Add data from first request
	if err = buffer.Write(req.GetData()); err != nil {
		return sl.LogAndMaskErr(err, s.log)
	}

	// Start a goroutine that will fill the buffer in the background
	go bufferStream(ctx, stream, buffer, s.log)

	// Block until the buffer has been filled or the upload process has been canceled
	// This implementation is suboptimal since it requires the content to be uploaded in memory before pushing it
	// This is due to the fact that our OCI push implementation does not support streaming/chunks for uncompressed layers
	// We can not use stream.Layer since it only supports compressed layers, we want to store raw data and set custom mimetypes
	// https://github.com/google/go-containerregistry/blob/main/pkg/v1/stream/README.md
	// TODO: Split content in multiple layers and do concurrent uploads/downloads
	err = <-buffer.errorChan

	// Now it's time to check if the data provider has sent an error
	if err != nil {
		if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
			s.log.Infow("msg", "upload canceled", "digest", req.resource.Digest, "name", req.resource.FileName)
			return nil
		}
		return sl.LogAndMaskErr(err, s.log)
	}

	s.log.Infow("msg", "artifact received, uploading now to backend", "name", req.resource.FileName, "digest", req.resource.Digest, "size", buffer.size)
	if err := backend.Upload(ctx, buffer, req.resource); err != nil {
		return sl.LogAndMaskErr(err, s.log)
	}

	s.log.Infow("msg", "upload finished", "name", req.resource.FileName, "digest", req.resource.Digest, "size", buffer.size)
	return stream.SendAndClose(&bytestream.WriteResponse{CommittedSize: buffer.size})
}

// Server-side streaming RPC for reading blobs, implements the bytestream interface
// NOTE: Due to the fact that we are using the OCI backend, we can not stream the content directly from the backend
// but instead we need to download the whole artifact and then stream it to the client
func (s *ByteStreamService) Read(req *bytestream.ReadRequest, stream bytestream.ByteStream_ReadServer) error {
	ctx := stream.Context()
	info, err := infoFromAuth(ctx)
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
	sw := &streamWriter{stream, s.log, req.ResourceName, sha256.New()}
	if err := backend.Download(ctx, sw, req.ResourceName); err != nil {
		if errors.Is(err, context.Canceled) {
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

			// Check if the client has finished sending data
			if req.GetFinishWrite() {
				return
			}

			// Write the data to the buffer
			if err = buffer.Write(req.GetData()); err != nil {
				bufferErr = err
				return
			}

			log.Debugw("msg", "upload chunk received", "digest", req.resource.Digest, "bufferSize", buffer.size, "chunkSize", len(req.GetData()))
		}
	}
}

type streamReader struct {
	*bytes.Buffer
	// total size of the in-memory buffer in bytes
	size int64
	// there was an error during stream data filling
	errorChan chan error
}

// Wrapper around a buffer that adds
// the ability to record the total size of the data that went through it
// and a channel to be used by the clients to signal when the buffer has been filled
func newStreamReader() *streamReader {
	return &streamReader{
		Buffer:    bytes.NewBuffer(nil),
		errorChan: make(chan error),
	}
}

func (r *streamReader) Write(data []byte) error {
	r.size += int64(len(data))
	_, err := r.Buffer.Write(data)
	return err
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
}

// Send the chunk of data through the bytestream
func (sw *streamWriter) Write(data []byte) (int, error) {
	sw.log.Debugw("msg", "sending download chunk", "digest", sw.wantChecksum, "chunkSize", len(data))

	// Update the checksum of the data being sent
	if _, err := sw.gotChecksum.Write(data); err != nil {
		return 0, err
	}
	return len(data), sw.stream.Send(&bytestream.ReadResponse{Data: data})
}

// GetChecksum retrieves the sha256 checksum of the read contents
func (sw *streamWriter) GetChecksum() string {
	return fmt.Sprintf("%x", sw.gotChecksum.Sum(nil))
}
