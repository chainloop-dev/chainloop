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

import "context"

// HarnessClaudeCode is the column name of Claude Code in the feature table.
const HarnessClaudeCode = "Claude Code"

const (
	// flagModel is the model flag that all the harnesses share.
	flagModel = "--model"
	// outputStreamJSON is the line-per-event output format of Claude Code and
	// the Cursor CLI.
	outputStreamJSON = "stream-json"
	// userHookMarker is in the hook of the user that a scenario seeds.
	userHookMarker = "e2e-user-hook"
)

// RunRequest is one headless session of a harness.
type RunRequest struct {
	// Dir is the repository the session runs in.
	Dir string
	// Model is the model of the session, as the harness names it.
	Model string
	// Prompt is the only user prompt of the session.
	Prompt string
	// Env is the environment of the harness process.
	Env []string
	// Wrapper is a command that runs the harness, for example
	// `chainloop trace run --local --claude --`. Empty runs the harness
	// directly.
	Wrapper []string
	// Images are PNG images that the user pastes with the prompt.
	Images [][]byte
}

// HookConfig describes the files where the harness reads its hooks.
type HookConfig struct {
	// Path is the file of the Chainloop hooks, relative to the repository
	// root.
	Path string
	// ChainloopMarker is a text that only a Chainloop hook in the file holds.
	ChainloopMarker string
	// UserPath is the file of the hook of the user, relative to the
	// repository root. Empty means Path: the hooks share one file.
	UserPath string
	// UserConfig is the content of the user file with one hook of the user
	// and no Chainloop hook.
	UserConfig string
	// UserMarker is a text that only the hook of the user holds.
	UserMarker string
}

// UserFile returns the file of the hook of the user.
func (c HookConfig) UserFile() string {
	if c.UserPath != "" {
		return c.UserPath
	}
	return c.Path
}

// HookMessage is a message that a Trace hook returned to the harness.
type HookMessage struct {
	// Event is the hook event, for example SessionStart.
	Event string
	// UserMessage is the text that the harness shows to the user.
	UserMessage string
	// ModelContext is the text that the harness gives to the model.
	ModelContext string
}

// RunOutput is what a harness showed during a session.
type RunOutput struct {
	// Raw is the full output of the harness process.
	Raw []byte
	// Hooks are the hook messages that the harness reported, in order. A
	// harness that does not report them leaves it empty.
	Hooks []HookMessage
	// HooksReported is true when the harness reports hook messages in its
	// headless output. When it is false, a check on a hook message is "not
	// covered" (R-004).
	HooksReported bool
}

// Harness adapts one coding agent. It is the only part of the suite that
// knows the agent (R-011).
type Harness interface {
	// Column is the column name of the harness in the feature table.
	Column() string
	// TraceFlag is the flag of `chainloop trace init` that selects the
	// harness, for example "--claude".
	TraceFlag() string
	// Version returns the installed version of the harness.
	Version(ctx context.Context) (string, error)
	// Run runs one headless session and returns its output. An error means
	// that the session could not run, not that a feature failed.
	Run(ctx context.Context, req RunRequest) (*RunOutput, error)
	// HookConfig describes the hook configuration file of the harness.
	HookConfig() HookConfig
	// DisableSessionStart removes the session-start hook that `trace init`
	// installed in the repository, so that a session starts with no
	// session-start hook.
	DisableSessionStart(repo string) error
	// SkillPath returns the file of a project skill, relative to the
	// repository root.
	SkillPath(name string) string
}
