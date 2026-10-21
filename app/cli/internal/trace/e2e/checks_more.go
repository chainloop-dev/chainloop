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

package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/codex"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// commandTimeout limits a command that a check runs in the workspace.
const commandTimeout = 2 * time.Minute

// readHookFile reads a hook file of the harness. A missing file is empty.
func readHookFile(s *Session, path string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(s.Workspace.Dir, path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// readHookConfig reads the hook configuration of the harness: the file of
// the Chainloop hooks, and the file of the hook of the user when it is
// another file.
func readHookConfig(s *Session) ([]byte, error) {
	hc := s.Harness.HookConfig()
	out, err := readHookFile(s, hc.Path)
	if err != nil || hc.UserFile() == hc.Path {
		return out, err
	}
	user, err := readHookFile(s, hc.UserFile())
	if err != nil {
		return nil, err
	}
	return append(out, user...), nil
}

// checkHookInstallation checks that `trace init` installs the Chainloop
// hooks next to a hook of the user, that a second init changes nothing, and
// that `trace uninstall` deletes only the Chainloop hooks.
func checkHookInstallation(s *Session) error {
	hc := s.Harness.HookConfig()
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	installed, err := readHookConfig(s)
	if err != nil {
		return err
	}
	if !bytes.Contains(installed, []byte(hc.ChainloopMarker)) {
		return fmt.Errorf("trace init did not install the Chainloop hooks in %s", hc.Path)
	}
	if !bytes.Contains(installed, []byte(hc.UserMarker)) {
		return fmt.Errorf("trace init deleted the hook of the user from %s", hc.Path)
	}

	if _, err := s.Workspace.TraceInit(ctx, s.Harness); err != nil {
		return err
	}
	again, err := readHookConfig(s)
	if err != nil {
		return err
	}
	if !bytes.Equal(installed, again) {
		return fmt.Errorf("a second trace init changed %s", hc.Path)
	}

	if _, err := s.Workspace.TraceUninstall(ctx); err != nil {
		return err
	}
	after, err := readHookConfig(s)
	if err != nil {
		return err
	}
	if bytes.Contains(after, []byte(hc.ChainloopMarker)) {
		return fmt.Errorf("trace uninstall left Chainloop hooks in %s", hc.Path)
	}
	if !bytes.Contains(after, []byte(hc.UserMarker)) {
		return fmt.Errorf("trace uninstall deleted the hook of the user from %s", hc.Path)
	}
	return nil
}

// checkHookConfigRestored checks that the hook configuration is the same as
// before the wrapped session.
func checkHookConfigRestored(s *Session) error {
	hc := s.Harness.HookConfig()
	got, err := readHookFile(s, hc.UserFile())
	if err != nil {
		return err
	}
	if string(got) != hc.UserConfig {
		return fmt.Errorf("the hook configuration changed after trace run: %s", got)
	}
	if hc.UserFile() == hc.Path {
		return nil
	}
	if got, err = readHookFile(s, hc.Path); err != nil {
		return err
	}
	if bytes.Contains(got, []byte(hc.ChainloopMarker)) {
		return fmt.Errorf("trace run left Chainloop hooks in %s", hc.Path)
	}
	return nil
}

// checkGenericSessionStored checks that `trace run --local` stored the
// wrapped session as a generic session.
func checkGenericSessionStored(s *Session) error {
	if mode := evidence(s).Session.Mode; mode != aicodingsession.ModeGeneric {
		return fmt.Errorf("the stored session has mode %q, want %q", mode, aicodingsession.ModeGeneric)
	}
	return nil
}

// checkNoSessionStartHook makes sure that the session-start hook did not
// fire, so that the scenario tests a missed session start.
func checkNoSessionStartHook(s *Session) error {
	_, err := hookMessage(s, "SessionStart")
	switch {
	case err == nil:
		return errors.New("the session-start hook fired, so the scenario does not test a missed start")
	case errors.Is(err, errNotCovered):
		// A harness that reports no hook messages cannot show that the hook
		// did not fire.
		return err
	}
	return nil
}

// checkBothCommitsCredited checks that each of the two commits of a file
// credits the session in its trailer.
func checkBothCommitsCredited(s *Session) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	out, err := s.Workspace.run(ctx, s.Checkout, "git", "log", "-2", "--format=%B%x00")
	if err != nil {
		return err
	}
	sessionID := evidence(s).Session.ID
	for i, msg := range strings.Split(strings.TrimRight(strings.TrimSpace(out), "\x00"), "\x00") {
		if !slices.Contains(state.ParseSessionIDsFromTrailer(msg), sessionID) {
			return fmt.Errorf("commit %d of 2 does not credit the session: %q", 2-i, strings.TrimSpace(msg))
		}
	}
	return nil
}

// checkPastedImage checks that the pasted image is stored once as an image
// spec entry, and that the transcript holds a pointer in place of the image.
func checkPastedImage(s *Session) error {
	if evidence(s).Agent.Name == codex.Name {
		count := 0
		for _, e := range evidence(s).Spec {
			if e.Kind == aicodingsession.SpecKindImage {
				count++
				if !codexViewerPointer(evidence(s).RawSession["main"], e.Digest) {
					return errors.New("the Codex viewer message has no image material pointer")
				}
			}
		}
		if count != 1 {
			return fmt.Errorf("the repeated Codex paste produced %d image materials, want one", count)
		}
	}
	for _, e := range evidence(s).Spec {
		if e.Kind != aicodingsession.SpecKindImage || e.Title != "Pasted image 1" {
			continue
		}
		content, err := s.Stored.Material(e.Digest)
		if err != nil {
			return fmt.Errorf("read the image material: %w", err)
		}
		if _, err := png.Decode(bytes.NewReader(content)); err != nil {
			return fmt.Errorf("the image material is not a valid PNG: %w", err)
		}

		pasted := base64.StdEncoding.EncodeToString(pastedPNG())
		hasPointer := false
		for _, msg := range evidence(s).RawSession["main"] {
			if bytes.Contains(msg, []byte(pasted)) {
				return errors.New("the transcript still holds the pasted image inline")
			}
			if bytes.Contains(msg, []byte(pointer.Marker)) && bytes.Contains(msg, []byte(e.Digest)) {
				hasPointer = true
			}
		}
		if !hasPointer {
			return errors.New("the transcript has no pointer to the pasted image")
		}
		return nil
	}
	return errors.New(`the evidence has no image spec entry "Pasted image 1"`)
}

// checkMediaKept checks that the redaction did not change the image that the
// agent read. Its base64 holds a fake secret, which a redaction that scans
// media would replace.
func checkMediaKept(s *Session) error {
	found, err := storedContains(s, "/"+mediaSecret+"/")
	if err != nil {
		return err
	}
	if !found {
		return errors.New("the stored transcript does not hold the image that the agent read, unchanged")
	}
	return nil
}

// checkLineRanges checks that the evidence gives the lines of a file that the
// agent wrote as one AI range, at their positions in the pushed commit.
func checkLineRanges(path string, lines int) func(s *Session) error {
	return func(s *Session) error {
		cc := evidence(s).CodeChanges
		if cc == nil {
			return errors.New("the evidence has no code changes")
		}
		want := []aicodingsession.LineRange{{Start: 1, End: lines, Attribution: trace.AttributionAI}}
		for _, f := range cc.Files {
			if f.Path != path {
				continue
			}
			if !slices.Equal(f.LineRanges, want) {
				return fmt.Errorf("%s: line ranges %+v, want %+v", path, f.LineRanges, want)
			}
			return nil
		}
		return fmt.Errorf("the code changes do not list %s", path)
	}
}

// fileChange returns the change of a file in the evidence.
func fileChange(s *Session, path string) (*aicodingsession.FileChange, error) {
	cc := evidence(s).CodeChanges
	if cc == nil {
		return nil, errors.New("the evidence has no code changes")
	}
	for i := range cc.Files {
		if cc.Files[i].Path == path {
			return &cc.Files[i], nil
		}
	}
	return nil, fmt.Errorf("the code changes do not list %s", path)
}

// headInSession checks that the last commit of the checkout is in the commit
// list of the session. Each entry of the list is the SHA and the subject of a
// commit.
func headInSession(s *Session) error {
	gc := evidence(s).GitContext
	if gc != nil && slices.ContainsFunc(gc.Commits, func(c string) bool {
		sha, _, _ := strings.Cut(c, " ")
		return sha == s.HeadSHA
	}) {
		return nil
	}
	return fmt.Errorf("the commit list of the session does not hold the last commit %s", s.HeadSHA)
}

// checkNetContribution checks that the line that the human commit replaced
// no longer counts as AI, that each line counts once, and that the file that
// the session created stays created.
func checkNetContribution(s *Session) error {
	f, err := fileChange(s, greetingFile)
	if err != nil {
		return err
	}
	if f.AILinesAdded != 2 || f.HumanLinesAdded != 1 {
		return fmt.Errorf("%s: %d AI lines and %d human lines added, want 2 and 1", greetingFile, f.AILinesAdded, f.HumanLinesAdded)
	}
	if f.LinesAdded != f.AILinesAdded+f.HumanLinesAdded {
		return fmt.Errorf("%s: %d lines added, want the sum of the AI and human lines", greetingFile, f.LinesAdded)
	}
	if f.Status != tracegit.StatusCreated {
		return fmt.Errorf("%s: status %q, want created", greetingFile, f.Status)
	}
	return nil
}

// checkSessionUpdated checks that the push after the human commit stored the
// session again, with the human commit in its commit list and the replacing
// line as a human line.
func checkSessionUpdated(s *Session) error {
	if err := headInSession(s); err != nil {
		return err
	}
	f, err := fileChange(s, greetingFile)
	if err != nil {
		return err
	}
	if f.HumanLinesAdded != 1 {
		return fmt.Errorf("%s: %d human lines added, want the 1 replacing line", greetingFile, f.HumanLinesAdded)
	}
	return nil
}

// checkAmended checks that the session follows its amended commit, with the
// lines of the agent as AI lines and the line that the amend added as a human
// line.
func checkAmended(s *Session) error {
	if err := headInSession(s); err != nil {
		return err
	}
	f, err := fileChange(s, greetingFile)
	if err != nil {
		return err
	}
	if f.AILinesAdded != 3 || f.HumanLinesAdded != 1 {
		return fmt.Errorf("%s: %d AI lines and %d human lines added, want 3 and 1", greetingFile, f.AILinesAdded, f.HumanLinesAdded)
	}
	return nil
}

// checkRenamed checks that the lines of the agent stay AI lines after a later
// commit renamed their file. The evidence lists the file under its path in
// the commit of the session or under its new path.
func checkRenamed(s *Session) error {
	f, err := fileChange(s, greetingFile)
	if err != nil {
		if f, err = fileChange(s, renamedFile); err != nil {
			return err
		}
	}
	if f.AILinesAdded != 3 || f.HumanLinesAdded != 0 {
		return fmt.Errorf("%s: %d AI lines and %d human lines added, want 3 and 0", f.Path, f.AILinesAdded, f.HumanLinesAdded)
	}
	return nil
}
