//
// Copyright 2024-2026 The Chainloop Authors.
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

package unmarshal

import (
	"encoding/json"
	"errors"
	"fmt"

	"buf.build/go/protovalidate"
	"buf.build/go/protoyaml"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v2"
	syaml "sigs.k8s.io/yaml"
)

type RawFormat string

const (
	RawFormatJSON RawFormat = "json"
	RawFormatYAML RawFormat = "yaml"
	// RawFormatCUE is retained only so contracts already stored with this format
	// (and the wire enum) remain valid. CUE is no longer accepted or evaluated:
	// evaluating attacker-supplied CUE server-side is an unbounded, uncancellable
	// operation and was a DoS vector. New contracts must be JSON or YAML.
	RawFormatCUE RawFormat = "cue"
)

// errCUENotSupported is returned wherever a CUE document would previously have
// been compiled and evaluated. CUE support was removed to close the unbounded
// server-side evaluation DoS.
var errCUENotSupported = errors.New("CUE contract format is no longer supported; use JSON or YAML")

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (RawFormat) Values() (kinds []string) {
	for _, s := range []RawFormat{RawFormatJSON, RawFormatYAML, RawFormatCUE} {
		kinds = append(kinds, string(s))
	}
	return
}

// validatorAdapter adapts protovalidate.Validator to work with protoyaml.Validator.
// protovalidate v1.1.0 changed the Validate signature to accept variadic options,
// but protoyaml v0.6.0 expects the old signature without options.
type validatorAdapter struct {
	validator protovalidate.Validator
}

func (v *validatorAdapter) Validate(msg proto.Message) error {
	return v.validator.Validate(msg)
}

// yamlValidator wraps the protovalidate global Validator for use with protoyaml,
// initialised once and reused across calls.
var yamlValidator = &validatorAdapter{validator: protovalidate.GlobalValidator}

func FromRaw(body []byte, format RawFormat, out proto.Message, doValidate bool) error {
	// DiscardUnknown allows contracts to include fields added in newer proto
	// versions without breaking older CLIs that haven't been updated yet. Unlike
	// the binary wire format, protojson/protoyaml error on unknown fields by default.
	jsonOpts := protojson.UnmarshalOptions{DiscardUnknown: true}

	switch format {
	case RawFormatJSON:
		if err := jsonOpts.Unmarshal(body, out); err != nil {
			return fmt.Errorf("error unmarshalling raw message: %w", err)
		}
	case RawFormatYAML:
		// protoyaml allows validating the contract while unmarshalling
		yamlOpts := protoyaml.UnmarshalOptions{DiscardUnknown: true}
		if doValidate {
			yamlOpts.Validator = yamlValidator
		}

		if err := yamlOpts.Unmarshal(body, out); err != nil {
			return fmt.Errorf("error unmarshalling raw message: %w", err)
		}
	case RawFormatCUE:
		return errCUENotSupported
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if doValidate {
		if err := protovalidate.Validate(out); err != nil {
			return fmt.Errorf("error validating raw message: %w", err)
		}
	}

	return nil
}

// IdentifyFormat does best effort to identify the format of the raw contract
// by going the unmarshalling path in the following order: json, yaml.
// NOTE that we are just validating the format, not the content using regular marshalling
// not even proto marshalling, that comes later once we know the format.
// CUE is intentionally not detected: it is no longer a supported contract format.
func IdentifyFormat(raw []byte) (RawFormat, error) {
	// json marshalling first
	var sink any
	if err := json.Unmarshal(raw, &sink); err == nil {
		return RawFormatJSON, nil
	}

	// yaml marshalling last
	if err := yaml.Unmarshal(raw, &sink); err == nil {
		return RawFormatYAML, nil
	}

	return "", errors.New("format not found")
}

// LoadJSONBytes Extracts raw data in JSON format from different sources, i.e yaml or json files
func LoadJSONBytes(rawData []byte, extension string) ([]byte, error) {
	var jsonRawData []byte
	var err error

	switch extension {
	case ".yaml", ".yml":
		jsonRawData, err = syaml.YAMLToJSON(rawData)
		if err != nil {
			return nil, err
		}
	case ".cue":
		return nil, errCUENotSupported
	case ".json":
		jsonRawData = rawData
	default:
		return nil, errors.New("unsupported file format")
	}

	return jsonRawData, nil
}
