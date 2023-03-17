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
	"io"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

type UpDownStatus struct {
	Filepath, Filename, Digest string
	ProcessedBytes             int64
}

type ResourceInfo struct {
	Digest   string
	Filename string
	Size     int64
}

type Uploader interface {
	Upload(ctx context.Context, filepath string) (*UpDownStatus, error)
}

type Downloader interface {
	Download(ctx context.Context, w io.Writer, digest string) error
}

type DownloaderUploader interface {
	Downloader
	Uploader
}

type ProgressStatusChan chan (*UpDownStatus)
type Client struct {
	conn   *grpc.ClientConn
	logger zerolog.Logger
	// channel to send progress status to the go-routine that's rendering the progress bar
	ProgressStatus ProgressStatusChan
}

func New(conn *grpc.ClientConn, opts ...ClientOpts) *Client {
	client := &Client{
		conn:           conn,
		ProgressStatus: make(chan *UpDownStatus),
		logger:         zerolog.Nop(),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

type ClientOpts func(u *Client)

func WithLogger(l zerolog.Logger) ClientOpts {
	return func(u *Client) {
		u.logger = l
	}
}
