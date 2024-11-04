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

package unmarshal

import (
	"encoding/json"
	"errors"
	"fmt"

	"cuelang.org/go/cue/cuecontext"
	"github.com/bufbuild/protovalidate-go"
	"github.com/bufbuild/protoyaml-go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v2"
	syaml "sigs.k8s.io/yaml"
)

type RawFormat string

const (
	RawFormatJSON RawFormat = "json"
	RawFormatYAML RawFormat = "yaml"
	RawFormatCUE  RawFormat = "cue"
)

type UnmarshalError struct {
}

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (RawFormat) Values() (kinds []string) {
	for _, s := range []RawFormat{RawFormatJSON, RawFormatYAML, RawFormatCUE} {
		kinds = append(kinds, string(s))
	}
	return
}

func UnmarshalFromRaw(body []byte, format RawFormat, out proto.Message) error {
	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("could not create validator: %w", err)
	}

	switch format {
	case RawFormatJSON:
		if err := protojson.Unmarshal(body, out); err != nil {
			return fmt.Errorf("error unmarshalling raw message: %w", err)
		}
	case RawFormatYAML:
		// protoyaml allows validating the contract while unmarshalling
		yamlOpts := protoyaml.UnmarshalOptions{Validator: validator}
		if err := yamlOpts.Unmarshal(body, out); err != nil {
			return fmt.Errorf("error unmarshalling raw message: %w", err)
		}
	case RawFormatCUE:
		ctx := cuecontext.New()
		v := ctx.CompileBytes(body)
		jsonRawData, err := v.MarshalJSON()
		if err != nil {
			return fmt.Errorf("error unmarshalling raw message: %w", err)
		}

		if err := protojson.Unmarshal(jsonRawData, out); err != nil {
			return fmt.Errorf("error unmarshalling raw message: %w", err)
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	err = validator.Validate(out)
	if err != nil {
		return fmt.Errorf("error validating raw message: %w", err)
	}
	return nil
}

// IdentifyFormat does best effort to identify the format of the raw contract
// by going the unmarshalling path in the following order: json, cue, yaml
// NOTE that we are just validating the format, not the content using regular marshalling
// not even proto marshalling, that comes later once we know the format
func IdentifyFormat(raw []byte) (RawFormat, error) {
	// json marshalling first
	var sink any
	if err := json.Unmarshal(raw, &sink); err == nil {
		return RawFormatJSON, nil
	}

	// cue marshalling next
	ctx := cuecontext.New()
	v := ctx.CompileBytes(raw)
	if _, err := v.MarshalJSON(); err == nil {
		return RawFormatCUE, nil
	}

	// yaml marshalling last
	if err := yaml.Unmarshal(raw, &sink); err == nil {
		return RawFormatYAML, nil
	}

	return "", errors.New("format not found")
}

// LoadJSONBytes Extracts raw data in JSON format from different sources, i.e cue or yaml files
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
