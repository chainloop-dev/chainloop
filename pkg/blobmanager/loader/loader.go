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
	"github.com/go-kratos/kratos/v2/log"

	backends "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/azureblob"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/s3"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/s3accesspoint"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
)

// Options gathers the optional, deployment-level config blocks that some
// providers need at startup. New providers should add a nilable field
// here, keeping the zero value equivalent to "don't register this
// provider". Passed by pointer so wire can supply it as a normal value.
type Options struct {
	// S3AccessPoint enables the AWS-S3-ACCESS-POINT provider. Nil = off.
	S3AccessPoint *s3accesspoint.Config
	// Logger is used to surface non-fatal provider-init warnings.
	// Optional; loader logs to the default kratos logger when nil.
	Logger log.Logger
}

// LoadProviders builds the registry of CAS backend providers consumed by
// both the controlplane and the artifact-cas binaries. The three always-on
// providers (oci, azureblob, s3) are registered unconditionally; the
// access-point provider is only registered when Options.S3AccessPoint is
// non-nil and validates.
//
// A failure to construct a conditional provider logs a warning and is
// otherwise ignored — this keeps a misconfigured s3accesspoint block from
// preventing the binary from starting at all.
//
// Passing a nil Options is valid and equivalent to "register only the
// unconditional providers", so existing test setups don't need to change.
func LoadProviders(creader credentials.Reader, opts *Options) backends.Providers {
	if opts == nil {
		opts = &Options{}
	}

	ociProvider := oci.NewBackendProvider(creader)
	azureBlobProvider := azureblob.NewBackendProvider(creader)
	s3Provider := s3.NewBackendProvider(creader)

	providers := backends.Providers{
		ociProvider.ID():       ociProvider,
		azureBlobProvider.ID(): azureBlobProvider,
		s3Provider.ID():        s3Provider,
	}

	if opts.S3AccessPoint != nil {
		apProvider, err := s3accesspoint.NewBackendProvider(opts.S3AccessPoint, creader)
		if err != nil {
			if opts.Logger != nil {
				log.NewHelper(opts.Logger).Warnf("s3accesspoint provider not registered: %v", err)
			}
		} else {
			providers[apProvider.ID()] = apProvider
		}
	}

	return providers
}
