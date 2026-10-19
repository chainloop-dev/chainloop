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
}

// The fake secrets of the scenarios. They match the GitHub token rule of the
// secret scanner, and they are not real tokens.
const (
	// fakeSecret is in the prompt. The stored session must not hold it.
	fakeSecret = "ghp_8fK2mQ9xLp4ZrT7vWc1NbH6yJd3sGa5eUo0i" //nolint:gosec // a fake token that the suite expects redaction to remove
	// fakeAllowedSecret is on a line with an allow marker. The stored session
	// keeps it.
	fakeAllowedSecret = "ghp_Zx7Lq2Wm9Rt4Yp6Vn3Bk8Hd1Jf5Gs0CaEu2o" //nolint:gosec // a fake token on a line with an allow marker
)

// The README of every scenario repository.
const (
	readmePath = "README.md"
	seedReadme = "# Trace E2E\n"
)

const (
	scenarioEditAndCommit = "edit-and-commit"
	scenarioTicket        = "ticket"
	scenarioSkill         = "skill"
)

const (
	ticketPath  = "docs/ticket-e2e-42.md"
	ticketTitle = "E2E-42: Add a farewell file"
	skillName   = "e2e-farewell"
)

// scenarios are the scenarios of the suite, by name.
var scenarios = map[string]Scenario{
	scenarioEditAndCommit: {
		Name: scenarioEditAndCommit,
		Seed: map[string]string{
			readmePath:    seedReadme,
			"fixture.txt": "token = " + fakeAllowedSecret + " # gitleaks:allow\n",
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
		Seed: map[string]string{
			readmePath: seedReadme,
			".claude/skills/" + skillName + "/SKILL.md": `---
name: ` + skillName + `
description: Use when asked to add the farewell file to the repository.
---

Create the file farewell.txt with the single line: goodbye
Then run: git add farewell.txt && git commit -m "add farewell" && git push
`,
		},
		Prompt: "This is an automated test. Use the " + skillName + " skill.",
	},
}
