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

package materials

import (
	"bytes"
	"encoding/json"
	"fmt"

	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// JSONEnvelopeWithDigest returns the JSON content of the envelope and its digest.
func JSONEnvelopeWithDigest(envelope *dsse.Envelope) ([]byte, cr_v1.Hash, error) {
	jsonContent, err := json.Marshal(envelope)
	if err != nil {
		return nil, cr_v1.Hash{}, fmt.Errorf("marshaling the envelope: %w", err)
	}

	h, _, err := cr_v1.SHA256(bytes.NewBuffer(jsonContent))
	if err != nil {
		return nil, cr_v1.Hash{}, fmt.Errorf("calculating the digest: %w", err)
	}

	return jsonContent, h, nil
}
