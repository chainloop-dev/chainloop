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

package attestation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"cuelang.org/go/cue/cuecontext"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"sigs.k8s.io/yaml"
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

// LoadJSONBytes Extracts raw data in JSON format from different sources, i.e cue or yaml files
func LoadJSONBytes(rawData []byte, extension string) ([]byte, error) {
	var jsonRawData []byte
	var err error

	switch extension {
	case ".yaml", ".yml":
		jsonRawData, err = yaml.YAMLToJSON(rawData)
		if err != nil {
			return nil, err
		}
	case ".cue":
		ctx := cuecontext.New()
		v := ctx.CompileBytes(rawData)
		jsonRawData, err = v.MarshalJSON()
		if err != nil {
			return nil, err
		}
	case ".json":
		jsonRawData = rawData
	default:
		return nil, errors.New("unsupported file format")
	}

	return jsonRawData, nil
}
