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

package grpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"code.cloudfoundry.org/bytefmt"
	v1 "github.com/chainloop-dev/bedrock/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/bedrock/internal/attestation/crafter/materials"
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/rs/zerolog"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"

	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
)

type casClient struct {
	conn   *grpc.ClientConn
	logger zerolog.Logger
	// channel to send progress status to the go-routine that's rendering the progress bar
	progressStatus chan (*materials.UpDownStatus)
	// wether to render progress bar
	renderProgress bool
}
type UploaderClient struct {
	*casClient
	bufferSize int
}

type ClientOpts func(u *casClient)

func WithProgressRender(b bool) ClientOpts {
	return func(u *casClient) {
		u.renderProgress = b
	}
}

func WithLogger(l zerolog.Logger) ClientOpts {
	return func(u *casClient) {
		u.logger = l
	}
}

const defaultUploadChunkSize = 1048576 // 1MB

func NewUploader(conn *grpc.ClientConn, opts ...ClientOpts) *UploaderClient {
	client := &UploaderClient{
		casClient: &casClient{
			conn:           conn,
			progressStatus: make(chan *materials.UpDownStatus, 2), // Adding some buffer
			logger:         zerolog.New(os.Stderr),
		},
		bufferSize: defaultUploadChunkSize,
	}

	for _, opt := range opts {
		opt(client.casClient)
	}

	return client
}

// Uploads a given file to a CAS server
func (c *UploaderClient) Upload(ctx context.Context, filepath string) (*materials.UpDownStatus, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// inititate progress bar
	if c.renderProgress {
		go c.renderUploadStatus(ctx, c.logger)
		defer close(c.progressStatus)
	}

	// open file and calculate digest
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("can't open file to upload: %w", err)
	}
	defer f.Close()

	hash, _, err := cr_v1.SHA256(f)
	if err != nil {
		return nil, fmt.Errorf("genering digest: %w", err)
	}

	// Since we have already iterated on the file to calculate the digest
	// we need to rewind the file pointer
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("rewinding file pointer: %w", err)
	}

	filename := path.Base(filepath)
	resource, err := encodeResource(filename, hash.Hex)
	if err != nil {
		return nil, fmt.Errorf("encoding resource name: %w", err)
	}

	c.logger.Info().Msgf("uploading %s - sha256:%s", filepath, hash.Hex)

	stream, err := bytestream.NewByteStreamClient(c.conn).Write(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating the gRPC client: %w", err)
	}

	buf := make([]byte, c.bufferSize)

	c.logger.Debug().Str("path", filepath).Str("digest", hash.String()).Msg("file opened")

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("retrieving file information: %w", err)
	}

	c.logger.Debug().
		Str("total-size", bytefmt.ByteSize(uint64(info.Size()))).
		Str("chunks", bytefmt.ByteSize(uint64(c.bufferSize))).
		Msg("uploading")

	var totalUploaded int64
	var latestStatus *materials.UpDownStatus

doUpload:
	for {
		n, err := f.Read(buf)
		if err == io.EOF {
			c.logger.Debug().Msg("finishing upload")
			// Indicate that there is no more data to send
			if err := stream.Send(&bytestream.WriteRequest{
				ResourceName: resource,
				FinishWrite:  true,
			}); err != nil {
				return nil, fmt.Errorf("sending the finished upload message %w", err)
			}
			break
		}
		// Another error occurred while reading the io.reader
		if err != nil {
			return nil, fmt.Errorf("reading content: %w", err)
		}

		totalUploaded += int64(n)
		select {
		case <-stream.Context().Done():
			// The server might have closed the connection
			return nil, stream.Context().Err()
		default:
			// Send the data in the buffer up to the latest read position
			if err := stream.Send(&bytestream.WriteRequest{
				ResourceName: resource,
				Data:         buf[:n],
			}); err != nil {
				// If there is an error. The server might return io.EOF
				// and the error will be then exposed by running CloseAndRecv()
				// That's why we need to break the loop here
				if errors.Is(err, io.EOF) {
					break doUpload
				}

				return nil, err
			}
		}

		latestStatus = &materials.UpDownStatus{
			Filepath: filepath, Filename: filename,
			Digest: hash.String(), TotalSizeBytes: info.Size(), ProcessedBytes: totalUploaded,
		}

		if c.renderProgress {
			c.progressStatus <- latestStatus
		}

		c.logger.Debug().
			Str("total-size", bytefmt.ByteSize(uint64(info.Size()))).
			Str("current", bytefmt.ByteSize(uint64(totalUploaded))).
			Msg("uploaded")
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		return nil, err
	}

	// Give some time for the progress renderer to finish
	// TODO: Implement with proper subroutine messaging
	if c.renderProgress {
		time.Sleep(renderUpdateFrequency)
		// Block until the buffer has been filled or the upload process has been canceled
	}

	return latestStatus, nil
}

var renderUpdateFrequency = progress.DefaultUpdateFrequency

func (c *UploaderClient) renderUploadStatus(ctx context.Context, output io.Writer) {
	pw := progress.NewWriter()
	pw.Style().Visibility.ETA = true
	pw.Style().Visibility.Speed = true
	pw.SetUpdateFrequency(renderUpdateFrequency)

	var tracker *progress.Tracker
	go pw.Render()
	defer pw.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case status, ok := <-c.progressStatus:
			if !ok {
				return
			}

			if tracker == nil {
				// Hack: Add 1 to the total to make sure the tracker is not marked as done before the upload is finished
				// this way the current value will never reach the total
				// but instead the tracker will be marked as done by the defer statement
				total := status.TotalSizeBytes + 1
				tracker = &progress.Tracker{Total: total, Units: progress.UnitsBytes}
				defer tracker.MarkAsDone()
				pw.AppendTracker(tracker)
			}

			tracker.SetValue(status.ProcessedBytes)
		}
	}
}

// encodedResource returns a base64-encoded v1.UploadResource which wraps both the digest and fileName
func encodeResource(fileName, digest string) (string, error) {
	if fileName == "" {
		return "", fmt.Errorf("file name is empty")
	}

	if digest == "" {
		return "", fmt.Errorf("digest is empty")
	}

	var encodedResource bytes.Buffer
	enc := gob.NewEncoder(&encodedResource)
	r := &v1.CASResource{FileName: fileName, Digest: digest}

	if err := enc.Encode(r); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encodedResource.Bytes()), nil
}
