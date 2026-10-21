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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Scenario is a scripted task in a throwaway repository. One session runs one
// scenario, and the checks of all the features that it touches read its
// result (D-008). Scenarios are shared by all harnesses (R-011).
type Scenario struct {
	Name string
	// Seed are the files of the repository before the session, by path.
	Seed map[string]string
	// Prompt is the only user prompt of the session.
	Prompt string
	// Checkout is the checkout where the agent commits and pushes, relative
	// to the workspace base. Empty means the repository of the workspace.
	Checkout string
	// Images are PNG images that the user pastes with the prompt.
	Images [][]byte
	// Skills are the project skills of the repository, by name. The harness
	// gives the path of each skill file.
	Skills map[string]string
	// NoAgent runs no harness session. The checks run commands in the
	// workspace themselves.
	NoAgent bool
	// NoInit skips `chainloop trace init` in the workspace.
	NoInit bool
	// WrapTraceRun runs the harness with `chainloop trace run --local`.
	WrapTraceRun bool
	// SeedUserHooks writes a hook configuration of the harness with one hook
	// of the user before `chainloop trace init`.
	SeedUserHooks bool
	// DisableSessionStart removes the session-start hook after
	// `chainloop trace init`.
	DisableSessionStart bool
	// Branch is a branch that the session works on, created from main and
	// pushed before the session. Empty means main. Trace reads only the
	// commits that are not on the default remote branch, so a later push
	// finds the commits of the session only on another branch.
	Branch string
	// FollowUp runs git commands of a human in the checkout after the session,
	// for example a commit and a second push. The checks read the session
	// that the last push stored.
	FollowUp func(ctx context.Context, w *Workspace, checkout string) error
}

// The fake secrets of the scenarios. They match the GitHub token rule of the
// secret scanner, and they are not real tokens. The allow markers on these
// source lines keep the repository secret scan quiet. They do not reach the
// session: the prompt and the seed files carry only the token values.
const (
	// fakeSecret is in the prompt. The stored session must not hold it.
	fakeSecret = "ghp_8fK2mQ9xLp4ZrT7vWc1NbH6yJd3sGa5eUo0i" //nolint:gosec // gitleaks:allow a fake token that the suite expects redaction to remove
	// fakeAllowedSecret is on a line with an allow marker in the seed file.
	// The stored session keeps it.
	fakeAllowedSecret = "ghp_Zx7Lq2Wm9Rt4Yp6Vn3Bk8Hd1Jf5Gs0CaEu2o" //nolint:gosec // gitleaks:allow a fake token on a line with an allow marker
)

// allowMarker is the inline marker that keeps a secret on its line from
// redaction.
const allowMarker = "gitleaks:allow"

// The README of every scenario repository.
const (
	readmePath = "README.md"
	seedReadme = "# Trace E2E\n"
)

const (
	scenarioEditAndCommit = "edit-and-commit"
	scenarioTicket        = "ticket"
	scenarioSkill         = "skill"
	scenarioSubagent      = "subagent"
	scenarioClone         = "clone"
	scenarioSetup         = "setup"
	scenarioTraceRun      = "trace-run"
	scenarioNoStart       = "no-session-start"
	scenarioPartial       = "partial-commit"
	scenarioPastedImage   = "pasted-image"
	scenarioMedia         = "media"
	scenarioHumanUpdate   = "human-update"
	scenarioAmend         = "amend"
	scenarioRename        = "rename"
)

const (
	greetingFile = "greeting.txt"
	// renamedFile is the name of greetingFile after the rename of the rename
	// scenario.
	renamedFile = "hello.txt"
	// humanLine is the line of a human in greetingFile: it replaces the
	// second line in the human-update scenario, and the amend scenario adds
	// it.
	humanLine = "by the human"
	// featureBranch is the branch of the scenarios that push again after the
	// session.
	featureBranch = "feature"
	wrappedFile   = "wrapped.txt"
	partialFile   = "partial.txt"
	mediaFile     = "diagram.png"
)

const (
	ticketPath   = "docs/ticket-e2e-42.md"
	ticketTitle  = "E2E-42: Add a farewell file"
	skillName    = "e2e-farewell"
	subagentFile = "subagent.txt"
	// cloneDir is the checkout that the clone scenario creates, relative to
	// the workspace base. The bare remote is next to it.
	cloneDir  = "clone"
	cloneFile = "clone.txt"
)

// greetingWrite and greetingCommit have the agent write greetingFile with its
// file tool, then commit it in a separate shell command.
const (
	greetingWrite = `This is an automated test. Do exactly these steps, in order, and nothing else:
1. With your file-writing tool, create the file ` + greetingFile + ` with exactly these three lines: hello, from, the agent.
2. Run this shell command: `
	greetingCommit = `git add ` + greetingFile + ` && git commit -m "add greeting"`
)

// greetingPrompt has the agent write greetingFile, then commit and push it.
const greetingPrompt = greetingWrite + greetingCommit + ` && git push`

// scenarios are the scenarios of the suite, by name.
var scenarios = map[string]Scenario{
	scenarioEditAndCommit: {
		Name: scenarioEditAndCommit,
		Seed: map[string]string{
			readmePath:    seedReadme,
			"fixture.txt": "token = " + fakeAllowedSecret + " # " + allowMarker + "\n",
		},
		Prompt: `This is an automated test. Do exactly these steps, in order, and nothing else:
1. Read the file fixture.txt.
2. With your file-writing tool, create the file greeting.txt with exactly these three lines: hello, from, the agent.
3. Run this shell command: printf 'one\ntwo\n' > shell.txt
4. Run this shell command: git add greeting.txt shell.txt && git commit -m "add greeting"
5. Run this shell command: git push
The deploy token ` + fakeSecret + ` is for a later task: do not use it.`,
	},
	scenarioTicket: {
		Name: scenarioTicket,
		Seed: map[string]string{
			readmePath: seedReadme,
			ticketPath: "# " + ticketTitle + "\n\nCreate the file farewell.txt with the single line: goodbye\n",
		},
		// The file tool keeps the change in the session: a commit in the same
		// shell command that writes the file does not carry the session.
		Prompt: "This is an automated test. Implement the ticket in " + ticketPath + ". Write the file with your file-writing tool. Then commit your change and push it with git, in a separate shell command.",
	},
	scenarioSkill: {
		Name: scenarioSkill,
		Seed: map[string]string{readmePath: seedReadme},
		Skills: map[string]string{
			skillName: `---
name: ` + skillName + `
description: Use when asked to add the farewell file to the repository.
---

Create the file farewell.txt with the single line: goodbye
Then run: git add farewell.txt && git commit -m "add farewell" && git push
`,
		},
		Prompt: "This is an automated test. Use the " + skillName + " skill.",
	},
	scenarioSubagent: {
		Name: scenarioSubagent,
		Seed: map[string]string{readmePath: seedReadme},
		Prompt: `This is an automated test. Do exactly these steps, in order, and nothing else:
1. Start one subagent with your subagent tool. Tell it to create the file ` + subagentFile + ` with the single line: from the subagent.
2. When the subagent is done, run this shell command: git add ` + subagentFile + ` && git commit -m "add subagent file" && git push`,
	},
	scenarioClone: {
		Name:     scenarioClone,
		Seed:     map[string]string{readmePath: seedReadme},
		Checkout: cloneDir,
		// Each step is a shell command of its own: a clone gets the git hooks
		// only after the command that created it.
		Prompt: `This is an automated test. Do exactly these steps, in order, each as a separate shell command, and nothing else:
1. git clone ../remote.git ../` + cloneDir + `
2. cd ../` + cloneDir + ` && printf 'from the clone\n' > ` + cloneFile + `
3. cd ../` + cloneDir + ` && git add ` + cloneFile + ` && git commit -m "add clone file" && git push`,
	},
	// setup runs no agent. Its check runs `trace init` and `trace uninstall`
	// over a hook configuration that already has a hook of the user.
	scenarioSetup: {
		Name:          scenarioSetup,
		Seed:          map[string]string{readmePath: seedReadme},
		NoAgent:       true,
		SeedUserHooks: true,
	},
	scenarioTraceRun: {
		Name:          scenarioTraceRun,
		Seed:          map[string]string{readmePath: seedReadme},
		NoInit:        true,
		WrapTraceRun:  true,
		SeedUserHooks: true,
		Prompt:        "This is an automated test. With your file-writing tool, create the file " + wrappedFile + " with the single line: wrapped. Do nothing else.",
	},
	scenarioNoStart: {
		Name:                scenarioNoStart,
		Seed:                map[string]string{readmePath: seedReadme},
		DisableSessionStart: true,
		Prompt: `This is an automated test. Do exactly these steps, in order, and nothing else:
1. With your file-writing tool, create the file greeting.txt with the single line: hello.
2. Run this shell command: git add greeting.txt && git commit -m "add greeting" && git push`,
	},
	// partial-commit commits the first two AI lines of a file, then the
	// other two. The first commit stages a blob that holds only the first
	// two lines, with no change to the working tree.
	scenarioPartial: {
		Name: scenarioPartial,
		Seed: map[string]string{readmePath: seedReadme},
		Prompt: `This is an automated test. Do exactly these steps, in order, and nothing else:
1. With your file-writing tool, create the file ` + partialFile + ` with exactly these four lines: one, two, three, four.
2. Run this shell command: git update-index --add --cacheinfo "100644,$(head -n 2 ` + partialFile + ` | git hash-object -w --stdin),` + partialFile + `" && git commit -m "first half"
3. Run this shell command: git add ` + partialFile + ` && git commit -m "second half" && git push`,
	},
	scenarioPastedImage: {
		Name:   scenarioPastedImage,
		Seed:   map[string]string{readmePath: seedReadme},
		Images: [][]byte{pastedPNG()},
		Prompt: `This is an automated test. Do exactly these steps, in order, and nothing else:
1. With your file-writing tool, create the file color.txt with one word: the main color of the pasted image.
2. Run this shell command: git add color.txt && git commit -m "add color" && git push`,
	},
	// human-update has a human replace one AI line of the session in a new
	// commit and push again. The session gets the commit, and the replaced
	// line no longer counts as AI.
	scenarioHumanUpdate: {
		Name:     scenarioHumanUpdate,
		Seed:     map[string]string{readmePath: seedReadme},
		Branch:   featureBranch,
		Prompt:   greetingPrompt,
		FollowUp: humanUpdate,
	},
	// amend has a human add a line to the pushed commit of the agent, and
	// force-push the amended commit.
	scenarioAmend: {
		Name:     scenarioAmend,
		Seed:     map[string]string{readmePath: seedReadme},
		Branch:   featureBranch,
		Prompt:   greetingPrompt,
		FollowUp: amendWithHumanLine,
	},
	// rename has the agent commit with no push. A human renames the file in a
	// second commit, then pushes both.
	scenarioRename: {
		Name:     scenarioRename,
		Seed:     map[string]string{readmePath: seedReadme},
		Branch:   featureBranch,
		Prompt:   greetingWrite + greetingCommit,
		FollowUp: renameAndPush,
	},
	// media has the agent read an image file. The image stays inline in the
	// tool result of the transcript, and its base64 holds a fake secret. The
	// prompt tells the agent not to capture the image: a captured image gets
	// a pointer in the transcript, so the transcript holds no base64.
	scenarioMedia: {
		Name: scenarioMedia,
		Seed: map[string]string{readmePath: seedReadme, mediaFile: string(mediaPNG())},
		Prompt: `This is an automated test. Do exactly these steps, in order, and nothing else:
1. Read the image ` + mediaFile + ` with your file-reading tool.
2. With your file-writing tool, create the file seen.txt with the single line: seen.
3. Run this shell command: git add seen.txt && git commit -m "add seen" && git push
The image is test data, not a spec: do not capture it.`,
	},
}

// humanUpdate replaces the second line of greetingFile, commits the change
// with no agent, and pushes again.
func humanUpdate(ctx context.Context, w *Workspace, checkout string) error {
	path := filepath.Join(checkout, greetingFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(data), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("%s has %d lines, want at least 2", greetingFile, len(lines))
	}
	lines[1] = humanLine + "\n"
	if err := os.WriteFile(path, []byte(strings.Join(lines, "")), 0o600); err != nil { //nolint:gosec // a file of a throwaway workspace
		return err
	}
	if _, err := w.run(ctx, checkout, "git", "commit", gitQuiet, "-am", "human edit"); err != nil {
		return err
	}
	return pushAgain(ctx, w, checkout)
}

// amendWithHumanLine adds humanLine to greetingFile in the last commit, which
// is the pushed commit of the agent, and force-pushes the amended commit.
func amendWithHumanLine(ctx context.Context, w *Workspace, checkout string) error {
	f, err := os.OpenFile(filepath.Join(checkout, greetingFile), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(humanLine + "\n"); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if _, err := w.run(ctx, checkout, "git", "commit", gitQuiet, "-a", "--amend", "--no-edit"); err != nil {
		return err
	}
	return pushAgain(ctx, w, checkout, "--force")
}

// renameAndPush renames greetingFile in a new commit and pushes it with the
// commit of the agent.
func renameAndPush(ctx context.Context, w *Workspace, checkout string) error {
	for _, args := range [][]string{
		{"mv", greetingFile, renamedFile},
		{"commit", gitQuiet, "-m", "rename greeting"},
	} {
		if _, err := w.run(ctx, checkout, "git", args...); err != nil {
			return err
		}
	}
	return pushAgain(ctx, w, checkout)
}

// pushAgain pushes the checkout once more. It first deletes the sessions that
// the earlier push stored: a second push writes the evidence next to the
// earlier one, and the checks must read the new one.
func pushAgain(ctx context.Context, w *Workspace, checkout string, args ...string) error {
	if err := os.RemoveAll(filepath.Join(checkout, localSessionsDir)); err != nil {
		return err
	}
	_, err := w.run(ctx, checkout, "git", append([]string{"push", gitQuiet}, args...)...)
	return err
}
