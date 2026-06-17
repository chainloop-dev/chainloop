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

package materials

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/radamsa"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

// AnnotationRadamsaCrashesCount is the annotation holding the number of crashing
// inputs recorded in a RADAMSA_CRASHES material (0 means no crash).
const AnnotationRadamsaCrashesCount = "chainloop.material.radamsa.crashes.count"

const radamsaToolName = "radamsa"

// RadamsaReportCrafter crafts a RADAMSA_REPORT material out of radamsa's -M
// metadata log.
type RadamsaReportCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewRadamsaReportCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*RadamsaReportCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_RADAMSA_REPORT {
		return nil, fmt.Errorf("material type is not a radamsa report")
	}
	return &RadamsaReportCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: schema},
	}, nil
}

func (c *RadamsaReportCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	if _, err := radamsa.Parse(f); err != nil {
		return nil, fmt.Errorf("invalid radamsa -M metadata log: %w: %w", ErrInvalidMaterialType, err)
	}

	m, err := uploadAndCraft(ctx, c.input, c.backend, filePath, c.logger)
	if err != nil {
		return nil, err
	}
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = radamsaToolName
	return m, nil
}

// RadamsaCrashesCrafter crafts a RADAMSA_CRASHES material out of either a single
// crashing input or a crashes/ archive (tar.gz or zip). It is metadata-only: the
// crash count is recorded as an annotation rather than evaluated as content.
type RadamsaCrashesCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewRadamsaCrashesCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*RadamsaCrashesCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_RADAMSA_CRASHES {
		return nil, fmt.Errorf("material type is not radamsa crashes")
	}
	return &RadamsaCrashesCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: schema},
	}, nil
}

func (c *RadamsaCrashesCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	isArchive, fileCount, err := inspectCrashesArchive(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading crashes archive: %w", err)
	}

	count := fileCount
	if !isArchive {
		// single crashing input: must be non-empty, counts as one crash.
		if info.Size() == 0 {
			return nil, fmt.Errorf("%w: crash file is empty", ErrInvalidMaterialType)
		}
		count = 1
	}

	m, err := uploadAndCraft(ctx, c.input, c.backend, filePath, c.logger)
	if err != nil {
		return nil, err
	}
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = radamsaToolName
	m.Annotations[AnnotationRadamsaCrashesCount] = strconv.Itoa(count)
	return m, nil
}

// inspectCrashesArchive reports whether path is a readable zip or tar.gz and, if
// so, how many regular-file entries it contains. A file that is not a valid
// archive returns (false, 0, nil) so the caller treats it as a single crash.
func inspectCrashesArchive(path string) (bool, int, error) {
	magic, err := readMagic(path)
	if err != nil {
		return false, 0, err
	}
	switch {
	// PK\x03\x04 local file header (entries present), PK\x05\x06 end-of-central
	// -directory (an empty archive), PK\x07\x08 spanned-archive marker.
	case bytes.HasPrefix(magic, []byte("PK\x03\x04")),
		bytes.HasPrefix(magic, []byte("PK\x05\x06")),
		bytes.HasPrefix(magic, []byte("PK\x07\x08")):
		n, ok := countZipEntries(path)
		return ok, n, nil
	case bytes.HasPrefix(magic, []byte{0x1f, 0x8b}):
		n, ok := countTarGzEntries(path)
		return ok, n, nil
	default:
		return false, 0, nil
	}
}

func readMagic(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, 4)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buf[:n], nil
}

func countZipEntries(path string) (int, bool) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return 0, false
	}
	defer zr.Close()
	count := 0
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			count++
		}
	}
	return count, true
}

func countTarGzEntries(path string) (int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, false
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	count := 0
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, false
		}
		if hdr.Typeflag == tar.TypeReg {
			count++
		}
	}
	return count, true
}
