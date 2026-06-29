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
	"bytes"
	"fmt"
	"os"
	"strings"
)

// ArchiveFormat identifies a supported archive container.
type ArchiveFormat int

const (
	ArchiveNone ArchiveFormat = iota
	ArchiveZip
	ArchiveTar
	ArchiveTarGz
)

// DetectArchive reports whether path is a supported archive and, if so, its
// format. Detection is by extension first; for files whose extension does not
// match, magic bytes are used as a backstop so renamed archives are still
// caught. A non-archive returns (ArchiveNone, nil).
func DetectArchive(path string) (ArchiveFormat, error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return ArchiveZip, nil
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return ArchiveTarGz, nil
	case strings.HasSuffix(lower, ".tar"):
		return ArchiveTar, nil
	}

	return detectByMagic(path)
}

func detectByMagic(path string) (ArchiveFormat, error) {
	f, err := os.Open(path)
	if err != nil {
		return ArchiveNone, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()

	// 512 bytes is enough for the gzip/zip magic and the tar "ustar" marker at
	// offset 257.
	header := make([]byte, 512)
	n, _ := f.Read(header)
	header = header[:n]

	switch {
	case bytes.HasPrefix(header, []byte("PK\x03\x04")), bytes.HasPrefix(header, []byte("PK\x05\x06")):
		return ArchiveZip, nil
	case bytes.HasPrefix(header, []byte{0x1f, 0x8b}):
		return ArchiveTarGz, nil
	case len(header) >= 262 && bytes.Equal(header[257:262], []byte("ustar")):
		return ArchiveTar, nil
	}

	return ArchiveNone, nil
}
