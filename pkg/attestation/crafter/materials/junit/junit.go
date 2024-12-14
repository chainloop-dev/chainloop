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
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/joshdk/go-junit"
)

func Ingest(filePath string) ([]junit.Suite, error) {
	var suites []junit.Suite

	// read first chunk
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file %q: %w", filePath, err)
	}
	r := bufio.NewReader(f)

	buf := make([]byte, 512)
	_, err = io.ReadFull(r, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("reading file %q: %w", filePath, err)
	}
	_ = f.Close()

	// check if it's a zip file and try to ingest all its contents
	mime := http.DetectContentType(buf)
	switch strings.Split(mime, ";")[0] {
	case "application/zip":
		suites, err = ingestZipArchive(filePath)
		if err != nil {
			return nil, fmt.Errorf("could not ingest Zip archive : %w", err)
		}
	case "application/x-gzip":
		suites, err = ingestGzipArchive(filePath)
		if err != nil {
			return nil, fmt.Errorf("could not ingest GZip archive: %w", err)
		}
	case "text/xml", "application/xml":
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

func ingestGzipArchive(filename string) ([]junit.Suite, error) {
	result := make([]junit.Suite, 0)

	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	// Decompress the file if possible
	uncompressedStream, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("can't uncompress file, unexpected material type: %w", err)
	}

	// Create a tar reader
	tarReader := tar.NewReader(uncompressedStream)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Reached the end of tar archive
				break
			}
			return nil, fmt.Errorf("can't read tar header: %w", err)
		}
		// Check if the file is a regular file
		if header.Typeflag != tar.TypeReg || !strings.HasSuffix(header.Name, ".xml") {
			continue // Skip if it's not a regular file
		}

		suites, err := junit.IngestReader(tarReader)
		if err != nil {
			return nil, fmt.Errorf("can't ingest JUnit XML file %q: %w", header.Name, err)
		}
		result = append(result, suites...)
	}

	return result, nil
}

func ingestZipArchive(filename string) ([]junit.Suite, error) {
	result := make([]junit.Suite, 0)

	archive, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("could not open zip archive: %w", err)
	}
	defer archive.Close()

	for _, zf := range archive.File {
		if zf.FileInfo().IsDir() || !strings.HasSuffix(zf.Name, ".xml") {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			return nil, fmt.Errorf("could not open file %q: %w", zf.Name, err)
		}

		suites, err := junit.IngestReader(rc)
		if err != nil {
			return nil, fmt.Errorf("could not ingest JUnit XML file %q: %w", zf.Name, err)
		}

		result = append(result, suites...)
		_ = rc.Close()
	}

	return result, nil
}
