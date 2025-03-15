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

package casclient

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

	"code.cloudfoundry.org/bytefmt"
	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"google.golang.org/genproto/googleapis/bytestream"

	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
)

const defaultUploadChunkSize = 1048576 // 1MB

// Uploads a given file to a CAS server
func (c *Client) UploadFile(ctx context.Context, filepath string) (*UpDownStatus, error) {
	// open file and calculate digest
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("can't open file to upload: %w", err)
	}
	defer f.Close()

	hash, _, err := cr_v1.SHA256(f)
	if err != nil {
		return nil, fmt.Errorf("generating digest: %w", err)
	}

	// Since we have already iterated on the file to calculate the digest
	// we need to rewind the file pointer
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("rewinding file pointer: %w", err)
	}

	return c.Upload(ctx, f, path.Base(filepath), hash.String())
}

func (c *Client) Upload(ctx context.Context, r io.Reader, filename, digest string) (*UpDownStatus, error) {
	// Check digest format, including the algorithm and the hex portion
	h, err := cr_v1.NewHash(digest)
	if err != nil {
		return nil, fmt.Errorf("decoding digest: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resource, err := encodeResource(filename, h.String())
	if err != nil {
		return nil, fmt.Errorf("encoding resource name: %w", err)
	}

	c.logger.Info().Msgf("uploading %s - %s", filename, h)

	stream, err := bytestream.NewByteStreamClient(c.conn).Write(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating the gRPC client: %w", err)
	}

	buf := make([]byte, defaultUploadChunkSize)

	c.logger.Debug().
		Str("chunks", bytefmt.ByteSize(uint64(defaultUploadChunkSize))).
		Msg("uploading")

	var totalUploaded int64
	latestStatus := &UpDownStatus{
		Filename: filename,
		Digest:   h.String(),
	}

doUpload:
	for {
		n, err := r.Read(buf)
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

		latestStatus.ProcessedBytes = totalUploaded

		select {
		case c.ProgressStatus <- latestStatus:
			// message sent
		default:
			c.logger.Debug().Msg("nobody listening to progress updates, dropping message")
		}

		c.logger.Debug().
			Str("current", bytefmt.ByteSize(uint64(totalUploaded))).
			Msg("uploaded")
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		return nil, err
	}

	return latestStatus, nil
}

// encodeResource returns a base64-encoded v1.UploadResource which wraps both the digest and fileName
func encodeResource(fileName, digest string) (string, error) {
	if fileName == "" {
		return "", fmt.Errorf("file name is empty")
	}

	// Check digest format, including the algorithm and the hex portion
	h, err := cr_v1.NewHash(digest)
	if err != nil {
		return "", fmt.Errorf("decoding digest: %w", err)
	}

	var encodedResource bytes.Buffer
	enc := gob.NewEncoder(&encodedResource)
	// Currently we only support SHA256
	r := &v1.CASResource{FileName: fileName, Digest: h.Hex}

	if err := enc.Encode(r); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encodedResource.Bytes()), nil
}
