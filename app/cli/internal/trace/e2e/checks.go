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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// errNotCovered marks a check that cannot see the feature in this run, for
// example because the harness does not report hook messages in headless mode
// (R-004).
var errNotCovered = errors.New("not covered")

// Session is the result of one scenario session.
type Session struct {
	Scenario  string
	Model     string
	Harness   Harness
	Workspace *Workspace
	// Checkout is the checkout where the agent committed and pushed.
	Checkout string
	Output   *RunOutput
	// Stored is the session that the push stored in local storage mode.
	Stored *StoredSession
	// HeadMessage is the message of the commit that the agent made.
	HeadMessage string
}

// Check verifies one feature on the result of one scenario session.
type Check struct {
	Scenario string
	Verify   func(s *Session) error
}

// Feature is the test definition of one feature row of the table. The key of
// the features map is the feature name in the table (R-002).
type Feature struct {
	// ControlPlane is true for a feature that needs a control plane. The suite
	// runs in local storage mode only, so its cells are "not covered" (D-007).
	ControlPlane bool
	// NotCovered is the reason why the suite cannot see the feature at all,
	// for example because neither the user nor the evidence can observe it.
	// Its cells are "not covered".
	NotCovered string
	// ModelDriven is true for a feature that depends on the model following an
	// instruction. Its cells run more than once (R-006).
	ModelDriven bool
	// Checks must all pass for a run of the feature to pass.
	Checks []Check
}

// features are the test definitions of the feature rows. A row of the table
// with no entry here is reported as "no check".
var features = map[string]Feature{
	// Setup
	"Hook installation and deletion": {Checks: []Check{{scenarioSetup, checkHookInstallation}}},
	"Trace run wrapper": {Checks: []Check{
		{scenarioTraceRun, checkHookConfigRestored},
		{scenarioTraceRun, checkGenericSessionStored},
	}},

	// Session collection
	"Missed session start": {Checks: []Check{
		{scenarioNoStart, checkNoSessionStartHook},
		{scenarioNoStart, checkCommitTrailer},
	}},
	"Transcript":           {Checks: []Check{{scenarioEditAndCommit, checkTranscript}}},
	"Token usage and cost": {Checks: []Check{{scenarioEditAndCommit, checkTokenUsage}}},
	"Model detection":      {Checks: []Check{{scenarioEditAndCommit, checkModel}}},
	"Subagents":            {Checks: []Check{{scenarioSubagent, checkSubagents}}},

	// Code attribution
	"Line attribution":          {Checks: []Check{{scenarioEditAndCommit, checkFileAttributedToAI("greeting.txt")}}},
	"Shell changes attribution": {Checks: []Check{{scenarioEditAndCommit, checkFileAttributedToAI("shell.txt")}}},
	"Partial commits":           {Checks: []Check{{scenarioPartial, checkBothCommitsCredited}}},
	"Line ranges":               {Checks: []Check{{scenarioEditAndCommit, checkLineRanges("greeting.txt", 3)}}},
	"Generated line counts":     {Checks: []Check{{scenarioEditAndCommit, checkGeneratedLineCounts}}},

	// User experience
	"Welcome message":         {Checks: []Check{{scenarioEditAndCommit, checkWelcomeMessage}}},
	"Session link after push": {ControlPlane: true},
	"Credentials warning":     {ControlPlane: true},

	// Specs and skills
	"Spec capture instruction":        {ModelDriven: true, Checks: []Check{{scenarioTicket, checkTicketCaptured}}},
	"Spec capture reminder":           {Checks: []Check{{scenarioEditAndCommit, checkPromptReminder}}},
	"Local spec sources read at push": {ModelDriven: true, Checks: []Check{{scenarioTicket, checkTicketContentStored}}},
	"Skills tracking":                 {ModelDriven: true, Checks: []Check{{scenarioSkill, checkSkillTracked}}},
	"Spec source pointers":            {ModelDriven: true, Checks: []Check{{scenarioTicket, checkTicketPointer}}},
	"Spec source role, title, and description": {
		ModelDriven: true, Checks: []Check{{scenarioTicket, checkTicketHeader}},
	},
	"Pasted image pointers": {Checks: []Check{{scenarioPastedImage, checkPastedImage}}},

	// Security
	"Secret redaction":           {Checks: []Check{{scenarioEditAndCommit, checkSecretRedacted}}},
	"Allow markers in redaction": {Checks: []Check{{scenarioEditAndCommit, checkAllowedSecretKept}}},
	"Media skipped by redaction": {Checks: []Check{{scenarioMedia, checkMediaKept}}},

	// Git integration
	"Commit trailer":        {Checks: []Check{{scenarioEditAndCommit, checkCommitTrailer}}},
	"Attestation at push":   {ControlPlane: true},
	"Local storage at push": {Checks: []Check{{scenarioEditAndCommit, checkLocalStorage}}},
	"Shell commands in another checkout": {Checks: []Check{
		{scenarioClone, checkCommitTrailer},
		{scenarioClone, checkFileAttributedToAI(cloneFile)},
	}},
}

func evidence(s *Session) *aicodingsession.Data {
	return &s.Stored.Evidence.Data
}

func checkTranscript(s *Session) error {
	main := evidence(s).RawSession["main"]
	if len(main) == 0 {
		return errors.New("the evidence has no main transcript")
	}
	for _, msg := range main {
		if bytes.Contains(msg, []byte("greeting.txt")) {
			return nil
		}
	}
	return errors.New("the transcript does not show the work of the session")
}

func checkTokenUsage(s *Session) error {
	u := evidence(s).Usage
	if u == nil || u.TotalTokens == 0 {
		return errors.New("the evidence has no token usage")
	}
	if u.EstimatedCostUSD <= 0 {
		return errors.New("the evidence has no cost")
	}
	return nil
}

func checkModel(s *Session) error {
	m := evidence(s).Model
	if m == nil || m.Primary == "" {
		return errors.New("the evidence has no primary model")
	}
	// A model can have a provider prefix, for example
	// amazon-bedrock/eu.anthropic.claude-haiku-5-5, that the evidence leaves
	// out, and a short name, for example haiku, that the evidence spells out.
	primary, want := strings.ToLower(m.Primary), strings.ToLower(path.Base(s.Model))
	if !strings.Contains(primary, want) && !strings.Contains(want, primary) {
		return fmt.Errorf("primary model %q is not the model of the session %q", m.Primary, s.Model)
	}
	return nil
}

func checkFileAttributedToAI(path string) func(s *Session) error {
	return func(s *Session) error {
		cc := evidence(s).CodeChanges
		if cc == nil {
			return errors.New("the evidence has no code changes")
		}
		for _, f := range cc.Files {
			if f.Path != path {
				continue
			}
			if f.Attribution != trace.AttributionAI || f.LinesAdded == 0 {
				return fmt.Errorf("%s: attribution %q with %d lines added, want AI lines", path, f.Attribution, f.LinesAdded)
			}
			return nil
		}
		return fmt.Errorf("the code changes do not list %s", path)
	}
}

// checkGeneratedLineCounts verifies that the evidence reports the lines of
// all the agent edits. Each net AI line came from an agent edit, so the
// generated lines are at least the net AI lines.
func checkGeneratedLineCounts(s *Session) error {
	cc := evidence(s).CodeChanges
	if cc == nil {
		return errors.New("the evidence has no code changes")
	}
	if cc.AIGeneratedLinesAdded == 0 {
		return errors.New("the evidence has no generated lines added")
	}
	if cc.AIGeneratedLinesAdded < cc.AILinesAdded {
		return fmt.Errorf("%d generated lines added, fewer than the %d net AI lines added", cc.AIGeneratedLinesAdded, cc.AILinesAdded)
	}
	return nil
}

// hookMessage returns the first hook message of an event, or errNotCovered
// when the harness does not report hook messages.
func hookMessage(s *Session, event string) (*HookMessage, error) {
	if !s.Output.HooksReported {
		return nil, fmt.Errorf("%w: the harness does not report hook messages in headless mode", errNotCovered)
	}
	for i := range s.Output.Hooks {
		if s.Output.Hooks[i].Event == event {
			return &s.Output.Hooks[i], nil
		}
	}
	return nil, fmt.Errorf("no %s hook message", event)
}

// checkWelcomeMessage checks the banner that the user sees at session start.
// In local storage mode, it names the on-disk destination.
func checkWelcomeMessage(s *Session) error {
	msg, err := hookMessage(s, "SessionStart")
	if err != nil {
		return err
	}
	if !strings.Contains(msg.UserMessage, "local-sessions") {
		return fmt.Errorf("the session start message does not name the local sessions folder: %q", msg.UserMessage)
	}
	return nil
}

func checkPromptReminder(s *Session) error {
	msg, err := hookMessage(s, "UserPromptSubmit")
	if err != nil {
		return err
	}
	if msg.ModelContext == "" {
		return errors.New("the user prompt hook gave no reminder to the model")
	}
	return nil
}

// ticketSpec returns the captured spec entry of the ticket of the scenario.
func ticketSpec(s *Session) (*aicodingsession.SpecEntry, error) {
	for i, e := range evidence(s).Spec {
		if e.Kind != aicodingsession.SpecKindSkill && strings.Contains(e.URI, filepath.Base(ticketPath)) {
			return &evidence(s).Spec[i], nil
		}
	}
	return nil, errors.New("the evidence has no spec entry for the ticket")
}

func checkTicketCaptured(s *Session) error {
	_, err := ticketSpec(s)
	return err
}

func checkTicketContentStored(s *Session) error {
	e, err := ticketSpec(s)
	if err != nil {
		return err
	}
	content, err := s.Stored.Material(e.Digest)
	if err != nil {
		return fmt.Errorf("read the spec material: %w", err)
	}
	if !bytes.Contains(content, []byte(ticketTitle)) {
		return errors.New("the spec material does not hold the text of the ticket file")
	}
	return nil
}

// checkTicketHeader checks that the capture of the ticket holds the role, the
// title, and the description that the agent wrote in its header.
func checkTicketHeader(s *Session) error {
	e, err := ticketSpec(s)
	if err != nil {
		return err
	}
	if e.Role == "" || e.Title == "" || e.Description == "" {
		return fmt.Errorf("the ticket entry has role %q, title %q, description %q: want all three", e.Role, e.Title, e.Description)
	}
	return nil
}

// checkTicketPointer checks that a copy of the ticket in the transcript, for
// example the full read of the ticket file, is replaced with a pointer to the
// spec material.
func checkTicketPointer(s *Session) error {
	e, err := ticketSpec(s)
	if err != nil {
		return err
	}
	for _, msg := range evidence(s).RawSession["main"] {
		if bytes.Contains(msg, []byte(pointer.Marker)) && bytes.Contains(msg, []byte(e.Digest)) {
			return nil
		}
	}
	return errors.New("the transcript has no pointer to the ticket material")
}

func checkSubagents(s *Session) error {
	subs := evidence(s).Subagents
	if len(subs) == 0 {
		return errors.New("the evidence has no subagent")
	}
	for _, sub := range subs {
		if sub.Tokens.Input+sub.Tokens.Output > 0 {
			return nil
		}
	}
	return errors.New("no subagent has token usage")
}

func checkSkillTracked(s *Session) error {
	for _, e := range evidence(s).Spec {
		if e.Kind != aicodingsession.SpecKindSkill || !strings.Contains(e.Title+e.URI, skillName) {
			continue
		}
		if e.Metadata == nil || e.Metadata.InvocationCount == 0 {
			return errors.New("the skill entry has no invocation")
		}
		return nil
	}
	return fmt.Errorf("the evidence has no entry for the skill %s", skillName)
}

// storedContains reports whether a file of the stored session holds the text.
func storedContains(s *Session, text string) (bool, error) {
	found := false
	err := filepath.WalkDir(s.Stored.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || found {
			return err
		}
		data, err := os.ReadFile(path) //nolint:gosec // a throwaway workspace of the suite
		if err != nil {
			return err
		}
		found = bytes.Contains(data, []byte(text))
		return nil
	})
	return found, err
}

func checkSecretRedacted(s *Session) error {
	if err := checkTranscript(s); err != nil {
		return fmt.Errorf("nothing to redact: %w", err)
	}
	found, err := storedContains(s, fakeSecret)
	if err != nil {
		return err
	}
	if found {
		return errors.New("the stored session holds the secret of the prompt")
	}
	// The prompt is in the transcript, so its secret must show as a
	// placeholder. With no placeholder, the check proves nothing.
	if found, err = storedContains(s, redactedPlaceholder); err != nil {
		return err
	}
	if !found {
		return errors.New("the stored session has no redaction placeholder")
	}
	return nil
}

// redactedPlaceholder is the start of the text that replaces a redacted
// secret, as internal/redaction writes it.
const redactedPlaceholder = "[CHAINLOOP_TRACE_REDACTED"

// checkAllowedSecretKept checks that the secret on the line with an allow
// marker is in the stored session. The line comes from the output of the
// tool that read the fixture. A transcript with no tool outputs holds no
// copy of the line, so it cannot show the feature.
func checkAllowedSecretKept(s *Session) error {
	found, err := storedContains(s, fakeAllowedSecret)
	if err != nil || found {
		return err
	}
	marked, err := storedContains(s, allowMarker)
	if err != nil {
		return err
	}
	if !marked {
		return fmt.Errorf("%w: the stored session holds no copy of the fixture line, for example because the transcript has no tool outputs", errNotCovered)
	}
	return errors.New("the secret on the line with an allow marker was redacted")
}

func checkCommitTrailer(s *Session) error {
	ids := state.ParseSessionIDsFromTrailer(s.HeadMessage)
	for _, id := range ids {
		if id == s.Stored.Evidence.Data.Session.ID {
			return nil
		}
	}
	return fmt.Errorf("the commit trailer has sessions %v, not the stored session", ids)
}

func checkLocalStorage(s *Session) error {
	if !s.Stored.HasStatement() {
		return errors.New("the stored session has no statement.json")
	}
	return nil
}
