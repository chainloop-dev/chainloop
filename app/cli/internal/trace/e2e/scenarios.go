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
)

const (
	wrappedFile = "wrapped.txt"
	partialFile = "partial.txt"
	mediaFile   = "diagram.png"
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
		Prompt: "This is an automated test. Implement the ticket in " + ticketPath + ". Then commit your change and push it with git.",
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
	// media has the agent read an image file. The image stays inline in the
	// tool result of the transcript, and its base64 holds a fake secret.
	scenarioMedia: {
		Name: scenarioMedia,
		Seed: map[string]string{readmePath: seedReadme, mediaFile: string(mediaPNG())},
		Prompt: `This is an automated test. Do exactly these steps, in order, and nothing else:
1. Read the image ` + mediaFile + ` with your file-reading tool.
2. With your file-writing tool, create the file seen.txt with the single line: seen.
3. Run this shell command: git add seen.txt && git commit -m "add seen" && git push`,
	},
}
