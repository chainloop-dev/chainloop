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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
)

// maxSkillEntries is the limit of skill entries for each session. It is apart
// from the limit of the spec sources that the agent writes, so that many
// skills cannot push out the task and its spec.
const maxSkillEntries = 25

// specAnnotationSkillContent tells the two materials of a skill apart.
const specAnnotationSkillContent = "chainloop.spec.skill.content"

// The values of specAnnotationSkillContent.
const (
	skillContentDefinition = "definition"
	skillContentPackage    = "package"
)

// captureSkillLoads copies the folder of each skill that the session loaded
// since the last hook, into the trace state. A hook runs it, so that the
// evidence holds the skill as it was when it ran, also when its files change
// before the push.
//
// A copy is never replaced. A later use that finds different content only
// marks the copy, and the push records a warning.
//
// Best effort throughout: a skill that cannot be copied here is copied at the
// push, and nothing here may slow down or fail the agent's tool call.
func captureSkillLoads(provider trace.Provider, store *state.Store, input *trace.HookInput, log zerolog.Logger) {
	tracker, ok := provider.(trace.SkillTracker)
	if !ok {
		return
	}

	for _, dir := range tracker.NewSkillLoads(store, input) {
		copySkill(store, input.SessionID, dir, log)
	}
}

// copySkill makes the copy of a skill folder for a session, or checks an
// existing copy against the folder.
func copySkill(store *state.Store, sessionID, dir string, log zerolog.Logger) {
	dst := store.SkillCopyDir(sessionID, dir)

	info, err := skill.Load(dst)
	if err != nil {
		if _, err := skill.Copy(dir, dst); err != nil {
			log.Debug().Err(err).Str("session_id", sessionID).Msg("could not copy a skill folder; the push copies it")
		}

		return
	}

	if info.Changed {
		return
	}

	live, err := skill.ContentDigest(dir)
	if err != nil || live == info.Digest {
		return
	}

	if err := skill.MarkChanged(dst); err != nil {
		log.Debug().Err(err).Str("session_id", sessionID).Msg("could not mark a changed skill folder")
	}
}

// sessionSkill is one used skill, ready to be stored: its counts, where it
// came from, and the copy of its folder.
type sessionSkill struct {
	use     trace.SkillUse
	source  string
	copyDir string
}

// readSessionSkills returns the skills that a session used, each with a copy
// of its folder, and a warning for each skill that is left out or not as it
// was when it ran.
//
// A provider without skill support gives no skills and no warning. A failure
// to read the skills is never a reason to lose the session.
func readSessionSkills(provider trace.Provider, store *state.Store, opts *trace.ParseOpts, repoRoot string, log zerolog.Logger) ([]sessionSkill, []string) {
	tracker, ok := provider.(trace.SkillTracker)
	if !ok {
		return nil, nil
	}

	uses, warnings, err := tracker.SessionSkills(opts)
	if err != nil {
		// The error stays in the local log: its text carries local paths.
		log.Warn().Err(err).Str("session", opts.SessionID).Msg("could not read the skills that the session used; the session is attested without them")
		return nil, []string{"the skills that the session used were not recorded: the session data could not be read"}
	}

	// A skill with no folder, such as a skill built into the agent, has
	// nothing to store. It is left out with a warning in the local log only.
	uses = slices.DeleteFunc(uses, func(use trace.SkillUse) bool {
		if use.Dir != "" {
			return false
		}

		log.Warn().Str("session", opts.SessionID).Str("skill", use.Name).Msg("a skill that the session used has no skill folder, so it is not recorded")

		return true
	})

	if len(uses) > maxSkillEntries {
		// The skills that the session used first are kept.
		warnings = append(warnings, fmt.Sprintf("%d skill entries beyond the first %d were dropped", len(uses)-maxSkillEntries, maxSkillEntries))
		uses = uses[:maxSkillEntries]
	}

	roots := tracker.SkillRoots(repoRoot)

	skills := make([]sessionSkill, 0, len(uses))
	for _, use := range uses {
		dst := store.SkillCopyDir(opts.SessionID, use.Dir)
		info, err := skill.Load(dst)
		switch {
		case err != nil:
			// No hook ran after the use, so the folder is copied now. It can
			// differ from the folder that the session used.
			if info, err = skill.Copy(use.Dir, dst); err != nil {
				log.Warn().Err(err).Str("session", opts.SessionID).Str("skill", use.Name).Msg("could not copy a skill folder")
				warnings = append(warnings, fmt.Sprintf("skill %q was not recorded: its folder could not be read", use.Name))
				continue
			}
			warnings = append(warnings, fmt.Sprintf("skill %q was copied at push time, not when the session used it", use.Name))
		case info.Changed:
			warnings = append(warnings, fmt.Sprintf("skill %q changed after its first use; the evidence holds the skill as it was at the first use", use.Name))
		}

		if info.SkippedLinks > 0 {
			warnings = append(warnings, fmt.Sprintf("skill %q: %d links out of the skill folder, or to a folder, were left out", use.Name, info.SkippedLinks))
		}

		skills = append(skills, sessionSkill{
			use:     use,
			source:  skill.Source(use.Name, use.Dir, roots),
			copyDir: dst,
		})
	}

	return skills, warnings
}

// attachSkills stores the materials of each used skill and returns the spec
// entries that the session material records for them.
//
// Each skill gives two EVIDENCE materials: its redacted SKILL.md file, which
// the digest of the entry points to so that a viewer can show it, and a
// reproducible archive of the full redacted folder, which the metadata points
// to as the download. A skill that cannot be stored is left out with a
// warning, as a spec source is.
func attachSkills(ctx context.Context, adder specMaterialAdder, redactor *specRedactor, names *materials.NameAllocator, sessionID string, skills []sessionSkill, log zerolog.Logger) (entries []aicodingsession.SpecEntry, warnings []string) {
	if len(skills) == 0 {
		return nil, nil
	}

	tmpDir, err := os.MkdirTemp("", "chainloop-trace-skill-*")
	if err != nil {
		return nil, []string{fmt.Sprintf("%d skill entries were not recorded: %v", len(skills), err)}
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	for i, s := range skills {
		entry, warning, err := storeSkill(ctx, adder, redactor, names, filepath.Join(tmpDir, strconv.Itoa(i)), sessionID, s)
		if err != nil {
			// The error stays in the local log. The warning goes into the
			// uploaded evidence, and error text can carry local paths.
			log.Warn().Err(err).Str("session", sessionID).Str("skill", s.use.Name).Msg("could not record a skill entry")
			warnings = append(warnings, fmt.Sprintf("skill %q was not recorded", s.use.Name))

			continue
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}

		entries = append(entries, entry)
	}

	return entries, warnings
}

// storeSkill adds the materials of one skill and returns its spec entry. A
// package that cannot be stored costs the package only, with a warning: the
// definition and the counts still go into the evidence.
func storeSkill(ctx context.Context, adder specMaterialAdder, redactor *specRedactor, names *materials.NameAllocator, dir, sessionID string, s sessionSkill) (aicodingsession.SpecEntry, string, error) {
	pkg, err := skill.MakePackage(s.copyDir, func(doc []byte) ([]byte, error) {
		return redactor.Redact(ctx, doc)
	})
	if err != nil {
		return aicodingsession.SpecEntry{}, "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return aicodingsession.SpecEntry{}, "", fmt.Errorf("staging the skill: %w", err)
	}

	// spec-<first 6 characters of the session ID>-skill-<skill name>. The
	// allocator makes each name unique within the attestation.
	stem := materialStem(s.use.Name, "skill")
	base := specMaterialPrefix(sessionID) + "skill-" + stem
	annotations := func(content string) map[string]string {
		return map[string]string{
			specAnnotationSession:      sessionID,
			specAnnotationKind:         aicodingsession.SpecKindSkill,
			specAnnotationTitle:        s.use.Name,
			specAnnotationSkillContent: content,
		}
	}

	definitionPath := filepath.Join(dir, skill.DefinitionFile)
	if err := os.WriteFile(definitionPath, pkg.Definition, 0o600); err != nil {
		return aicodingsession.SpecEntry{}, "", fmt.Errorf("staging the skill: %w", err)
	}

	digest, err := adder.AddMaterial(ctx, names.AllocateNamed(base), definitionPath, specMaterialKind, annotations(skillContentDefinition))
	if err != nil {
		return aicodingsession.SpecEntry{}, "", err
	}

	var packageDigest, warning string
	if pkg.Oversize {
		warning = fmt.Sprintf("the package of skill %q is larger than %d MB and was not uploaded", s.use.Name, skill.MaxPackageSize>>20)
	} else {
		// The archive is stored under the name of the skill.
		archivePath := filepath.Join(dir, stem+".tar.gz")
		if err = os.WriteFile(archivePath, pkg.Archive, 0o600); err == nil {
			packageDigest, err = adder.AddMaterial(ctx, names.AllocateNamed(base+"-pkg"), archivePath, specMaterialKind, annotations(skillContentPackage))
		}
		if err != nil {
			warning = fmt.Sprintf("the package of skill %q was not uploaded", s.use.Name)
		}
	}

	capturedAt := s.use.FirstUsedAt
	if capturedAt == "" {
		capturedAt = state.NowTimestamp()
	}

	return aicodingsession.SpecEntry{
		Kind:       aicodingsession.SpecKindSkill,
		Title:      s.use.Name,
		Digest:     digest,
		CapturedAt: capturedAt,
		Metadata: &aicodingsession.SpecMetadata{
			Source:          s.source,
			PackageDigest:   packageDigest,
			InvocationCount: s.use.ByModel + s.use.ByUser,
			ByModel:         s.use.ByModel,
			ByUser:          s.use.ByUser,
			InSubagents:     s.use.InSubagents,
		},
	}, warning, nil
}
