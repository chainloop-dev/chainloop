package cursor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
)

const (
	// hookMarker identifies Chainloop-installed entries inside .cursor/hooks.json.
	hookMarker = "chainloop trace hook cursor"
	// settingsFile is the project-local Cursor hooks configuration file.
	settingsFile = ".cursor/hooks.json"
	// hooksSchemaVersion is the schema version Cursor expects in hooks.json.
	hooksSchemaVersion = 1
	// defaultHookTimeoutSeconds is the per-hook timeout persisted in hooks.json.
	defaultHookTimeoutSeconds = 30

	// Cursor hook event names.
	eventSessionStart  = "sessionStart"
	eventSessionEnd    = "sessionEnd"
	eventAfterFileEdit = "afterFileEdit"

	// syntheticEditToolName is the synthetic tool name set on HookInput for
	// afterFileEdit events so generic handlers can treat them like
	// Claude-style file-writing tool invocations.
	syntheticEditToolName = "edit"
)

type cursorHookEvent struct {
	event   string
	command string
}

// cursorHookEvents is the full list of hooks installed by InstallHooks.
var cursorHookEvents = []cursorHookEvent{
	{eventSessionStart, "chainloop trace hook cursor session-start"},
	{eventSessionEnd, "chainloop trace hook cursor session-end"},
	{eventAfterFileEdit, "chainloop trace hook cursor after-file-edit"},
}

// SettingsFile returns the absolute path to .cursor/hooks.json for the given repo.
func (p *Provider) SettingsFile(repoRoot string) string {
	return filepath.Join(repoRoot, settingsFile)
}

// InstallHooks adds Chainloop hooks to .cursor/hooks.json.
func (p *Provider) InstallHooks(repoRoot string) error {
	return p.installHookEvents(repoRoot, cursorHookEvents)
}

// InstallHooksForTraceRun installs every hook except SessionEnd; trace
// run drives end-of-session itself.
func (p *Provider) InstallHooksForTraceRun(repoRoot string) error {
	events := make([]cursorHookEvent, 0, len(cursorHookEvents))
	for _, h := range cursorHookEvents {
		if h.event == eventSessionEnd {
			continue
		}
		events = append(events, h)
	}

	return p.installHookEvents(repoRoot, events)
}

func (p *Provider) installHookEvents(repoRoot string, events []cursorHookEvent) error {
	settingsPath := filepath.Join(repoRoot, settingsFile)

	settings, err := readJSONFile(settingsPath)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	changed := false
	for _, h := range events {
		if addHookEntry(hooks, h.event, h.command) {
			changed = true
		}
	}

	if version, _ := settings["version"].(float64); int(version) != hooksSchemaVersion {
		settings["version"] = hooksSchemaVersion
		changed = true
	}

	if !changed {
		return nil
	}

	settings["hooks"] = hooks

	return writeJSONFile(settingsPath, settings)
}

// UninstallHooks removes Chainloop hooks from .cursor/hooks.json.
// Leaves user-authored hooks intact.
func (p *Provider) UninstallHooks(repoRoot string) error {
	settingsPath := filepath.Join(repoRoot, settingsFile)

	settings, err := readJSONFile(settingsPath)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}

	changed := false
	for _, h := range cursorHookEvents {
		if removeHookEntry(hooks, h.event) {
			changed = true
		}
	}

	if !changed {
		return nil
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
		// version is meaningless without any hooks; drop it too
		delete(settings, "version")
	} else {
		settings["hooks"] = hooks
	}

	if len(settings) == 0 {
		err := os.Remove(settingsPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	return writeJSONFile(settingsPath, settings)
}

// cursorHookInput is the wire-level structure of a Cursor hook JSON payload.
// Only the fields we consume are modelled; unknown fields are ignored.
type cursorHookInput struct {
	ConversationID string           `json:"conversation_id"`
	SessionID      string           `json:"session_id"`
	HookEventName  string           `json:"hook_event_name"`
	CursorVersion  string           `json:"cursor_version"`
	Model          string           `json:"model"`
	FilePath       string           `json:"file_path"`
	Edits          []cursorHookEdit `json:"edits"`
}

type cursorHookEdit struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// maxHookPayloadBytes caps hook payload reads to defend against runaway or
// malformed payloads (afterFileEdit edits can embed arbitrary file content).
const maxHookPayloadBytes = 16 * 1024 * 1024

// ReadHookInput parses a Cursor hook JSON payload from the given reader.
// ConversationID is preferred as the session identifier; session_id is used
// as a fallback for forward-compatibility.
func (p *Provider) ReadHookInput(r io.Reader) (*trace.HookInput, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxHookPayloadBytes))
	if err != nil {
		return nil, err
	}

	var raw cursorHookInput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	sessionID := raw.ConversationID
	if sessionID == "" {
		sessionID = raw.SessionID
	}

	input := &trace.HookInput{
		SessionID:     sessionID,
		HookEventName: raw.HookEventName,
		FilePath:      raw.FilePath,
		AgentVersion:  raw.CursorVersion,
		Model:         raw.Model,
	}

	if raw.HookEventName == eventAfterFileEdit {
		input.ToolName = syntheticEditToolName
	}

	if len(raw.Edits) > 0 {
		input.Edits = make([]trace.HookEdit, len(raw.Edits))
		for i, e := range raw.Edits {
			input.Edits[i] = trace.HookEdit{OldString: e.OldString, NewString: e.NewString}
		}
	}

	return input, nil
}

func addHookEntry(hooks map[string]any, event, command string) bool {
	entries, _ := hooks[event].([]any)

	for i, entry := range entries {
		if !entryContainsChainloopHook(entry) {
			continue
		}
		m, _ := entry.(map[string]any)
		existingCmd, _ := m["command"].(string)
		if existingCmd == command {
			return false
		}
		entries[i] = buildHookEntry(command)
		hooks[event] = entries

		return true
	}

	hooks[event] = append(entries, buildHookEntry(command))

	return true
}

func removeHookEntry(hooks map[string]any, event string) bool {
	entries, _ := hooks[event].([]any)
	if len(entries) == 0 {
		return false
	}

	var kept []any
	for _, entry := range entries {
		if !entryContainsChainloopHook(entry) {
			kept = append(kept, entry)
		}
	}

	if len(kept) == len(entries) {
		return false
	}

	if len(kept) == 0 {
		delete(hooks, event)
	} else {
		hooks[event] = kept
	}

	return true
}

func buildHookEntry(command string) map[string]any {
	return map[string]any{
		"command": command,
		"timeout": defaultHookTimeoutSeconds,
	}
}

func entryContainsChainloopHook(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}

	cmd, _ := m["command"].(string)

	return strings.Contains(cmd, hookMarker)
}

func readJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}

func writeJSONFile(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	content = append(content, '\n')

	return os.WriteFile(path, content, 0600)
}
