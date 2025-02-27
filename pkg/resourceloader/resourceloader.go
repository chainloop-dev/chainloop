//
// Copyright 20245 The Chainloop Authors.
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

package resourceloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// UnrecognizedSchemeError is an error type for when a URL scheme is not recognized out of
// the supported ones.
type UnrecognizedSchemeError struct {
	Scheme string
}

func (e *UnrecognizedSchemeError) Error() string {
	return fmt.Sprintf("loading URL: unrecognized scheme: %s", e.Scheme)
}

// GetPathForResource tries to load a file or URL from the given path.
// If the path starts with "http://" or "https://", it will try to load the file from the URL and save it
// in a temporary file. It will return the path to the temporary file.
// If the path is an actual file path, it will return the filepath
func GetPathForResource(resourcePath string) (string, error) {
	if _, err := os.Stat(resourcePath); err == nil {
		return resourcePath, nil
	}

	// Try to load the resource from a URL
	raw, err := loadResourceFromURLOrEnv(resourcePath)
	if err != nil {
		return "", fmt.Errorf("loading resource: %w", err)
	}

	// If the resource is loaded from a URL, save it in a temporary file
	return createTempFile(resourcePath, raw)
}

func loadResourceFromURLOrEnv(resourcePath string) ([]byte, error) {
	parts := strings.SplitAfterN(resourcePath, "://", 2)
	// If the path does not contain a scheme, it is considered a file path
	if len(parts) != 2 {
		return nil, &UnrecognizedSchemeError{Scheme: parts[0]}
	}

	switch parts[0] {
	case "http://", "https://":
		return loadFromURL(resourcePath)
	case "env://":
		return loadFromEnv(parts[1])
	default:
		return nil, &UnrecognizedSchemeError{Scheme: parts[0]}
	}
}

// loadFromURL loads the content of a URL and returns it as a byte slice.
func loadFromURL(url string) ([]byte, error) {
	// As cosign does: https://github.com/sigstore/cosign/blob/beb9cf21bc6741bc6e6b9736bdf57abfb91599c0/pkg/blob/load.go#L47
	// #nosec G107
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("requesting URL: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("loading URL response: %w", err)
	}
	return raw, nil
}

// loadFromEnv loads the content of an environment variable and returns it as a byte slice.
func loadFromEnv(envVar string) ([]byte, error) {
	value, found := os.LookupEnv(envVar)
	if !found {
		return nil, fmt.Errorf("loading URL: env var $%s not found", envVar)
	}
	return []byte(value), nil
}

// createTempFile creates a temporary file with the given filename and writes the given data to it.
func createTempFile(filename string, rawData []byte) (string, error) {
	// Create a temporary directory with a random name to avoid collisions
	tempDir, err := os.MkdirTemp("", "chainloop-inflight-dir-*")
	if err != nil {
		return "", fmt.Errorf("creating temporary directory: %w", err)
	}

	// Create a temporary file with the same name as the original file
	tempFile, err := os.Create(filepath.Join(tempDir, filepath.Base(filename)))
	if err != nil {
		return "", fmt.Errorf("creating temporary file: %w", err)
	}
	// Close the file when we are done
	defer tempFile.Close()

	// Write the data to the temporary file
	if _, err := tempFile.Write(rawData); err != nil {
		return "", fmt.Errorf("writing to temporary file: %w", err)
	}

	return tempFile.Name(), nil
}
