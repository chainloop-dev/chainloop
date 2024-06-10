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

package loader

import (
	backends "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/azureblob"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/s3"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
)

func LoadProviders(creader credentials.Reader) backends.Providers {
	// Initialize CAS backend providers
	ociProvider := oci.NewBackendProvider(creader)
	azureBlobProvider := azureblob.NewBackendProvider(creader)
	s3Provider := s3.NewBackendProvider(creader)

	return backends.Providers{
		ociProvider.ID():       ociProvider,
		azureBlobProvider.ID(): azureBlobProvider,
		s3Provider.ID():        s3Provider,
	}
}
