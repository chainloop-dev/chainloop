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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// localSessionsDir is where local storage mode writes the sessions, under the
// default trace directory.
var localSessionsDir = filepath.Join(".git", "chainloop-trace", "local-sessions")

// gitConfig isolates the git commands of a workspace, and of the harness
// that runs in it, from the git configuration of the user who runs the suite.
// A global hooks path would hide the Trace git hooks, and commit signing could
// ask for a passphrase.
const gitConfig = `[user]
	name = Chainloop Trace E2E
	email = trace-e2e@chainloop.local
[commit]
	gpgsign = false
[tag]
	gpgsign = false
[init]
	defaultBranch = main
`

// Workspace is a throwaway git repository with Trace in local storage mode,
// and a local bare remote to push to.
type Workspace struct {
	// Dir is the repository.
	Dir string
	// Env is the environment of every command in the workspace, the harness
	// included: the chainloop binary under test first in PATH, and an
	// isolated git configuration.
	Env []string
	// Chainloop is the chainloop CLI under test.
	Chainloop string
}

// NewWorkspace creates a workspace under base for a scenario. It writes the
// seed files, runs `chainloop trace init --local` for the harness unless the
// scenario says no, commits the result and pushes it, so the session starts
// from a clean, pushed tree.
func NewWorkspace(ctx context.Context, base, chainloopBin string, h Harness, sc Scenario) (*Workspace, error) {
	repo := filepath.Join(base, "repo")
	remote := filepath.Join(base, "remote.git")
	gitConfigPath := filepath.Join(base, "gitconfig")

	if err := os.MkdirAll(repo, 0o750); err != nil {
		return nil, err
	}
	if err := os.WriteFile(gitConfigPath, []byte(gitConfig), 0o600); err != nil {
		return nil, err
	}

	w := &Workspace{
		Dir:       repo,
		Chainloop: chainloopBin,
		Env: append(os.Environ(),
			"PATH="+filepath.Dir(chainloopBin)+string(os.PathListSeparator)+os.Getenv("PATH"),
			"GIT_CONFIG_GLOBAL="+gitConfigPath,
			"GIT_CONFIG_NOSYSTEM=1",
			"CHAINLOOP_NO_PROMPT=1",
			"DO_NOT_TRACK=1",
		),
	}

	if _, err := w.run(ctx, base, "git", "init", "--bare", "--quiet", remote); err != nil {
		return nil, err
	}
	if _, err := w.Git(ctx, "init", "--quiet"); err != nil {
		return nil, err
	}

	seed := sc.Seed
	if sc.SeedUserHooks {
		hc := h.HookConfig()
		seed = maps.Clone(seed)
		seed[hc.Path] = hc.UserConfig
	}
	for name, content := range seed {
		path := filepath.Join(repo, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return nil, err
		}
	}

	if !sc.NoInit {
		if _, err := w.TraceInit(ctx, h); err != nil {
			return nil, err
		}
	}
	if sc.DisableSessionStart {
		if err := h.DisableSessionStart(repo); err != nil {
			return nil, err
		}
	}

	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "--quiet", "-m", "seed"},
		{"remote", "add", "origin", remote},
		{"push", "--quiet", "-u", "origin", "main"},
	} {
		if _, err := w.Git(ctx, args...); err != nil {
			return nil, err
		}
	}

	return w, nil
}

// TraceInit runs `chainloop trace init --local` for the harness in the
// repository.
func (w *Workspace) TraceInit(ctx context.Context, h Harness) (string, error) {
	return w.run(ctx, w.Dir, w.Chainloop, "trace", "init", "--local", h.TraceFlag())
}

// TraceUninstall runs `chainloop trace uninstall` in the repository.
func (w *Workspace) TraceUninstall(ctx context.Context) (string, error) {
	return w.run(ctx, w.Dir, w.Chainloop, "trace", "uninstall", "--yes")
}

// Git runs a git command in the repository and returns its output.
func (w *Workspace) Git(ctx context.Context, args ...string) (string, error) {
	return w.run(ctx, w.Dir, "git", args...)
}

func (w *Workspace) run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = w.Env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// StoredSession is one session that the pre-push hook stored in local storage
// mode.
type StoredSession struct {
	// Dir is the directory of the stored session.
	Dir string
	// Evidence is the AI coding session evidence.
	Evidence aicodingsession.Evidence
}

// Material returns the stored content of a material, by its digest.
func (s *StoredSession) Material(digest string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.Dir, "evidence", strings.ReplaceAll(digest, ":", "-")))
}

// HasStatement reports whether the stored session has its statement index.
func (s *StoredSession) HasStatement() bool {
	_, err := os.Stat(filepath.Join(s.Dir, "statement.json"))
	return err == nil
}

// StoredSessions reads the sessions that local storage mode stored in a
// checkout. Each session directory holds the AI coding session evidence among
// its materials.
func StoredSessions(checkout string) ([]StoredSession, error) {
	root := filepath.Join(checkout, localSessionsDir)
	dirs, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []StoredSession
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		dir := filepath.Join(root, d.Name())
		ev, err := findSessionEvidence(filepath.Join(dir, "evidence"))
		if err != nil {
			return nil, fmt.Errorf("stored session %s: %w", d.Name(), err)
		}
		out = append(out, StoredSession{Dir: dir, Evidence: *ev})
	}

	return out, nil
}

// findSessionEvidence finds the AI coding session evidence among the
// materials of a stored session. The other materials, for example spec
// sources, are not JSON or have another ID.
func findSessionEvidence(dir string) (*aicodingsession.Evidence, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, err
		}
		var ev aicodingsession.Evidence
		if json.Unmarshal(data, &ev) == nil && ev.ID == aicodingsession.EvidenceID {
			return &ev, nil
		}
	}
	return nil, errors.New("no AI coding session evidence")
}
