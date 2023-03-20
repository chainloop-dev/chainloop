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

	casAPI "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"

	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type Attestation struct {
	Envelope *dsse.Envelope
}

type AttestationUseCase struct {
	logger *log.Helper
	CASUploader

	// DEPRECATED
	// We will remove it once we force all the clients to use the CAS instead
	backendProvider backend.Provider
}

type AttestationRef struct {
	// Sha256 is the digest of the attestation and used as reference for the CAS
	Sha256 string
	// Unique identifier of the secret containing the credentials to access the CAS
	SecretRef string
}

func NewAttestationUseCase(uploader CASUploader, p backend.Provider, logger log.Logger) *AttestationUseCase {
	if logger == nil {
		logger = log.NewStdLogger(io.Discard)
	}

	return &AttestationUseCase{
		logger:          servicelogger.ScopedHelper(logger, "biz/attestation"),
		CASUploader:     uploader,
		backendProvider: p,
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

func (uc *AttestationUseCase) UploadToCAS(ctx context.Context, envelope *dsse.Envelope, secretID, workflowRunID string) (string, error) {
	filename := fmt.Sprintf("attestation-%s.json", workflowRunID)
	jsonContent, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("marshaling the envelope: %w", err)
	}

	hash := sha256.New()
	hash.Write(jsonContent)
	digest := fmt.Sprintf("%x", hash.Sum(nil))

	if uc.CASUploader.Configured() {
		if err := uc.CASUploader.Upload(ctx, secretID, bytes.NewBuffer(jsonContent), filename, digest); err != nil {
			return "", fmt.Errorf("uploading to CAS: %w", err)
		}

		return digest, nil
	}

	uc.logger.Warnw("msg", "no CAS configured, falling back to old mechanism")

	// fallback to old mechanism, this will be removed once we force all the clients to use the CAS
	// TODO: remove
	uploader, err := uc.backendProvider.FromCredentials(ctx, secretID)
	if err != nil {
		return "", err
	}

	if err := uploader.Upload(ctx, bytes.NewBuffer(jsonContent), &casAPI.CASResource{
		FileName: filename, Digest: digest,
	}); err != nil {
		return "", fmt.Errorf("uploading to OCI: %w", err)
	}

	return digest, nil
}
