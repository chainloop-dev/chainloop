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

package action

import (
	"context"
	"time"

	casclient "github.com/chainloop-dev/chainloop/app/cli/internal/casclient/grpc"
	"github.com/jedib0t/go-pretty/v6/progress"
	"google.golang.org/grpc"
)

type CASArtifact struct {
	Digest, fileName string
}

type ArtifactUpload struct {
	*ActionsOpts
	artifactsCASConn *grpc.ClientConn
}

type ArtifactUploadOpts struct {
	*ActionsOpts
	ArtifactsCASConn *grpc.ClientConn
}

func NewArtifactUpload(opts *ArtifactUploadOpts) *ArtifactUpload {
	return &ArtifactUpload{opts.ActionsOpts, opts.ArtifactsCASConn}
}

func (a *ArtifactUpload) Run(filePath string) (*CASArtifact, error) {
	client := casclient.NewUploader(a.artifactsCASConn, casclient.WithLogger(a.Logger))
	// render progress bar
	go renderOperationStatus(context.Background(), client.ProgressStatus, a.Logger)
	defer close(client.ProgressStatus)

	res, err := client.Upload(context.Background(), filePath)
	if err != nil {
		return nil, err
	}

	// Give some time for the progress renderer to finish
	// TODO: Implement with proper subroutine messaging
	time.Sleep(progress.DefaultUpdateFrequency)

	return &CASArtifact{Digest: res.Digest, fileName: res.Filename}, nil
}
