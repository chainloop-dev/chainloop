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

// ValidateAndExtractName validates and extracts a name from either
// an explicit name parameter OR from metadata.name in the file content.
// Ensures exactly one source is provided. Returns error when:
// - Neither explicit name nor metadata.name is provided
// - Both explicit name and metadata.name are provided (ambiguous)
func ValidateAndExtractName(explicitName, filePath string) (string, error) {
	// Load file content if provided
	var content []byte
	var err error
	if filePath != "" {
		content, err = LoadFileOrURL(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to load file: %w", err)
		}
	}

	// Extract name from v2 metadata (if present)
	metadataName, err := extractNameFromMetadata(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse content: %w", err)
	}

	// Both provided - ambiguous
	if explicitName != "" && metadataName != "" {
		return "", fmt.Errorf("conflicting names: explicit name (%q) and metadata.name (%q) both provided. Please provide only one", explicitName, metadataName)
	}

	// Neither provided - missing required name
	if explicitName == "" && metadataName == "" {
		if len(content) == 0 {
			return "", errors.New("name is required when no file is provided")
		}
		return "", errors.New("name is required: either provide explicit name or include metadata.name in the schema")
	}

	// Return whichever name was provided
	if explicitName != "" {
		return explicitName, nil
	}
	return metadataName, nil
}

// metadataWithName represents a partial structure to extract metadata.name field
type metadataWithName struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

// extractNameFromMetadata attempts to extract the name from metadata.name.
func extractNameFromMetadata(content []byte) (string, error) {
	if len(content) == 0 {
		return "", nil
	}

	// Identify the format
	format, err := unmarshal.IdentifyFormat(content)
	if err != nil {
		return "", err
	}

	// Convert to JSON for consistent unmarshaling
	var jsonData []byte
	switch format {
	case unmarshal.RawFormatJSON:
		jsonData = content
	case unmarshal.RawFormatYAML:
		jsonData, err = unmarshal.LoadJSONBytes(content, ".yaml")
		if err != nil {
			return "", err
		}
	case unmarshal.RawFormatCUE:
		jsonData, err = unmarshal.LoadJSONBytes(content, ".cue")
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	// Unmarshal just the metadata field
	var schema metadataWithName
	if err := json.Unmarshal(jsonData, &schema); err != nil {
		// Not a v2 schema or invalid format
		return "", nil
	}

	return schema.Metadata.Name, nil
}
