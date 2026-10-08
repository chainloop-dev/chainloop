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

// Package skill keeps a copy of each skill folder that a coding session used,
// and turns the copy into the materials that the evidence holds.
//
// The copy is made when the session uses the skill, not when the session is
// pushed, so the evidence holds the skill that ran also when its files change
// before the push. A copy is never replaced: a later use that finds different
// content only marks the copy as changed.
//
// A copy holds the files of the folder as they are. Secrets are removed only
// when a package is made from the copy, by the same redaction as the other
// spec sources.
package skill

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// DefinitionFile is the file that defines a skill. A folder without it is
	// not a skill, and is never read.
	DefinitionFile = "SKILL.md"

	// MaxPackageSize is the largest archive that is uploaded. A larger skill
	// keeps its definition and its counts, and loses its package only.
	MaxPackageSize = 5 << 20

	// maxCopySize bounds the bytes that a hook copies. The archive is
	// compressed, so a folder can be larger than MaxPackageSize and still
	// give a package under the limit. A folder over this bound cannot.
	maxCopySize = 50 << 20

	// filesDir holds the copied files inside a copy.
	filesDir = "files"
	// infoFile records what the copy was made from.
	infoFile = "info.json"
	// changedFile marks a copy whose folder changed after the first use.
	changedFile = "changed"
	// packageDir holds the redacted definition and archive, made at the first
	// push that needs them.
	packageDir     = "package"
	packageArchive = "package.tar.gz"
	// oversizeFile marks a copy whose archive is over MaxPackageSize.
	oversizeFile = "oversize"

	// baseDirMarker opens the line that tells where an agent loaded a skill
	// from. Claude Code writes it with a path, and OpenCode with a file URL.
	baseDirMarker = "Base directory for this skill:"
)

// vcsDirs are the version-control folders that a package leaves out.
var vcsDirs = []string{".git", ".hg", ".svn", ".bzr", ".jj"}

// ErrNotSkill is returned for a folder that holds no SKILL.md file.
var ErrNotSkill = errors.New("the folder holds no " + DefinitionFile)

// Info describes a copy.
type Info struct {
	// Digest identifies the content of the folder when it was copied. The
	// same content gives the same digest in every session.
	Digest string `json:"digest"`
	// Truncated reports a folder over the copy bound. Only its definition was
	// copied, and it gets no package.
	Truncated bool `json:"truncated,omitempty"`
	// SkippedLinks counts the links that point out of the folder, or to a
	// folder, and so were left out.
	SkippedLinks int `json:"skipped_links,omitempty"`
	// Changed reports that a later use found different content in the folder.
	// The copy keeps the content of the first use.
	Changed bool `json:"-"`
}

// file is one file of a skill folder that goes into a copy.
type file struct {
	// rel is the path inside the folder, with forward slashes.
	rel string
	// path is where the content is read from. For a link inside the folder,
	// it is the target of the link.
	path string
	// executable keeps the one mode bit a script needs.
	executable bool
	size       int64
}

// Copy copies the skill folder src into dst, which must not exist yet, and
// returns what it copied. When another hook renamed its copy into dst first,
// Copy returns that copy unchanged.
func Copy(src, dst string) (Info, error) {
	files, skipped, err := listFiles(src)
	if err != nil {
		return Info{}, err
	}

	info := Info{SkippedLinks: skipped}

	var total int64
	for _, f := range files {
		total += f.size
	}
	// A folder over the bound keeps only its definition. Its digest is still
	// that of the whole folder, for the comparison at a later use.
	info.Truncated = total > maxCopySize

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return Info{}, fmt.Errorf("create skill copy directory: %w", err)
	}

	// The copy is made in a sibling folder and renamed into place, so that a
	// reader never sees half of it and two hooks cannot mix their files.
	tmp, err := os.MkdirTemp(filepath.Dir(dst), "."+filepath.Base(dst)+"-*")
	if err != nil {
		return Info{}, fmt.Errorf("create skill copy directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	h := sha256.New()
	for _, f := range files {
		content, err := os.ReadFile(f.path)
		if err != nil {
			return Info{}, fmt.Errorf("read skill file %q: %w", f.rel, err)
		}

		writeDigestEntry(h, f.rel, f.executable, content)
		if info.Truncated && f.rel != DefinitionFile {
			continue
		}

		mode := os.FileMode(0o600)
		if f.executable {
			mode = 0o700
		}

		// The path comes from a walk of the folder, so it cannot leave the
		// copy. The check keeps that true if the walk ever changes.
		target := filepath.Join(tmp, filesDir, filepath.FromSlash(f.rel))
		if !Within(target, filepath.Join(tmp, filesDir)) {
			return Info{}, fmt.Errorf("skill file %q is outside the skill folder", f.rel)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return Info{}, fmt.Errorf("copy skill file %q: %w", f.rel, err)
		}
		//nolint:gosec // target is checked above to stay inside the copy
		if err := os.WriteFile(target, content, mode); err != nil {
			return Info{}, fmt.Errorf("copy skill file %q: %w", f.rel, err)
		}
	}

	info.Digest = "sha256:" + hex.EncodeToString(h.Sum(nil))

	data, err := json.Marshal(info)
	if err != nil {
		return Info{}, fmt.Errorf("encode skill copy info: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, infoFile), data, 0o600); err != nil {
		return Info{}, fmt.Errorf("write skill copy info: %w", err)
	}

	if err := os.Rename(tmp, dst); err != nil {
		// Another hook renamed its copy into place first. Its copy is of the
		// same use, so it stands.
		if existing, loadErr := Load(dst); loadErr == nil {
			return existing, nil
		}

		return Info{}, fmt.Errorf("store skill copy: %w", err)
	}

	return info, nil
}

// Load returns the description of an existing copy.
func Load(dst string) (Info, error) {
	data, err := os.ReadFile(filepath.Join(dst, infoFile))
	if err != nil {
		return Info{}, err
	}

	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, fmt.Errorf("parse skill copy info: %w", err)
	}

	if _, err := os.Stat(filepath.Join(dst, changedFile)); err == nil {
		info.Changed = true
	}

	return info, nil
}

// MarkChanged records that a later use found content in the folder that is
// not the content of the copy.
func MarkChanged(dst string) error {
	return os.WriteFile(filepath.Join(dst, changedFile), nil, 0o600)
}

// ContentDigest returns the digest of the content of a skill folder, as Copy
// computes it, without copying anything.
func ContentDigest(src string) (string, error) {
	files, _, err := listFiles(src)
	if err != nil {
		return "", err
	}

	return contentDigest(files)
}

func contentDigest(files []file) (string, error) {
	h := sha256.New()
	for _, f := range files {
		content, err := os.ReadFile(f.path)
		if err != nil {
			return "", fmt.Errorf("read skill file %q: %w", f.rel, err)
		}
		writeDigestEntry(h, f.rel, f.executable, content)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// writeDigestEntry adds one file to a content digest. Each field has its
// length first, so that no two lists of files give the same input.
func writeDigestEntry(w io.Writer, rel string, executable bool, content []byte) {
	sum := sha256.Sum256(content)
	_, _ = fmt.Fprintf(w, "%d:%s:%t:%x\n", len(rel), rel, executable, sum)
}

// listFiles returns the files of a skill folder that go into a copy, sorted by
// path, and the number of links that it left out.
//
// The folder itself can be a link: an agent often installs a skill as a link
// to a checkout. The links inside it are followed only to a file inside the
// folder. A link out of the folder, or to a folder, is left out, so a skill
// cannot pull other files of the user into the evidence, and a loop of links
// cannot stop the walk.
func listFiles(src string) (files []file, skipped int, err error) {
	root, err := filepath.EvalSymlinks(src)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve skill folder: %w", err)
	}

	if info, err := os.Stat(filepath.Join(root, DefinitionFile)); err != nil || !info.Mode().IsRegular() {
		return nil, 0, ErrNotSkill
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if path != root && slices.Contains(vcsDirs, d.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		// A git worktree or submodule keeps a .git file, not a folder.
		if slices.Contains(vcsDirs, d.Name()) {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		source := path
		if d.Type()&fs.ModeSymlink != 0 {
			target, ok := linkInside(root, path)
			if !ok {
				skipped++
				return nil
			}
			source = target
		}

		info, err := os.Stat(source)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			if info.IsDir() {
				skipped++
			}

			return nil
		}

		files = append(files, file{
			rel:        filepath.ToSlash(rel),
			path:       source,
			executable: info.Mode().Perm()&0o111 != 0,
			size:       info.Size(),
		})

		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("read skill folder: %w", err)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })

	return files, skipped, nil
}

// linkInside resolves a link and reports whether its target is a file or a
// folder inside root.
func linkInside(root, link string) (string, bool) {
	target, err := filepath.EvalSymlinks(link)
	if err != nil {
		return "", false
	}

	return target, Within(target, root)
}

// Within reports whether path is root or a path below it.
func Within(path, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}

	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// Package is what the evidence holds for one skill.
type Package struct {
	// Definition is the redacted SKILL.md file.
	Definition []byte
	// Archive is a reproducible gzip-compressed tar archive of the redacted
	// folder, or nil when it is over MaxPackageSize or the copy was
	// truncated.
	Archive []byte
	// Oversize reports that the archive was left out for its size.
	Oversize bool
}

// Redactor removes secrets from the text of one file.
type Redactor func(doc []byte) ([]byte, error)

// MakePackage returns the package of a copy. The first call redacts each text
// file and keeps the result in the copy, so a later push of the session does
// not redact an unchanged skill again. Binary files are stored as they are.
//
// Redaction fails closed: a file that could not be scanned fails the package,
// and nothing of the skill is uploaded.
func MakePackage(dst string, redact Redactor) (Package, error) {
	info, err := Load(dst)
	if err != nil {
		return Package{}, fmt.Errorf("read skill copy: %w", err)
	}

	cache := filepath.Join(dst, packageDir)
	if pkg, ok := loadPackage(cache); ok {
		return pkg, nil
	}

	files, _, err := listFiles(filepath.Join(dst, filesDir))
	if err != nil {
		return Package{}, err
	}

	var pkg Package
	var buf bytes.Buffer

	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return Package{}, err
	}
	tw := tar.NewWriter(gz)

	for _, f := range files {
		content, err := os.ReadFile(f.path)
		if err != nil {
			return Package{}, fmt.Errorf("read skill file %q: %w", f.rel, err)
		}

		if utf8.Valid(content) {
			if content, err = redact(content); err != nil {
				return Package{}, fmt.Errorf("scanning %q for secrets: %w", f.rel, err)
			}
		}

		if f.rel == DefinitionFile {
			pkg.Definition = content
		}

		if err := writeTarEntry(tw, f, content); err != nil {
			return Package{}, err
		}
	}

	if err := tw.Close(); err != nil {
		return Package{}, fmt.Errorf("close skill archive: %w", err)
	}
	if err := gz.Close(); err != nil {
		return Package{}, fmt.Errorf("close skill archive: %w", err)
	}

	if info.Truncated || buf.Len() > MaxPackageSize {
		pkg.Oversize = true
	} else {
		pkg.Archive = buf.Bytes()
	}

	// A cache we cannot write costs the next push a redaction, never this
	// push its package.
	_ = storePackage(cache, pkg)

	return pkg, nil
}

// writeTarEntry adds one file with fixed metadata, so the same content gives
// the same archive on every machine: no time, no owner, and a mode that keeps
// only the executable bit.
func writeTarEntry(tw *tar.Writer, f file, content []byte) error {
	mode := int64(0o644)
	if f.executable {
		mode = 0o755
	}

	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     f.rel,
		Mode:     mode,
		Size:     int64(len(content)),
		ModTime:  time.Unix(0, 0).UTC(),
		Format:   tar.FormatPAX,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write skill archive: %w", err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("write skill archive: %w", err)
	}

	return nil
}

func loadPackage(cache string) (Package, bool) {
	definition, err := os.ReadFile(filepath.Join(cache, DefinitionFile))
	if err != nil {
		return Package{}, false
	}

	pkg := Package{Definition: definition}
	if _, err := os.Stat(filepath.Join(cache, oversizeFile)); err == nil {
		pkg.Oversize = true
		return pkg, true
	}

	archive, err := os.ReadFile(filepath.Join(cache, packageArchive))
	if err != nil {
		return Package{}, false
	}
	pkg.Archive = archive

	return pkg, true
}

// storePackage writes the package next to the copy. The definition goes last,
// because loadPackage takes its presence to mean that the rest is there.
func storePackage(cache string, pkg Package) error {
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return err
	}

	if pkg.Oversize {
		if err := os.WriteFile(filepath.Join(cache, oversizeFile), nil, 0o600); err != nil {
			return err
		}
	} else if err := os.WriteFile(filepath.Join(cache, packageArchive), pkg.Archive, 0o600); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(cache, DefinitionFile), pkg.Definition, 0o600)
}

// BaseDir returns the skill folder that a "Base directory for this skill:"
// line in text names. Claude Code writes a path, and OpenCode a file URL. Only
// an absolute path is accepted.
func BaseDir(text string) (string, bool) {
	for line := range strings.SplitSeq(text, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), baseDirMarker)
		if !ok {
			continue
		}

		return ParseDir(rest)
	}

	return "", false
}

// ParseDir reads a skill folder as an agent gives it: a path or a file URL.
// Only an absolute path is accepted.
func ParseDir(s string) (string, bool) {
	dir := strings.TrimSpace(s)
	if strings.HasPrefix(dir, "file://") {
		u, err := url.Parse(dir)
		if err != nil {
			return "", false
		}
		dir = u.Path
	}

	if dir == "" || !filepath.IsAbs(dir) {
		return "", false
	}

	return filepath.Clean(dir), true
}

// HasBaseDir reports whether text can hold a base directory line. It is a
// cheap test that a scan of a large transcript runs before it decodes a line.
func HasBaseDir(text []byte) bool {
	return bytes.Contains(text, []byte(baseDirMarker))
}
