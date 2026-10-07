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

package skill

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSkill makes a skill folder from a map of relative paths to contents.
func writeSkill(t *testing.T, dir string, files map[string]string) string {
	t.Helper()

	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	return dir
}

// skillDoc is the definition of most test skills, and insideDoc the content
// of a file inside a test skill.
const (
	skillDoc  = "# Skill"
	insideDoc = "inside"
)

// noRedaction keeps every file as it is.
func noRedaction(doc []byte) ([]byte, error) { return doc, nil }

// archiveFiles reads an archive back as a map of paths to contents.
func archiveFiles(t *testing.T, archive []byte) map[string]string {
	t.Helper()

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	require.NoError(t, err)

	out := make(map[string]string)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		assert.Zero(t, hdr.ModTime.Unix(), "an entry has no time")
		assert.Zero(t, hdr.Uid)
		assert.Empty(t, hdr.Uname)

		content, err := io.ReadAll(tr)
		require.NoError(t, err)
		out[hdr.Name] = string(content)
	}

	return out
}

func TestCopyAndPackage(t *testing.T) {
	t.Run("a package holds the full folder, redacted", func(t *testing.T) {
		src := writeSkill(t, t.TempDir(), map[string]string{
			DefinitionFile:              "# Skill\nuse token SECRET\n",
			"references/rules.md":       "the rules",
			"scripts/lint.py":           "print('SECRET')",
			".git/HEAD":                 "ref: refs/heads/main",
			"nested/.git":               "gitdir: ../../.git/modules/nested",
			"examples/before-after.txt": "before and after",
		})

		dst := filepath.Join(t.TempDir(), "copy")
		info, err := Copy(src, dst)
		require.NoError(t, err)
		assert.False(t, info.Truncated)
		assert.True(t, strings.HasPrefix(info.Digest, "sha256:"))

		redact := func(doc []byte) ([]byte, error) {
			return bytes.ReplaceAll(doc, []byte("SECRET"), []byte("[REDACTED]")), nil
		}

		pkg, err := MakePackage(dst, redact)
		require.NoError(t, err)

		assert.Equal(t, "# Skill\nuse token [REDACTED]\n", string(pkg.Definition))
		assert.Equal(t, map[string]string{
			DefinitionFile:              "# Skill\nuse token [REDACTED]\n",
			"references/rules.md":       "the rules",
			"scripts/lint.py":           "print('[REDACTED]')",
			"examples/before-after.txt": "before and after",
		}, archiveFiles(t, pkg.Archive), "version-control folders and files are left out")
	})

	t.Run("the same content gives the same digests in every session", func(t *testing.T) {
		files := map[string]string{DefinitionFile: skillDoc, "a/b.md": "b", "c.md": "c"}
		first := writeSkill(t, t.TempDir(), files)
		second := writeSkill(t, t.TempDir(), files)

		dst1 := filepath.Join(t.TempDir(), "copy")
		dst2 := filepath.Join(t.TempDir(), "copy")

		info1, err := Copy(first, dst1)
		require.NoError(t, err)
		info2, err := Copy(second, dst2)
		require.NoError(t, err)
		assert.Equal(t, info1.Digest, info2.Digest)

		pkg1, err := MakePackage(dst1, noRedaction)
		require.NoError(t, err)
		pkg2, err := MakePackage(dst2, noRedaction)
		require.NoError(t, err)
		assert.Equal(t, pkg1.Archive, pkg2.Archive, "the archive is reproducible")
	})

	t.Run("a copy does not change when the folder changes", func(t *testing.T) {
		src := writeSkill(t, t.TempDir(), map[string]string{DefinitionFile: "first"})
		dst := filepath.Join(t.TempDir(), "copy")

		first, err := Copy(src, dst)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(src, DefinitionFile), []byte("second"), 0o600))

		again, err := Copy(src, dst)
		require.NoError(t, err)
		assert.Equal(t, first.Digest, again.Digest, "the first copy stands")

		live, err := ContentDigest(src)
		require.NoError(t, err)
		assert.NotEqual(t, first.Digest, live)

		require.NoError(t, MarkChanged(dst))
		loaded, err := Load(dst)
		require.NoError(t, err)
		assert.True(t, loaded.Changed)

		pkg, err := MakePackage(dst, noRedaction)
		require.NoError(t, err)
		assert.Equal(t, "first", string(pkg.Definition))
	})

	t.Run("a folder without SKILL.md is not read", func(t *testing.T) {
		src := writeSkill(t, t.TempDir(), map[string]string{"README.md": "not a skill"})

		_, err := Copy(src, filepath.Join(t.TempDir(), "copy"))
		assert.ErrorIs(t, err, ErrNotSkill)
	})

	t.Run("a link out of the folder is not followed", func(t *testing.T) {
		outside := writeSkill(t, t.TempDir(), map[string]string{"secret.txt": "private"})
		src := writeSkill(t, t.TempDir(), map[string]string{DefinitionFile: skillDoc, "docs/inside.md": insideDoc})
		require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(src, "secret.txt")))
		require.NoError(t, os.Symlink(outside, filepath.Join(src, "outside-dir")))
		require.NoError(t, os.Symlink(filepath.Join(src, "docs", "inside.md"), filepath.Join(src, "alias.md")))

		dst := filepath.Join(t.TempDir(), "copy")
		info, err := Copy(src, dst)
		require.NoError(t, err)
		assert.Equal(t, 2, info.SkippedLinks)

		pkg, err := MakePackage(dst, noRedaction)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			DefinitionFile:   skillDoc,
			"docs/inside.md": insideDoc,
			"alias.md":       insideDoc,
		}, archiveFiles(t, pkg.Archive))
	})

	t.Run("a skill folder that is itself a link is read", func(t *testing.T) {
		target := writeSkill(t, t.TempDir(), map[string]string{DefinitionFile: skillDoc})
		link := filepath.Join(t.TempDir(), "skill")
		require.NoError(t, os.Symlink(target, link))

		dst := filepath.Join(t.TempDir(), "copy")
		_, err := Copy(link, dst)
		require.NoError(t, err)

		pkg, err := MakePackage(dst, noRedaction)
		require.NoError(t, err)
		assert.Equal(t, skillDoc, string(pkg.Definition))
	})

	t.Run("a package over the limit keeps its definition only", func(t *testing.T) {
		big := make([]byte, MaxPackageSize+1024)
		_, err := rand.Read(big)
		require.NoError(t, err)

		src := writeSkill(t, t.TempDir(), map[string]string{DefinitionFile: skillDoc})
		require.NoError(t, os.WriteFile(filepath.Join(src, "model.bin"), big, 0o600))

		dst := filepath.Join(t.TempDir(), "copy")
		_, err = Copy(src, dst)
		require.NoError(t, err)

		pkg, err := MakePackage(dst, noRedaction)
		require.NoError(t, err)
		assert.True(t, pkg.Oversize)
		assert.Nil(t, pkg.Archive)
		assert.Equal(t, skillDoc, string(pkg.Definition))

		// The second push reads the stored result.
		again, err := MakePackage(dst, func([]byte) ([]byte, error) { return nil, errors.New("not called") })
		require.NoError(t, err)
		assert.Equal(t, pkg, again)
	})

	t.Run("a file that cannot be scanned fails the package", func(t *testing.T) {
		src := writeSkill(t, t.TempDir(), map[string]string{DefinitionFile: skillDoc})
		dst := filepath.Join(t.TempDir(), "copy")
		_, err := Copy(src, dst)
		require.NoError(t, err)

		_, err = MakePackage(dst, func([]byte) ([]byte, error) { return nil, errors.New("scanner down") })
		assert.Error(t, err)
	})

	t.Run("binary files are stored as they are", func(t *testing.T) {
		binary := string([]byte{0xff, 0xfe, 0x00, 'S', 'E', 'C', 'R', 'E', 'T'})
		src := writeSkill(t, t.TempDir(), map[string]string{DefinitionFile: skillDoc, "icon.bin": binary})
		dst := filepath.Join(t.TempDir(), "copy")
		_, err := Copy(src, dst)
		require.NoError(t, err)

		pkg, err := MakePackage(dst, func(doc []byte) ([]byte, error) {
			return bytes.ReplaceAll(doc, []byte("SECRET"), []byte("x")), nil
		})
		require.NoError(t, err)
		assert.Equal(t, binary, archiveFiles(t, pkg.Archive)["icon.bin"])
	})
}

func TestBaseDir(t *testing.T) {
	testCases := []struct {
		name   string
		text   string
		want   string
		wantOK bool
	}{
		{
			name:   "a Claude Code path",
			text:   "Base directory for this skill: /Users/me/.claude/skills/asd-ste100\n\n# Simplified Technical English",
			want:   "/Users/me/.claude/skills/asd-ste100",
			wantOK: true,
		},
		{
			name:   "an OpenCode file URL inside the tool output",
			text:   "<skill_content name=\"git-release\">\n# Skill: git-release\n\nBase directory for this skill: file:///home/me/.config/opencode/skills/git%20release\n",
			want:   "/home/me/.config/opencode/skills/git release",
			wantOK: true,
		},
		{name: "a relative path", text: "Base directory for this skill: skills/x", wantOK: false},
		{name: "no marker", text: "Launching skill: x", wantOK: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := BaseDir(tc.text)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
