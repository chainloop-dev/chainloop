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

	"github.com/chainloop-dev/chainloop/internal/attestation"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"

	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
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

func (uc *AttestationUseCase) UploadToCAS(ctx context.Context, envelope *dsse.Envelope, backend *CASBackend, workflowRunID string) (*cr_v1.Hash, error) {
	filename := fmt.Sprintf("attestation-%s.json", workflowRunID)
	jsonContent, h, err := attestation.JSONEnvelopeWithDigest(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshaling the envelope: %w", err)
	}

	if err := uc.CASClient.Upload(ctx, string(backend.Provider), backend.SecretName, bytes.NewBuffer(jsonContent), filename, h.String()); err != nil {
		return nil, fmt.Errorf("uploading to CAS: %w", err)
	}

	return &h, nil
}
