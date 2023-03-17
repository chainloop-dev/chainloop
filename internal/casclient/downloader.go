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
	"context"
	"errors"
	"fmt"
	"io"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"google.golang.org/genproto/googleapis/bytestream"
)

// Download downloads a file from the CAS and writes it to the provided writer
func (c *Client) Download(ctx context.Context, w io.Writer, digest string) error {
	if digest == "" {
		return errors.New("a digest is required")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Open the stream to start reading chunks
	reader, err := bytestream.NewByteStreamClient(c.conn).Read(ctx, &bytestream.ReadRequest{ResourceName: digest})
	if err != nil {
		return fmt.Errorf("creating the gRPC client: %w", err)
	}

	var totalDownloaded int64
	var latestStatus *UpDownStatus

	for {
		// Get a chunk
		res, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return err
		}

		// Write the chunk to the writer and send its status
		n, err := w.Write(res.GetData())
		if err != nil {
			return err
		}

		totalDownloaded += int64(n)

		latestStatus = &UpDownStatus{ProcessedBytes: totalDownloaded}

		select {
		case c.ProgressStatus <- latestStatus:
			// message sent
		default:
			c.logger.Debug().Msg("nobody listening to progress updates, dropping message")
		}
	}

	return nil
}

// Describe returns the metadata of a resource by its digest
// We use this to get the filename and the total size of the artifact
func (c *Client) Describe(ctx context.Context, digest string) (*ResourceInfo, error) {
	client := v1.NewResourceServiceClient(c.conn)
	resp, err := client.Describe(ctx, &v1.ResourceServiceDescribeRequest{Digest: digest})
	if err != nil {
		return nil, fmt.Errorf("contacting API to get resource Info: %w", err)
	}

	return &ResourceInfo{
		Digest: resp.GetResult().GetDigest(), Filename: resp.Result.GetFileName(), Size: resp.Result.GetSize(),
	}, nil
}
