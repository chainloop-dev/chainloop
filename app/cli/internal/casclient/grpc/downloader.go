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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	v1 "github.com/chainloop-dev/bedrock/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/bedrock/internal/attestation/crafter/materials"
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/rs/zerolog"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
)

type DownloaderClient struct {
	*casClient
}

func NewDownloader(conn *grpc.ClientConn, opts ...ClientOpts) *DownloaderClient {
	client := &DownloaderClient{
		casClient: &casClient{
			conn:           conn,
			logger:         zerolog.New(os.Stderr),
			progressStatus: make(chan *materials.UpDownStatus, 2),
		},
	}

	for _, opt := range opts {
		opt(client.casClient)
	}

	return client
}

// Download downloads a file from the CAS and writes it to the provided writer
// It also receives a totalBytes parameter to render the progress bar
func (c *DownloaderClient) Download(ctx context.Context, w io.Writer, digest string, totalBytes int64) error {
	if digest == "" {
		return errors.New("a digest is required")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if c.renderProgress {
		go c.renderDownloadStatus(ctx, c.logger)
		defer close(c.progressStatus)
	}

	// Open the stream to start reading chunks
	reader, err := bytestream.NewByteStreamClient(c.conn).Read(ctx, &bytestream.ReadRequest{ResourceName: digest})
	if err != nil {
		return fmt.Errorf("creating the gRPC client: %w", err)
	}

	var totalDownloaded int64
	var latestStatus *materials.UpDownStatus

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

		latestStatus = &materials.UpDownStatus{
			TotalSizeBytes: totalBytes, ProcessedBytes: totalDownloaded,
		}

		if c.renderProgress {
			c.progressStatus <- latestStatus
		}
	}

	// Give some time for the progress renderer to finish
	// TODO: Implement with proper subroutine messaging
	if c.renderProgress {
		time.Sleep(renderUpdateFrequency)
		// Block until the buffer has been filled or the upload process has been canceled
	}

	return nil
}

// Describe returns the metadata of a resource by its digest
// We use this to get the filename and the total size of the artifacct
func (c *DownloaderClient) Describe(ctx context.Context, digest string) (*materials.ResourceInfo, error) {
	client := v1.NewResourceServiceClient(c.conn)
	resp, err := client.Describe(ctx, &v1.ResourceServiceDescribeRequest{Digest: digest})
	if err != nil {
		return nil, fmt.Errorf("contacting API to get resource Info: %w", err)
	}

	return &materials.ResourceInfo{
		Digest: resp.GetResult().GetDigest(), Filename: resp.Result.GetFileName(), Size: resp.Result.GetSize(),
	}, nil
}

func (c *DownloaderClient) renderDownloadStatus(ctx context.Context, output io.Writer) {
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
		case s, ok := <-c.progressStatus:
			if !ok {
				return
			}

			// Initialize tracker
			if tracker == nil {
				total := s.TotalSizeBytes
				tracker = &progress.Tracker{
					Total: total,
					Units: progress.UnitsBytes,
				}
				defer tracker.MarkAsDone()
				pw.AppendTracker(tracker)
			}

			tracker.SetValue(s.ProcessedBytes)
		}
	}
}
