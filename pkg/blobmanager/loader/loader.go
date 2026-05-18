//
// Copyright 2023-2026 The Chainloop Authors.
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
	backends "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/azureblob"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/s3"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/s3accesspoint"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
)

// LoadProviders builds the registry of CAS backend providers consumed by
// both the controlplane and the artifact-cas binaries. All providers are
// registered unconditionally — the s3accesspoint provider has no
// deployment-level config of its own (everything per-tenant lives in the
// secret blob), so on-prem deployments without managed CAS simply never
// have managed rows and the provider is dormant.
func LoadProviders(creader credentials.Reader) backends.Providers {
	ociProvider := oci.NewBackendProvider(creader)
	azureBlobProvider := azureblob.NewBackendProvider(creader)
	s3Provider := s3.NewBackendProvider(creader)
	apProvider := s3accesspoint.NewBackendProvider(creader)

	return backends.Providers{
		ociProvider.ID():       ociProvider,
		azureBlobProvider.ID(): azureBlobProvider,
		s3Provider.ID():        s3Provider,
		apProvider.ID():        apProvider,
	}
}
