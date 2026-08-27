//
// Copyright 2026 The Chainloop Authors.
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

package prinfo

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Version represents the version of the PR/MR info schema.
type Version string

const (
	// Version1_0 represents PR/MR Info version 1.0 schema.
	Version1_0 Version = "1.0"
	// Version1_1 represents PR/MR Info version 1.1 schema (adds reviewers).
	Version1_1 Version = "1.1"
	// Version1_2 represents PR/MR Info version 1.2 schema (adds requested and review_status to reviewers).
	Version1_2 Version = "1.2"
	// Version1_3 represents PR/MR Info version 1.3 schema (author as object with type).
	Version1_3 Version = "1.3"

	// LatestVersion is the schema version emitted by this package.
	LatestVersion = Version1_3
)

// ErrInvalidJSONPayload represents an error for an invalid JSON payload.
var ErrInvalidJSONPayload = errors.New("invalid JSON payload")

var (
	//go:embed schemas/pr-info-1.0.schema.json
	specVersion1_0 string
	//go:embed schemas/pr-info-1.1.schema.json
	specVersion1_1 string
	//go:embed schemas/pr-info-1.2.schema.json
	specVersion1_2 string
	//go:embed schemas/pr-info-1.3.schema.json
	specVersion1_3 string
)

// rawSchemas holds the canonical JSON schema documents indexed by version.
var rawSchemas = map[Version]string{
	Version1_0: specVersion1_0,
	Version1_1: specVersion1_1,
	Version1_2: specVersion1_2,
	Version1_3: specVersion1_3,
}

var (
	compiledSchemas map[Version]*jsonschema.Schema
	compileOnce     sync.Once
)

// SchemaURL returns the canonical, published URL of the given schema version.
func SchemaURL(version Version) string {
	return fmt.Sprintf("https://schemas.chainloop.dev/prinfo/%s/pr-info.schema.json", version)
}

// Schema returns the raw JSON schema document for the given version.
func Schema(version Version) (string, error) {
	raw, ok := rawSchemas[version]
	if !ok {
		return "", fmt.Errorf("invalid PR info schema version %q", version)
	}

	return raw, nil
}

// Versions returns the supported schema versions, oldest first.
func Versions() []Version {
	return []Version{Version1_0, Version1_1, Version1_2, Version1_3}
}

func initSchemas() {
	compiler := jsonschema.NewCompiler()
	for _, version := range Versions() {
		url := SchemaURL(version)
		if err := compiler.AddResource(url, strings.NewReader(rawSchemas[version])); err != nil {
			panic(fmt.Sprintf("prinfo: failed to add resource %s: %v", url, err))
		}
	}

	compiledSchemas = make(map[Version]*jsonschema.Schema, len(rawSchemas))
	for _, version := range Versions() {
		compiledSchemas[version] = compiler.MustCompile(SchemaURL(version))
	}
}

// Validate validates a generically-decoded PR/MR info data payload against the
// given schema version. An empty version defaults to LatestVersion.
func Validate(data any, version Version) error {
	compileOnce.Do(initSchemas)

	if version == "" {
		version = LatestVersion
	}

	schema, ok := compiledSchemas[version]
	if !ok {
		return fmt.Errorf("invalid PR info schema version %q", version)
	}

	if err := schema.Validate(data); err != nil {
		var invalidJSONTypeError jsonschema.InvalidJSONTypeError
		if errors.As(err, &invalidJSONTypeError) {
			return ErrInvalidJSONPayload
		}
		return err
	}

	return nil
}
