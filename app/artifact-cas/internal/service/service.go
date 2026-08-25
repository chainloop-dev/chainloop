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

package service

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewByteStreamService, NewResourceService, NewDownloadService)

type commonService struct {
	log      *log.Helper
	backends backend.Providers
	// best-effort audit events publisher, nil-safe
	audit *AuditDispatcher
	// stagingDir is the local directory where uploads are staged on disk while
	// their SHA256 is verified against the declared digest before reaching the
	// backend. It must be set and writable: leaving it empty is a deployment
	// error and uploads are refused rather than staged somewhere unintended.
	stagingDir string
}

func (s *commonService) loadBackend(ctx context.Context, providerType, secretID string) (backend.UploaderDownloader, error) {
	// get the OCI provider from the map
	p, ok := s.backends[providerType]
	if !ok || p == nil {
		return nil, kerrors.NotFound("backend provider", fmt.Sprintf("backend %q not found", providerType))
	}

	s.log.Infow("msg", "selected provider", "provider", providerType)

	// Retrieve the OCI backend from where to download the file
	backend, err := p.FromCredentials(ctx, secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve backend: %w", err)
	}

	return backend, nil
}

type NewOpt func(s *commonService)

func WithLogger(logger log.Logger) NewOpt {
	return func(s *commonService) {
		s.log = servicelogger.ScopedHelper(logger, "service")
	}
}

func WithAuditDispatcher(d *AuditDispatcher) NewOpt {
	return func(s *commonService) {
		s.audit = d
	}
}

// WithStagingDir sets the local directory where uploads are spilled and
// verified before being sent to the backend. It must point at a writable volume
// dedicated to this pod; there is no default, so an upload fails loudly rather
// than silently staging somewhere unintended.
func WithStagingDir(dir string) NewOpt {
	return func(s *commonService) {
		s.stagingDir = dir
	}
}

func newCommonService(backends backend.Providers, opts ...NewOpt) *commonService {
	s := &commonService{
		log:      servicelogger.EmptyLogger(),
		backends: backends,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// isClientDisconnect returns true if the error indicates the client has disconnected.
// This includes context cancellation, gRPC canceled status, and network-level
// errors such as "connection reset by peer" and "broken pipe".
func isClientDisconnect(err error) bool {
	if err == nil {
		return false
	}

	// Context cancellation (e.g. client canceled the request)
	if errors.Is(err, context.Canceled) {
		return true
	}

	// gRPC canceled status (client disconnect in gRPC streaming)
	if status.Code(err) == codes.Canceled {
		return true
	}

	// Network-level disconnects: connection reset by peer, broken pipe
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}

	return false
}
