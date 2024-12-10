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

package junit

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/joshdk/go-junit"
)

func Ingest(filePath string) ([]junit.Suite, error) {
	var suites []junit.Suite
	// check if it's a zip file and try to ingest all its contents
	mime, err := mimetype.DetectFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not detect mime type: %w", err)
	}
	switch {
	case mime.Is("application/zip"):
		suites, err = ingestArchive(filePath)
		if err != nil {
			return nil, fmt.Errorf("could not ingest JUnit XML: %w", err)
		}
	case mime.Is("text/xml"):
		suites, err = junit.IngestFile(filePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("invalid file path: %w", err)
			}
			return nil, fmt.Errorf("invalid JUnit XML file: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid JUnit XML file: %s", filePath)
	}

	return suites, nil
}

func ingestArchive(filename string) ([]junit.Suite, error) {
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("could not open zip archive: %w", err)
	}
	defer archive.Close()
	dir, err := os.MkdirTemp("", "junit")
	if err != nil {
		return nil, fmt.Errorf("could not create temporary directory: %w", err)
	}
	for _, zf := range archive.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		// extract file to dir
		// nolint: gosec
		path := filepath.Join(dir, zf.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dir)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("illegal file path: %s", path)
		}

		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("could not open file %s: %w", path, err)
		}

		rc, err := zf.Open()
		if err != nil {
			return nil, fmt.Errorf("could not open file %s: %w", path, err)
		}

		_, err = f.ReadFrom(rc)
		if err != nil {
			return nil, fmt.Errorf("could not read file %s: %w", path, err)
		}

		rc.Close()
		f.Close()
	}

	suites, err := junit.IngestDir(dir)
	if err != nil {
		return nil, fmt.Errorf("could not ingest JUnit XML: %w", err)
	}

	return suites, nil
}
