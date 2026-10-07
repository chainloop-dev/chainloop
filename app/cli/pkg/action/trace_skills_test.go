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

package action

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	skillSessionID = "7412a0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"
	skillDoc       = "# Skill"
	skillTenOClock = "2026-10-07T10:00:00Z"
	steSkillName   = "asd-ste100"
)

// skillProvider is a provider that records skills, with uses and loads set by
// the test.
type skillProvider struct {
	trace.Provider

	uses  []trace.SkillUse
	loads []string
	err   error
	roots skill.Roots
}

func (p *skillProvider) NewSkillLoads(_ *state.Store, _ *trace.HookInput) []string {
	return p.loads
}

func (p *skillProvider) SessionSkills(_ *trace.ParseOpts) ([]trace.SkillUse, []string, error) {
	return p.uses, nil, p.err
}

func (p *skillProvider) SkillRoots(_ string) skill.Roots {
	return p.roots
}

// noSkillProvider is a provider without skill support, such as Cursor.
type noSkillProvider struct {
	trace.Provider
}

func makeSkill(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	return dir
}

func untar(t *testing.T, archive string) map[string]string {
	t.Helper()

	gz, err := gzip.NewReader(bytes.NewReader([]byte(archive)))
	require.NoError(t, err)

	out := make(map[string]string)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		require.NoError(t, err)

		content, err := io.ReadAll(tr)
		require.NoError(t, err)
		out[hdr.Name] = string(content)
	}
}

func TestCaptureSkillLoads(t *testing.T) {
	t.Run("a skill file that changes after the use does not change the package", func(t *testing.T) {
		store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
		dir := makeSkill(t, map[string]string{skill.DefinitionFile: "as it ran"})
		provider := &skillProvider{
			loads: []string{dir},
			uses:  []trace.SkillUse{{Name: steSkillName, Dir: dir, FirstUsedAt: skillTenOClock, ByModel: 2}},
		}
		input := &trace.HookInput{SessionID: skillSessionID}

		captureSkillLoads(provider, store, input, zerolog.Nop())
		require.NoError(t, os.WriteFile(filepath.Join(dir, skill.DefinitionFile), []byte("changed later"), 0o600))
		// A later use finds the changed folder.
		captureSkillLoads(provider, store, input, zerolog.Nop())

		skills, warnings := readSessionSkills(provider, store, &trace.ParseOpts{SessionID: skillSessionID}, t.TempDir(), zerolog.Nop())
		require.Len(t, skills, 1)
		assert.Equal(t, []string{`skill "asd-ste100" changed after its first use; the evidence holds the skill as it was at the first use`}, warnings)

		adder := &fakeMaterialAdder{}
		_, _ = attachSkills(context.Background(), adder, newSpecRedactor(t.TempDir()), materials.NewNameAllocator(nil), skillSessionID, skills, zerolog.Nop())
		require.Len(t, adder.added, 2)
		assert.Equal(t, "as it ran", adder.added[0].content)
	})

	t.Run("a provider without skill support copies nothing", func(t *testing.T) {
		store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
		captureSkillLoads(&noSkillProvider{}, store, &trace.HookInput{SessionID: skillSessionID}, zerolog.Nop())

		skills, warnings := readSessionSkills(&noSkillProvider{}, store, &trace.ParseOpts{SessionID: skillSessionID}, t.TempDir(), zerolog.Nop())
		assert.Empty(t, skills)
		assert.Empty(t, warnings, "no skill entry means not recorded, with no warning")
	})
}

func TestReadSessionSkills(t *testing.T) {
	t.Run("a skill that no hook copied is copied at push time, with a warning", func(t *testing.T) {
		store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
		dir := makeSkill(t, map[string]string{skill.DefinitionFile: skillDoc})
		provider := &skillProvider{uses: []trace.SkillUse{{Name: steSkillName, Dir: dir, ByUser: 1}}}

		skills, warnings := readSessionSkills(provider, store, &trace.ParseOpts{SessionID: skillSessionID}, t.TempDir(), zerolog.Nop())
		require.Len(t, skills, 1)
		assert.Equal(t, []string{`skill "asd-ste100" was copied at push time, not when the session used it`}, warnings)
	})

	t.Run("a missing folder gives a warning and the other skills", func(t *testing.T) {
		store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
		dir := makeSkill(t, map[string]string{skill.DefinitionFile: skillDoc})
		provider := &skillProvider{uses: []trace.SkillUse{
			{Name: "gone", Dir: filepath.Join(t.TempDir(), "gone"), ByModel: 1},
			{Name: "unknown-dir", ByModel: 1},
			{Name: steSkillName, Dir: dir, ByModel: 1},
		}}
		captureSkillLoads(&skillProvider{loads: []string{dir}}, store, &trace.HookInput{SessionID: skillSessionID}, zerolog.Nop())

		skills, warnings := readSessionSkills(provider, store, &trace.ParseOpts{SessionID: skillSessionID}, t.TempDir(), zerolog.Nop())
		require.Len(t, skills, 1)
		assert.Equal(t, steSkillName, skills[0].use.Name)
		assert.Equal(t, []string{
			`skill "gone" was not recorded: its folder could not be read`,
			`skill "unknown-dir" was not recorded: the session data does not tell its folder`,
		}, warnings)
		for _, w := range warnings {
			assert.NotContains(t, w, string(filepath.Separator)+"gone", "a warning holds no local path")
		}
	})

	t.Run("skills beyond the limit are dropped, the first ones kept", func(t *testing.T) {
		store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
		dir := makeSkill(t, map[string]string{skill.DefinitionFile: skillDoc})
		captureSkillLoads(&skillProvider{loads: []string{dir}}, store, &trace.HookInput{SessionID: skillSessionID}, zerolog.Nop())

		uses := make([]trace.SkillUse, 0, maxSkillEntries+2)
		for i := range maxSkillEntries + 2 {
			uses = append(uses, trace.SkillUse{Name: "skill-" + string(rune('a'+i)), Dir: dir, ByModel: 1})
		}

		skills, warnings := readSessionSkills(&skillProvider{uses: uses}, store, &trace.ParseOpts{SessionID: skillSessionID}, t.TempDir(), zerolog.Nop())
		require.Len(t, skills, maxSkillEntries)
		assert.Equal(t, "skill-a", skills[0].use.Name)
		assert.Contains(t, warnings, "2 skill entries beyond the first 25 were dropped")
	})

	t.Run("a session data failure costs the skills only", func(t *testing.T) {
		store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
		skills, warnings := readSessionSkills(&skillProvider{err: errors.New("/home/me/x: no such file")}, store, &trace.ParseOpts{SessionID: skillSessionID}, t.TempDir(), zerolog.Nop())
		assert.Empty(t, skills)
		assert.Equal(t, []string{"the skills that the session used were not recorded: the session data could not be read"}, warnings)
	})

	t.Run("the source comes from the provider roots", func(t *testing.T) {
		store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
		dir := makeSkill(t, map[string]string{skill.DefinitionFile: skillDoc})
		captureSkillLoads(&skillProvider{loads: []string{dir}}, store, &trace.HookInput{SessionID: skillSessionID}, zerolog.Nop())

		provider := &skillProvider{
			uses:  []trace.SkillUse{{Name: steSkillName, Dir: dir, ByModel: 1}},
			roots: skill.Roots{User: []string{filepath.Dir(dir)}},
		}

		skills, _ := readSessionSkills(provider, store, &trace.ParseOpts{SessionID: skillSessionID}, t.TempDir(), zerolog.Nop())
		require.Len(t, skills, 1)
		assert.Equal(t, aicodingsession.SkillSourceUser, skills[0].source)
	})
}

func TestAttachSkills(t *testing.T) {
	// Assembled from fragments so that nothing credential-shaped is committed.
	const pat = "ghp_erOZlZv0B1e3amrQ" + "ugdwZ8Ro2W4kDql9WPTf"

	stored := func(t *testing.T, files map[string]string, use trace.SkillUse) sessionSkill {
		t.Helper()

		store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
		dir := makeSkill(t, files)
		dst := store.SkillCopyDir(skillSessionID, dir)
		_, err := skill.Copy(dir, dst)
		require.NoError(t, err)

		use.Dir = dir

		return sessionSkill{use: use, source: aicodingsession.SkillSourceUser, copyDir: dst}
	}

	t.Run("each skill gives a definition, a package and an entry", func(t *testing.T) {
		s := stored(t, map[string]string{
			skill.DefinitionFile:  "# STE\ntoken " + pat + "\n",
			"scripts/ste-lint.py": "print('" + pat + "')",
		}, trace.SkillUse{Name: steSkillName, FirstUsedAt: "2026-10-07T10:14:51Z", ByModel: 2, ByUser: 1, InSubagents: 1})

		adder := &fakeMaterialAdder{}
		entries, warnings := attachSkills(context.Background(), adder, newSpecRedactor(t.TempDir()), materials.NewNameAllocator(nil), skillSessionID, []sessionSkill{s}, zerolog.Nop())
		assert.Empty(t, warnings)
		require.Len(t, adder.added, 2)

		definition, pkg := adder.added[0], adder.added[1]
		assert.Equal(t, "spec-7412a0-skill-asd-ste100", definition.name)
		assert.Equal(t, skill.DefinitionFile, definition.fileName)
		assert.Equal(t, "EVIDENCE", definition.kind)
		assert.NotContains(t, definition.content, pat, "the definition is redacted")
		assert.Equal(t, map[string]string{
			specAnnotationSession:      skillSessionID,
			specAnnotationKind:         aicodingsession.SpecKindSkill,
			specAnnotationTitle:        steSkillName,
			specAnnotationSkillContent: skillContentDefinition,
		}, definition.annotations)

		assert.Equal(t, "spec-7412a0-skill-asd-ste100-pkg", pkg.name)
		assert.Equal(t, "asd-ste100.tar.gz", pkg.fileName)
		assert.Equal(t, "EVIDENCE", pkg.kind)
		assert.Equal(t, "package", pkg.annotations["chainloop.spec.skill.content"])

		files := untar(t, pkg.content)
		assert.Equal(t, definition.content, files[skill.DefinitionFile], "the definition and the packaged SKILL.md are the same bytes")
		assert.NotContains(t, files["scripts/ste-lint.py"], pat, "each text file is redacted")

		assert.Equal(t, []aicodingsession.SpecEntry{{
			Kind:       aicodingsession.SpecKindSkill,
			Title:      steSkillName,
			Digest:     "sha256:spec-7412a0-skill-asd-ste100",
			CapturedAt: "2026-10-07T10:14:51Z",
			Metadata: &aicodingsession.SpecMetadata{
				Source:          aicodingsession.SkillSourceUser,
				PackageDigest:   "sha256:spec-7412a0-skill-asd-ste100-pkg",
				InvocationCount: 3, ByModel: 2, ByUser: 1, InSubagents: 1,
			},
		}}, entries, "no role, description or source address")
	})

	t.Run("a package over the limit keeps the definition and the counts", func(t *testing.T) {
		big := make([]byte, skill.MaxPackageSize+1024)
		_, err := rand.Read(big)
		require.NoError(t, err)

		s := stored(t, map[string]string{skill.DefinitionFile: "# Big", "model.bin": string(big)}, trace.SkillUse{Name: "big", FirstUsedAt: skillTenOClock, ByModel: 1})

		adder := &fakeMaterialAdder{}
		entries, warnings := attachSkills(context.Background(), adder, newSpecRedactor(t.TempDir()), materials.NewNameAllocator(nil), skillSessionID, []sessionSkill{s}, zerolog.Nop())
		assert.Equal(t, []string{`the package of skill "big" is larger than 5 MB and was not uploaded`}, warnings)
		require.Len(t, adder.added, 1)
		require.Len(t, entries, 1)
		assert.Empty(t, entries[0].Metadata.PackageDigest)
		assert.Equal(t, 1, entries[0].Metadata.InvocationCount)
	})

	t.Run("a package that cannot be stored costs the package only", func(t *testing.T) {
		s := stored(t, map[string]string{skill.DefinitionFile: skillDoc}, trace.SkillUse{Name: steSkillName, FirstUsedAt: skillTenOClock, ByModel: 1})

		adder := &fakeMaterialAdder{failOn: map[string]bool{"spec-7412a0-skill-asd-ste100-pkg": true}}
		entries, warnings := attachSkills(context.Background(), adder, newSpecRedactor(t.TempDir()), materials.NewNameAllocator(nil), skillSessionID, []sessionSkill{s}, zerolog.Nop())
		assert.Equal(t, []string{`the package of skill "asd-ste100" was not uploaded`}, warnings)
		require.Len(t, entries, 1)
		assert.Empty(t, entries[0].Metadata.PackageDigest)
	})

	t.Run("a definition that cannot be stored leaves the skill out", func(t *testing.T) {
		s := stored(t, map[string]string{skill.DefinitionFile: skillDoc}, trace.SkillUse{Name: steSkillName, FirstUsedAt: skillTenOClock, ByModel: 1})

		adder := &fakeMaterialAdder{failOn: map[string]bool{"spec-7412a0-skill-asd-ste100": true}}
		entries, warnings := attachSkills(context.Background(), adder, newSpecRedactor(t.TempDir()), materials.NewNameAllocator(nil), skillSessionID, []sessionSkill{s}, zerolog.Nop())
		assert.Empty(t, entries)
		assert.Equal(t, []string{`skill "asd-ste100" was not recorded`}, warnings)
	})

	t.Run("a plugin skill gets a valid material name", func(t *testing.T) {
		s := stored(t, map[string]string{skill.DefinitionFile: skillDoc}, trace.SkillUse{Name: "superpowers:brainstorming", FirstUsedAt: skillTenOClock, ByModel: 1})

		adder := &fakeMaterialAdder{}
		_, _ = attachSkills(context.Background(), adder, newSpecRedactor(t.TempDir()), materials.NewNameAllocator(nil), skillSessionID, []sessionSkill{s}, zerolog.Nop())
		require.Len(t, adder.added, 2)
		assert.Equal(t, "spec-7412a0-skill-superpowers-brainstorming", adder.added[0].name)
		assert.Equal(t, "superpowers-brainstorming.tar.gz", adder.added[1].fileName)
		assert.Equal(t, "superpowers:brainstorming", adder.added[0].annotations["chainloop.spec.title"])
	})
}
