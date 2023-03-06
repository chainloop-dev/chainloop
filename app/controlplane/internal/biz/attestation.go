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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	casAPI "github.com/chainloop-dev/bedrock/app/artifact-cas/api/cas/v1"

	backend "github.com/chainloop-dev/bedrock/internal/blobmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type Attestation struct {
	Envelope *dsse.Envelope
}

type AttestationUseCase struct {
	logger *log.Helper
}

type AttestationRef struct {
	// Sha256 is the digest of the attestation and used as reference for the CAS
	Sha256 string
	// Unique identifier of the secret containing the credentials to access the CAS
	SecretRef string
}

func NewAttestationUseCase(logger log.Logger) *AttestationUseCase {
	if logger == nil {
		logger = log.NewStdLogger(io.Discard)
	}

	return &AttestationUseCase{
		logger: log.NewHelper(logger),
	}
}

func (uc *AttestationUseCase) FetchFromStore(ctx context.Context, downloader backend.Downloader, digest string) (*Attestation, error) {
	uc.logger.Infow("msg", "downloading attestation", "digest", digest)
	buf := bytes.NewBuffer(nil)

	if err := downloader.Download(ctx, buf, digest); err != nil {
		return nil, err
	}

	var envelope dsse.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		return nil, err
	}

	return &Attestation{Envelope: &envelope}, nil
}

// UploadAttestationToOCI uploads the attestation to the OCI CAS returning the reference to the attestation
func (uc *AttestationUseCase) UploadAttestationToOCI(ctx context.Context, envelope *dsse.Envelope, uploader backend.Uploader, workflowRunID string) (string, error) {
	digest, err := doUploadToOCI(ctx, uploader, workflowRunID, envelope, uc.logger)
	if err != nil {
		return "", err
	}

	return digest, nil
}

func doUploadToOCI(ctx context.Context, backend backend.Uploader, runID string, envelope *dsse.Envelope, logger *log.Helper) (string, error) {
	fileName := fmt.Sprintf("attestation-%s.json", runID)
	jsonContent, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("marshalling the envelope: %w", err)
	}

	hash := sha256.New()
	hash.Write(jsonContent)
	digest := fmt.Sprintf("%x", hash.Sum(nil))

	if err := backend.Upload(ctx, bytes.NewBuffer(jsonContent), &casAPI.CASResource{
		FileName: fileName, Digest: digest,
	}); err != nil {
		return "", fmt.Errorf("uploading to OCI: %w", err)
	}

	logger.Infow("msg", "attestation uploaded to OCI", "digest", digest, "filename", fileName, "runID", runID)

	return digest, nil
}
