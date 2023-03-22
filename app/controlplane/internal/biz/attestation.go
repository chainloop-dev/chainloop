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
	"encoding/json"
	"fmt"
	"io"

	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"

	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type Attestation struct {
	Envelope *dsse.Envelope
}

type AttestationUseCase struct {
	logger *log.Helper
	CASClient
}

type AttestationRef struct {
	// Sha256 is the digest of the attestation and used as reference for the CAS
	Sha256 string
	// Unique identifier of the secret containing the credentials to access the CAS
	SecretRef string
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

func (uc *AttestationUseCase) FetchFromStore(ctx context.Context, secretID string, digest *cr_v1.Hash) (*Attestation, error) {
	uc.logger.Infow("msg", "downloading attestation", "digest", digest)
	buf := bytes.NewBuffer(nil)

	if err := uc.CASClient.Download(ctx, secretID, buf, digest.String()); err != nil {
		return nil, fmt.Errorf("downloading from CAS: %w", err)
	}

	var envelope dsse.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		return nil, err
	}

	return &Attestation{Envelope: &envelope}, nil
}

func (uc *AttestationUseCase) UploadToCAS(ctx context.Context, envelope *dsse.Envelope, secretID, workflowRunID string) (*cr_v1.Hash, error) {
	filename := fmt.Sprintf("attestation-%s.json", workflowRunID)
	jsonContent, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshaling the envelope: %w", err)
	}

	h, _, err := cr_v1.SHA256(bytes.NewBuffer(jsonContent))
	if err != nil {
		return nil, fmt.Errorf("calculating the digest: %w", err)
	}

	if err := uc.CASClient.Upload(ctx, secretID, bytes.NewBuffer(jsonContent), filename, h.String()); err != nil {
		return nil, fmt.Errorf("uploading to CAS: %w", err)
	}

	return &h, nil
}
