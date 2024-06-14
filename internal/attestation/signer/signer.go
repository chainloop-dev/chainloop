//
// Copyright 2024 The Chainloop Authors.
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

package signer

import (
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/signer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/attestation/signer/cosign"
	"github.com/rs/zerolog"
	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
)

// GetSigner creates a new Signer based on input parameters
func GetSigner(keyPath string, logger zerolog.Logger, client pb.SigningServiceClient) sigstoresigner.Signer {
	var signer sigstoresigner.Signer
	if keyPath != "" {
		signer = cosign.NewSigner(keyPath, logger)
	} else {
		signer = chainloop.NewSigner(client, logger)
	}

	return signer
}
