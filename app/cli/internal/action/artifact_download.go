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
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/jedib0t/go-pretty/v6/progress"
	"google.golang.org/grpc"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
)

type ArtifactDownload struct {
	*ActionsOpts
	artifactsCASConn *grpc.ClientConn
}

type ArtifactDownloadOpts struct {
	*ActionsOpts
	ArtifactsCASConn *grpc.ClientConn
}

func NewArtifactDownload(opts *ArtifactDownloadOpts) *ArtifactDownload {
	return &ArtifactDownload{opts.ActionsOpts, opts.ArtifactsCASConn}
}

func (a *ArtifactDownload) Run(downloadPath, digest string) error {
	h, err := crv1.NewHash(digest)
	if err != nil {
		return fmt.Errorf("invalid digest: %w", err)
	}

	client := casclient.New(a.artifactsCASConn)
	ctx := context.Background()
	info, err := client.Describe(ctx, h.Hex)
	if err != nil {
		return fmt.Errorf("resource with digest %s not found", h)
	}

	if downloadPath == "" {
		var err error
		downloadPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current dir: %w", err)
		}
	}

	downloadPath = path.Join(downloadPath, info.Filename)
	f, err := os.Create(downloadPath)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer f.Close()

	// Calculate the checksum as we write it to a file
	hash := sha256.New()
	w := io.MultiWriter(f, hash)

	a.Logger.Info().Str("name", info.Filename).Str("to", downloadPath).Msg("downloading file")

	// render progress bar
	go renderOperationStatus(ctx, client.ProgressStatus, info.Size)
	defer close(client.ProgressStatus)

	err = client.Download(ctx, w, h.Hex)
	if err != nil {
		a.Logger.Debug().Err(err).Msg("problem downloading file")
		return errors.New("problem downloading file")
	}

	if got, want := fmt.Sprintf("%x", hash.Sum(nil)), h.Hex; got != want {
		return fmt.Errorf("checksums mismatch: got: %s, expected: %s", got, want)
	}

	// Give some time for the progress renderer to finish
	// TODO: Implement with proper subroutine messaging
	time.Sleep(progress.DefaultUpdateFrequency)

	a.Logger.Info().Str("path", downloadPath).Msg("file downloaded!")

	return nil
}

func renderOperationStatus(ctx context.Context, progressChan casclient.ProgressStatusChan, totalSize int64) {
	pw := progress.NewWriter()
	pw.Style().Visibility.ETA = true
	pw.Style().Visibility.Speed = true
	pw.SetUpdateFrequency(progress.DefaultUpdateFrequency)

	var tracker *progress.Tracker
	go pw.Render()
	defer pw.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case status, ok := <-progressChan:
			if !ok {
				return
			}

			// Initialize tracker
			if tracker == nil {
				// Hack: Add 1 to the total to make sure the tracker is not marked as done before the upload is finished
				// this way the current value will never reach the total
				// but instead the tracker will be marked as done by the defer statement
				tracker = &progress.Tracker{Total: totalSize + 1, Units: progress.UnitsBytes}
				defer tracker.MarkAsDone()
				pw.AppendTracker(tracker)
			}

			tracker.SetValue(status.ProcessedBytes)
		}
	}
}
