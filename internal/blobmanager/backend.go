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

package backend

import (
	"context"
	"io"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
)

type Uploader interface {
	Upload(ctx context.Context, r io.Reader, resource *v1.CASResource) error
	Exists(ctx context.Context, digest string) (bool, error)
	CheckWritePermissions(ctx context.Context) error
}

type UploaderDownloader interface {
	Uploader
	Downloader
	Describer
}

type Describer interface {
	Describe(ctx context.Context, digest string) (*v1.CASResource, error)
}

type Downloader interface {
	Download(ctx context.Context, w io.Writer, digest string) error
}

// Provider is an interface that allows to create a backend from a secret
type Provider interface {
	FromCredentials(ctx context.Context, secretName string) (UploaderDownloader, error)
}

type Validator interface {
	ValidateAndExtractCredentials(location string, credsJSON []byte) (any, error)
}
