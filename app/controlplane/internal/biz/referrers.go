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
	"encoding/json"
	"fmt"
	"io"

	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type Referrer struct {
	digest       string
	artifactType string
	// points to other digests
	references []string
}

type ReferrerMap map[string]*Referrer

type ReferrerRepo interface{}

type ReferrerUseCase struct {
	repo   ReferrerRepo
	logger *log.Helper
}

func NewReferrerUseCase(repo ReferrerRepo, l log.Logger) *ReferrerUseCase {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	return &ReferrerUseCase{repo, servicelogger.ScopedHelper(l, "biz/Referrer")}
}

const ReferrerAttestationType = "ATTESTATION"

func extractReferrers(att *dsse.Envelope) (ReferrerMap, error) {
	// Calculate the attestation hash
	jsonAtt, err := json.Marshal(att)
	if err != nil {
		return nil, fmt.Errorf("marshaling attestation: %w", err)
	}

	// Calculate the attestation hash
	h, _, err := cr_v1.SHA256(bytes.NewBuffer(jsonAtt))
	if err != nil {
		return nil, fmt.Errorf("calculating attestation hash: %w", err)
	}
	attestationHash := h.String()

	predicate, err := chainloop.ExtractPredicate(att)
	if err != nil {
		return nil, fmt.Errorf("extracting predicate: %w", err)
	}

	referrers := make(ReferrerMap)
	// Add the attestation itself as a referrer to the map without references yet
	referrers[attestationHash] = &Referrer{
		digest:       attestationHash,
		artifactType: ReferrerAttestationType,
	}

	// Create new referrers for each material
	// and link them to the attestation
	for _, material := range predicate.GetMaterials() {
		// Create its referrer entry if it doesn't exist yet
		// the reason it might exist is because you might be attaching the same material twice
		// i.e the same SBOM twice, in that case we don't want to create a new referrer
		if _, ok := referrers[material.Hash.String()]; ok {
			continue
		}

		referrers[material.Hash.String()] = &Referrer{
			digest:       material.Hash.String(),
			artifactType: material.Type,
		}

		// Add the reference to the attestation
		referrers[attestationHash].references = append(referrers[attestationHash].references, material.Hash.String())
	}

	// TODO: add subjects including commits, tags, etc.

	return referrers, nil
}
