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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// commandTimeout limits a command that a check runs in the workspace.
const commandTimeout = 2 * time.Minute

// readHookConfig reads the hook configuration file of the harness.
func readHookConfig(s *Session) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.Workspace.Dir, s.Harness.HookConfig().Path))
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
	got, err := readHookConfig(s)
	if err != nil {
		return err
	}
	if string(got) != s.Harness.HookConfig().UserConfig {
		return fmt.Errorf("the hook configuration changed after trace run: %s", got)
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
	if _, err := hookMessage(s, "SessionStart"); err == nil {
		return errors.New("the session-start hook fired, so the scenario does not test a missed start")
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
