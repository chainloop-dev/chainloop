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

package biz

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
)

type AttestationUseCase struct {
	logger *log.Helper
	CASClient
}

func NewAttestationUseCase(client CASClient, logger log.Logger) *AttestationUseCase {
	if logger == nil {
		logger = log.NewStdLogger(io.Discard)
	}

	return &AttestationUseCase{
		logger:    servicelogger.ScopedHelper(logger, "biz/attestation"),
		CASClient: client,
	}
}

func (uc *AttestationUseCase) UploadEnvelopeToCAS(ctx context.Context, envelope *dsse.Envelope, backend *CASBackend, workflowRunID string) (string, error) {
	jsonContent, h, err := attestation.JSONEnvelopeWithDigest(envelope)
	if err != nil {
		return "", fmt.Errorf("marshaling the envelope: %w", err)
	}

	if err = uc.doUploadToCAS(ctx, fmt.Sprintf("attestation-%s.json", workflowRunID), jsonContent, backend, h.String()); err != nil {
		return "", fmt.Errorf("uploading to CAS: %w", err)
	}

	return h.String(), nil
}

func (uc *AttestationUseCase) UploadBundleToCAS(ctx context.Context, bundle *protobundle.Bundle, backend *CASBackend, workflowRunID string) (string, error) {
	jsonContent, h, err := attestation.JSONBundleWithDigest(bundle)
	if err != nil {
		return "", fmt.Errorf("marshaling the bundle: %w", err)
	}

	if err = uc.doUploadToCAS(ctx, fmt.Sprintf("attestation-bundle-%s.json", workflowRunID), jsonContent, backend, h.String()); err != nil {
		return "", fmt.Errorf("uploading to CAS: %w", err)
	}

	return h.String(), nil
}

func (uc *AttestationUseCase) doUploadToCAS(ctx context.Context, filename string, content []byte, backend *CASBackend, digest string) error {
	if err := uc.CASClient.Upload(ctx, string(backend.Provider), backend.SecretName, bytes.NewBuffer(content), filename, digest); err != nil {
		return err
	}

	return nil
}
