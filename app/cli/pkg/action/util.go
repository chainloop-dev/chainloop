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

package action

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
)

// LoadFileOrURL loads a file from a local path or a URL
func LoadFileOrURL(fileRef string) ([]byte, error) {
	parts := strings.SplitAfterN(fileRef, "://", 2)
	if len(parts) == 2 {
		scheme := parts[0]
		switch scheme {
		case "http://":
			fallthrough
		case "https://":
			// #nosec G107
			resp, err := http.Get(fileRef)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		default:
			return nil, errors.New("invalid file scheme")
		}
	}

	return os.ReadFile(filepath.Clean(fileRef))
}

// SchemaBase represents the minimal structure of a schema with metadata containing a name
type SchemaBase struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

// ExtractNameFromRawSchema tries to extract the name from any schema with metadata
func extractNameFromRawSchema(content []byte) (string, error) {
	// Identify format
	format, err := unmarshal.IdentifyFormat(content)
	if err != nil {
		return "", fmt.Errorf("failed to identify schema format: %w", err)
	}

	// Convert to JSON for consistent parsing
	jsonData, err := unmarshal.LoadJSONBytes(content, "."+string(format))
	if err != nil {
		return "", fmt.Errorf("failed to convert to JSON: %w", err)
	}

	// Unmarshal to extract name
	var schemaBase SchemaBase
	if err := json.Unmarshal(jsonData, &schemaBase); err != nil {
		// Unmarshalling error, don't fail, return empty name,
		return "", nil
	}

	return schemaBase.Metadata.Name, nil
}

// LoadSchemaAndExtractName loads a schema from file path and extracts name from metadata if available
func LoadSchemaAndExtractName(filePath, explicitName string) ([]byte, string, error) {
	finalName := explicitName

	if filePath != "" {
		rawSchema, err := LoadFileOrURL(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load schema file: %w", err)
		}

		// Extract name from the schema file content
		extractedName, err := extractNameFromRawSchema(rawSchema)
		if err != nil {
			return nil, "", err
		}

		// Name is required
		if extractedName == "" && explicitName == "" {
			return nil, "", fmt.Errorf("name in schema not found, --name flag is required")
		} else if extractedName != "" {
			finalName = extractedName
		}

		return rawSchema, finalName, nil
	}

	return nil, finalName, nil
}
