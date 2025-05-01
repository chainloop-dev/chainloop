//
// Copyright 2025 The Chainloop Authors.
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

package attestation

import (
	"encoding/json"
	"fmt"

	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func ExtractDSSEEnvelope(data []byte) (*dsse.Envelope, error) {
	var dsseEnvelope dsse.Envelope
	var bundle protobundle.Bundle
	if err := protojson.Unmarshal(data, &bundle); err == nil && bundle.GetMediaType() != "" {
		// if it has a media type, we can confirm it's a bundle
		env := attestation.DSSEEnvelopeFromBundle(&bundle)
		dsseEnvelope = *env
	} else {
		// try to parse it as a DSSE envelope
		if err = json.Unmarshal(data, &dsseEnvelope); err != nil {
			return nil, fmt.Errorf("failed to parse the provided file as a DSSE envelope: %w", err)
		}
	}
	return &dsseEnvelope, nil
}
