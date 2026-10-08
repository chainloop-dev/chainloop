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

package spec

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"gopkg.in/yaml.v3"
)

// Placeholder is the body that the agent writes for a capture whose source is
// a local file (spec issue-3561). It tells that the push fills the body from
// the file, and that the push has not done so yet. It is never recorded as
// content.
const Placeholder = "<!-- chainloop: content is read from the uri at push time -->"

// maxLocalSourceSize is the largest local file that the push reads as the body
// of a capture. A spec is text and small: a larger file is more likely a
// mistake than a spec.
const maxLocalSourceSize = 1 << 20

// readText converts one text file of the spec folder into a capture, and
// returns a warning when the file asked for content that the push could not
// get.
//
// When the header names a local file as the source, the body is the content
// of that file now, whatever the agent wrote below the header: the agent can
// edit the file many turns after it wrote the capture, and the evidence must
// hold the current version. The header the agent wrote is kept as it is. When
// the file cannot be used, the body the agent wrote stands, as for any other
// capture. The placeholder alone is never recorded.
func readText(repoRoot, name string, doc []byte, modTime time.Time) (*Capture, string) {
	header, end := frontmatterEnd(string(doc))
	body := string(doc[end:])

	uri := headerURI(header)
	path, local := localPath(repoRoot, homeDir(), uri)
	if local {
		if content, fileTime, ok := readLocalSource(path); ok {
			filled := fill(doc[:end], content)
			// An empty file gives no capture, and the fallback below decides
			// whether that is worth a warning.
			if entry := Parse(filled, fileTime); entry != nil {
				entry.FileName = name
				entry.Raw = filled
				entry.SourceDigest = pointer.Digest(content)

				return entry, ""
			}
		}
	}

	if entry := Parse(doc, modTime); entry != nil {
		entry.FileName = name
		entry.Raw = doc

		return entry, ""
	}

	switch {
	case isPlaceholder(body) && local:
		return nil, fmt.Sprintf("spec file %q was not recorded: its source file could not be read", name)
	case isPlaceholder(body):
		return nil, fmt.Sprintf("spec file %q was not recorded: it holds only the placeholder, and its source is not a local file", name)
	case local && strings.TrimSpace(body) == "":
		return nil, fmt.Sprintf("spec file %q was not recorded: it has no content, and its source file could not be read", name)
	}

	// A file with nothing in it and no local source is what "nothing to
	// capture" leaves behind.
	return nil, ""
}

// headerURI returns the source that a frontmatter header names, or "" when
// the header names none or cannot be read.
func headerURI(header string) string {
	if header == "" {
		return ""
	}

	var meta frontmatter
	if err := yaml.Unmarshal([]byte(header), &meta); err != nil {
		return ""
	}

	return strings.TrimSpace(meta.URI)
}

// homeDir returns the home directory, or "" when it is not known.
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return home
}

// localPath returns the path of the local file that a capture names as its
// source, and whether it names one. A local file is an absolute path, a path
// in the home directory (~/), a file:// URI, or a path relative to the
// repository root. Anything with a scheme, such as a web address, is not.
func localPath(repoRoot, home, uri string) (string, bool) {
	uri = strings.TrimSpace(uri)

	switch {
	case uri == "":
		return "", false
	case strings.HasPrefix(uri, "~/"):
		if home == "" {
			return "", false
		}
		return filepath.Join(home, uri[2:]), true
	case strings.HasPrefix(uri, "~"):
		// Another user's home directory is not a source of this session.
		return "", false
	case strings.HasPrefix(uri, "file://"):
		u, err := url.Parse(uri)
		if err != nil || (u.Host != "" && u.Host != "localhost") || !filepath.IsAbs(u.Path) {
			return "", false
		}
		return filepath.Clean(u.Path), true
	case filepath.IsAbs(uri):
		return filepath.Clean(uri), true
	case hasScheme(uri):
		return "", false
	}

	return filepath.Join(repoRoot, uri), true
}

// hasScheme reports whether uri starts with a URI scheme, such as https: or
// mailto:, rather than with a path.
func hasScheme(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil {
		// A path that net/url rejects, for example "1:x", is not a web
		// address either, but a colon before any slash is a scheme in all
		// but name.
		before, _, found := strings.Cut(uri, ":")
		return found && !strings.Contains(before, "/")
	}

	return u.Scheme != ""
}

// readLocalSource reads a local source file, and returns its content and its
// modification time. It reports false for anything other than a regular text
// file under maxLocalSourceSize.
func readLocalSource(path string) ([]byte, time.Time, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxLocalSourceSize {
		return nil, time.Time{}, false
	}

	// The file can grow between the stat and the read.
	content, err := io.ReadAll(io.LimitReader(f, maxLocalSourceSize+1))
	if err != nil || len(content) > maxLocalSourceSize {
		return nil, time.Time{}, false
	}

	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return nil, time.Time{}, false
	}

	return content, info.ModTime(), true
}

// fill joins the header block that the agent wrote and the content of the
// source file into the capture that the push records.
func fill(header, content []byte) []byte {
	out := make([]byte, 0, len(header)+1+len(content))
	out = append(out, header...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}

	return append(out, content...)
}
