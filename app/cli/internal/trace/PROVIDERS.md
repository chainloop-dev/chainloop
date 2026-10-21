# Chainloop Trace: how to add a provider

A provider connects one AI coding agent (a "harness") to Chainloop Trace. This page tells how to add a provider. It uses the Claude Code (`claude/`), Cursor (`cursor/`), and OpenCode (`opencode/`) providers as examples. The Pi provider (`pi/`) is a provider that is not complete: it parses sessions, but it is not in the registry yet.

Read these documents first:

- `provider.go`: the interfaces that a provider implements.
- `README.md`: the feature table and the compatibility matrix. A new provider adds a column to each table.
- `docs/specs/`: the specs of Trace. For example, `issue-3520-pi-trace-provider.md` is the spec of a new provider. A large new provider needs its own spec. Write it with `br:eng-spec`.

## Lifecycle of a session

This section tells which part of a provider runs at each step.

```
trace init --<agent>      InstallHooks          writes the agent hook configuration
                          (no provider code)       init installs the git hooks
agent: session start      ReadHookInput         -> HandleAgentSessionStart
                          CopySessionData          (session record, first copy)
                          AnnounceSessionStart     (banner, spec instruction)
agent: user prompt        AnnouncePromptSubmit  -> HandleAgentPromptSubmit
agent: before a tool      CaptureFileSnapshot   -> HandleAgentPreToolUse
                          (or worktree snapshot for a shell tool)
agent: after a tool       ResolveBeforeContent  -> HandleAgentPostToolUse
                          CleanupAfterEdit         (AI lines in the ledger)
                          AnnounceToUser           (session link after a push)
git commit                (no provider code)       commit-msg, post-commit hooks
agent: session end        CopySessionData       -> HandleAgentSessionEnd
git push                  CopySessionData       -> pre-push: RunTracePush
                          ParseSession             (evidence for each session)
                          SkillTracker, SpecCopyReplacer, PastedImageFinder
trace uninstall           UninstallHooks
trace run --<agent> -- …  InstallHooksForTraceRun, SettingsFile (backup and restore)
```

The handlers are in `app/cli/pkg/action/trace_agent_hook.go` (agent hooks) and `app/cli/pkg/action/trace_hook_handler.go` (git hooks and push). They are the same for all providers. A provider gives only the agent-specific parts.

Each hook must never block the agent. A handler that cannot read its input, or that cannot find the trace state, writes a debug log and returns `nil`.

## The interfaces

### `trace.Provider` (required)

The provider is a stateless singleton. Each method that changes state gets a `*state.Store`. Do not keep a store in the provider. Add a compile-time check:

```go
const Name = "myagent" // stored in SessionRecord.Provider and in the evidence

var _ trace.Provider = (*Provider)(nil)
```

The methods fall into six groups.

#### 1. Identity

| Method | What to do | Examples |
|--------|------------|----------|
| `Name()` | Return the exported `Name` constant. The CLI wiring and the session records use this value. Do not change it after a release: old session records name the provider with it. | `claude-code`, `cursor`, `opencode` |

#### 2. Hook installation

| Method | What to do | Examples |
|--------|------------|----------|
| `InstallHooks(repoRoot)` | Write the Chainloop hooks into the project-local configuration of the agent. Keep the hooks of the user. A second call changes nothing. | Claude: merges entries into `.claude/settings.json`. Cursor: merges entries into `.cursor/hooks.json` and sets `version: 1`. OpenCode: writes a TypeScript plugin, `.opencode/plugins/chainloop-trace.ts`, from a template. |
| `InstallHooksForTraceRun(repoRoot)` | The same hooks, but without the session-end hook. `trace run` ends the session itself. | All three filter out the session-end event. OpenCode removes the `session.deleted` block from the template. |
| `SettingsFile(repoRoot)` | Return the absolute path of the file that `InstallHooks` writes. `trace init` makes a backup of this file and restores it only when the install fails. After a successful init, the hooks stay. `trace run` makes a backup and always restores it when the agent command exits. | |
| `UninstallHooks(repoRoot)` | Remove only the Chainloop hooks. Keep the hooks of the user. | Claude and Cursor find the entries by a marker text (`chainloop trace hook <agent>`). OpenCode removes the plugin file, because Chainloop owns all of it. |
| `HooksInstalled(repoRoot)` | Tell if the repository has the Chainloop hooks of this agent. Hooks of the user do not count. The init wizard uses it to preselect the harnesses. | |

Each installed hook runs `chainloop trace hook <agent> <event>`. Use the same marker text in the hook command, so that `UninstallHooks` and `HooksInstalled` can find it.

If the agent has no hook configuration file but has a plugin or extension system (OpenCode, Pi), write a small plugin. The plugin calls the `chainloop trace hook` subcommands, sends the JSON payload on stdin, and reads the JSON response on stdout. Generate the tool lists of the plugin from the Go slices, so that the plugin and `IsFileWritingTool`/`IsCommandTool` cannot drift (see `toolsArrayLiteral` in `opencode/hooks.go`).

A repository keeps the plugin that an older CLI wrote. When you change what the plugin sends (for example, a new field in the payload), the user must run `chainloop trace init` again to get the new plugin. Write this in the README "Why not Yes" line of each feature that needs the new plugin. For an example, see the OpenCode notes on skills tracking and on shell commands in another checkout.

#### 3. Hook input

| Method | What to do | Examples |
|--------|------------|----------|
| `ReadHookInput(r)` | Decode the hook payload into a typed struct, then map it to `trace.HookInput`. Limit the read size (`maxHookPayloadBytes`, 16 MiB). | See below. |
| `IsFileWritingTool(name)` | Return true for the tools that write one file with a known path. | Claude: `Edit`, `Write`, `MultiEdit`. OpenCode: `edit`, `write`, `apply_patch`, `patch`. Cursor: the synthetic name `edit` that `ReadHookInput` sets for `afterFileEdit`. |
| `IsCommandTool(name)` | Return true for the tools that run a shell command. | Claude: `Bash`. OpenCode: `bash`, `shell`. Cursor: `Shell`. |

Fill these `HookInput` fields when the agent gives them:

- `SessionID` (required). It must pass `state.ValidSessionID`. Cursor uses `conversation_id` first.
- `FilePath`: an absolute path. OpenCode 2 sends a relative path, so its provider resolves it.
- `Cwd`: the directory where the agent keeps the transcripts of the session. Cursor uses the first workspace root.
- `TranscriptPath`: the path of the transcript, if the agent gives it. It is better than `Cwd`.
- `ToolName`, `ToolUseID`, `AgentID`: these pair the pre and post hooks of one call, and they tell subagents apart.
- `Command`: the command line of a shell tool. Trace uses it to find other checkouts (`cd`, `git -C`, `git clone`).
- `AgentVersion`, `Model`: set them at session start when the transcript does not hold them (Cursor). They go to `ParseOpts`.
- `Edits`: the old and new strings of an edit, for an agent with no pre-edit hook (Cursor).
- `SkillDir`: the folder of a skill that the tool call loaded.
- `ToolFailed`: the hook fires after a failed tool call (Claude `PostToolUseFailure`).

#### 4. Code attribution

Trace attributes lines to the AI from the content of a file before and after each agent edit. Select one of two strategies:

| Strategy | When to use it | `CaptureFileSnapshot` | `ResolveBeforeContent` | `CleanupAfterEdit` | Examples |
|----------|----------------|-----------------------|------------------------|--------------------|----------|
| Snapshot | The agent has a hook before each file edit. | Read the file and call `store.SaveFileSnapshot`. A missing file is not an error (a new file). | Return `store.LoadFileSnapshot`. | Call `store.DeleteFileSnapshot`. | Claude, OpenCode |
| Reverse edits | The agent has only a hook after the edit, with the old and new strings. | No-op. | Return `attribution.ReverseApplyEdits(after, input.Edits)`. | No-op. | Cursor |

`ResolveBeforeContent` returns `nil` when it cannot rebuild the content. Then all lines of the file are AI lines.

Shell commands need no provider code. When `IsCommandTool` is true, the handlers snapshot the working tree before the command and compare it after the command. The agent must send a pre and a post hook for its shell tool, with the same `ToolUseID`.

The git hooks (`commit-msg`, `post-commit`, `post-rewrite`, `pre-push`) are the same for all providers. Do not change them for a new provider.

#### 5. Session data and parsing

| Method | What to do | Examples |
|--------|------------|----------|
| `CopySessionData(store, loc)` | Copy the session data of the agent into `store.RawSessionDir()`. The handlers call it at session start, at session end, and before each parse at push. So the copy must be safe to repeat, and the push must not depend on the storage of the agent. Make sure that a path from a hook payload stays inside the storage folder of the agent. | Claude: copies `~/.claude/projects/<encoded cwd>/<id>.jsonl` and the subagent files with `store.CopySessionJSONL`. It checks `TranscriptPath` with `transcriptSourceDir`. Cursor: finds the transcript from the workspace path and calls `store.CopyRawSessionFile`. OpenCode: runs `opencode export` (1.x) or `opencode session export` (2) into `state.RawSessionPath`. Pi: copies the reported transcript to a temporary file, then renames it, so a failed copy keeps the last good copy. |
| `ParseSession(ctx, opts)` | Read the copy from `opts.SessionDir` (use `state.FindRawSessionFile`) and return `*aicodingsession.Evidence`. Never parse the live file of the agent. | `claude/parse.go`, `cursor/parse.go`, `opencode/parse.go`, `pi/parse.go` |

Fill the evidence (`pkg/attestation/crafter/materials/aicodingsession`) as follows. Use `aicodingsession.NewEvidence(data)`.

| Field | Content | Claude | Cursor | OpenCode |
|-------|---------|--------|--------|----------|
| `Agent.Name`, `Agent.Version` | The provider name and the agent version. | From the transcript. | `opts.AgentVersion` (from the session-start hook). | From the export (1.x only). |
| `Session.ID`, `Slug`, `StartedAt`, `EndedAt`, `DurationSeconds` | RFC3339 times. | First and last transcript times. | File modification time only. | Epoch milliseconds of the export. |
| `Model.Primary`, `Provider`, `ModelsUsed` | The model with the most messages, the vendor, and a sorted list of all models. | From the assistant messages. Provider `anthropic`. | `opts.Model`. No list. | From the assistant messages. Provider from `providerID`. |
| `Usage` | Input, output, total, cache read, and cache write tokens, and `EstimatedCostUSD` rounded to 4 decimals. | Summed per model, with the subagents. The cost comes from the embedded `pricing.json`. | Not set (`Usage` is `nil`), with a warning. | The session totals of the export, or the sum of the messages. The cost is the value that OpenCode reports. |
| `ToolsUsed` | A count per tool, sorted by count, then by name. | | | |
| `Conversation` | User and assistant message counts. | | | |
| `Subagents` | One entry per subagent, with its tokens. | Yes. | No. | No. |
| `RawSession` | A map of streams. `main` is required: use an empty slice, not `nil`. Other keys are subagent streams. Each line must be one JSON object. If the agent has no line format, convert each message to the Claude line shape (`{type, uuid, timestamp, message: {role, content}}`) so the UI can show it. | Transcript lines. | Transcript lines. | Rebuilt by `buildRawSessionEntries`. |
| `Warnings` | Each data item that the parser could not read or that the agent does not give. | | Always "no token usage". | |

The caller sets `Session.Mode`, `GitContext`, `CodeChanges`, `Spec`, and the skills. Do not set them in the provider. Claude sets `GitContext.Branch` and `WorkDir` from the transcript, but the push replaces all of `GitContext` when it can read the repository.

Cost:

- If the agent records the cost, use its value (OpenCode, Pi).
- If the agent records tokens but no cost, compute the cost from an embedded price table (Claude `pricing.json`, `pricing.go`). Check the table at `init` and use a default model for an unknown model. Add a warning when you use the default price.
- If the agent records no tokens, set no `Usage` and add a warning (Cursor).

#### 6. Messages to the user and to the model

| Method | What to do | Claude | Cursor | OpenCode |
|--------|------------|--------|--------|----------|
| `SupportsSessionStartBanner()` | True when a session-start response can show text to the user. | true (`systemMessage`) | false | false |
| `SupportsSessionStartInstruction()` | True when a session-start response can give text to the model. | true (`additionalContext`) | true (`additional_context`) | true (the plugin posts it) |
| `AnnounceSessionStart(msg)` | Write one JSON document to stdout with `msg.Banner` and `msg.Instruction`. Write nothing when the message is empty. | | | |
| `SupportsPromptReminder()` | True when a prompt-submit response can give text to the model. | true | false | true |
| `AnnouncePromptSubmit(reminder)` | Write the prompt-submit response. Write nothing for an empty reminder. | | no-op | |
| `AnnounceToUser(msg)` | Show a message (the session link) to the user after a shell command. Return `trace.ErrAnnounceUnsupported` when there is no channel. Then the link stays on disk. | `systemMessage` and `additionalContext` | unsupported | unsupported |

Write only one JSON document to stdout for each hook. The agent reads stdout as one document and drops a second one.

Some agents need a response from a pre-tool hook, or they block the tool. Cursor needs `{"permission":"allow"}`. Write that response before you call the handler (see `newTraceHookCursorPreToolUseCmd` in `app/cli/cmd/trace_hook.go`).

### Optional interfaces

The handlers find these interfaces with a type assertion. A provider without one gets a degraded feature. Write the reason in the README.

| Interface | Feature | Implemented by | What to do |
|-----------|---------|----------------|------------|
| `trace.SkillTracker` | Skills tracking | Claude, OpenCode | `NewSkillLoads`: return the skill folders that the session loaded since the last call (from the hook input or the transcript). `SessionSkills`: return the skill uses from the copied session data, in the order of the first use. `SkillRoots`: return the plugin, organization, user, and project skill folders of the agent. |
| `trace.SpecCopyReplacer` | Spec source pointers | Claude | Replace each exact copy of a spec source in the raw session with `pointer.New(digest)`. Report the result per finder. |
| `trace.PastedImageFinder` | Pasted image pointers | Claude | Return each distinct pasted image of the raw session, with its digest, media type, and number. |

Add a compile-time check for each one, for example `var _ trace.SkillTracker = (*Provider)(nil)`.

## Local storage mode

A provider needs no code for local mode (`trace init --local`, `trace run --local`). The shared handlers select the mode:

- `HandleAgentSessionStart` reads `config.IsLocalStorage(repoRoot)`, or `store.IsTraceRunLocal()` for a `trace run` session. In local mode, it does no credentials check. The banner names the folder on disk, not the dashboard. `AnnounceSessionStart` sends it as usual.
- The `pre-push` hook calls `HandlePrePushHookLocal`. It uses the same `CopySessionData` and `ParseSession` as an attestation, and it writes the evidence to the `local-sessions` folder of the trace directory.

Local mode sets these conditions on a provider:

- `CopySessionData` and `ParseSession` must not use the control plane or the network. Local mode needs no login and no network. A local agent command is permitted (OpenCode runs `opencode export`).
- `trace run --local` can run outside a git repository. Use only the `*state.Store` that the handler gives (for example `store.RawSessionDir()`). Do not find the trace directory or `.git` yourself.
- The end-to-end suite runs in local mode only. A feature that needs a control plane is `not covered` for each provider.

## CLI wiring

A provider is not active until the CLI knows about it. Change these files:

1. `app/cli/internal/trace/providers/registry.go`: add `myagent.New()` to `all`. The order of `all` is the order of the init wizard. The wizard (`resolveTraceProviders` in `app/cli/cmd/trace_init_providers.go`) lists all registered providers. It preselects the providers that `HooksInstalled` reports. `trace uninstall` and the push find the provider through the registry.
2. `app/cli/cmd/trace_init_providers.go`: add a field to `traceProviderFlags`, and add the field to `any()` and `names()`.
3. `app/cli/cmd/trace_init.go`: add the `--myagent` flag, set it in `traceProviderFlags`, and add it to the help text that lists the harness flags.
4. `app/cli/cmd/trace_run.go`: add the same `--myagent` flag and set it in `traceProviderFlags`.
5. `app/cli/cmd/trace_hook.go`: add `newTraceHookMyAgentCmd()` to `newTraceHookCmd`, with one subcommand for each installed hook. Each subcommand calls `InitHookLogger()` and the shared handler with `myagent.New()`:
   - `session-start`: `runAgentSessionStart(cmd, myagent.New())`
   - `user-prompt-submit`: `action.HandleAgentPromptSubmit`
   - `pre-tool-use`: `action.HandleAgentPreToolUse`
   - `post-tool-use` (or an agent event name, such as Cursor `after-file-edit`): `action.HandleAgentPostToolUse`
   - `session-end`: `action.HandleAgentSessionEnd`

   Set the annotation `skipActionOptsInit: "true"` on each subcommand, so that the root command does not open a control-plane connection before the hook runs. The session-start handler still connects later, when it needs to: for the credentials check and the dashboard link of the banner.
6. Run `make generate` to update `app/cli/documentation/cli-reference.md`.

## Tests

Use table-driven tests. Write the tests first.

- Provider unit tests in the provider package:
  - `hooks_test.go`: install, a second install, keep the hooks of the user, uninstall, `HooksInstalled`, `InstallHooksForTraceRun` without session end, and `ReadHookInput` for each event.
  - `parse_test.go`: parse real transcripts in `testdata/` (remove all secrets and personal data first). Test the models, the tokens, the cost, the tools, the counts, and the warnings for bad lines.
  - `announce_test.go`: the exact JSON of each response, and no output for an empty message.
  - `provider_test.go`: `CopySessionData` with a missing transcript, a moved transcript, and a path outside the storage of the agent.
- Registry tests in `app/cli/internal/trace/providers/`:
  - `capabilities_test.go`: add a case to `TestSessionStartChannels` and add the name to `TestSessionStartChannelsCoverEveryProvider`. The second test fails until you do this.
  - `hooks_installed_test.go` runs for each registered provider. It needs no change.
- CLI tests in `app/cli/cmd/`: `trace_init_providers_test.go` (flags and wizard) and `trace_init_test.go` (files that init writes, harness list in the summary).

### End-to-end tests (required)

Each provider must have an end-to-end harness. The suite in `e2e/` runs the agent in headless mode with real models. It checks the README cells marked **Yes**, **Partial**, or **Unknown** that have a registered check in `e2e/checks.go` (spec issue-3635). It does not check the other cells:

- A cell marked **No** or **Not applicable** is `skipped`.
- A feature that needs a control plane is `not covered`, because the suite uses local storage mode only.
- A row with no registered check is `no check`.

A cell that the suite did not check stays at **Unknown**, or you add a check for it.

1. Add `e2e/myagent.go` with a type that implements `e2e.Harness` (`e2e/harness.go`): `Column`, `TraceFlag`, `Version`, `Run`, `HookConfig`, `DisableSessionStart`, and `SkillPath`. Use `e2e/claude.go`, `e2e/cursor.go`, `e2e/opencode.go`, and `e2e/pi.go` as examples. Run the agent with its configuration and session folders next to the workspace, so the run does not touch the configuration of the user.
2. Add the harness to the `harnesses` map in `e2e/e2e_test.go`. The key is the column name in the README.
3. Add the harness to `e2e/config.yaml`, with the pinned version and the models. Write the API key that the models need in a comment.
4. If the agent shows hook messages in its headless output, set `RunOutput.Hooks` and `HooksReported`. Otherwise the hook message checks are "not covered".
5. Run the suite for the new column:

   ```bash
   CHAINLOOP_E2E_HARNESS="My Agent" make -C app/cli test-trace-e2e
   ```

Do not add a provider with no end-to-end harness: its **Unknown** cells stay not verified.

### Compatibility matrix (required)

Update `README.md` in the same pull request:

1. Add a column for the agent to each feature table. Start each cell at **Unknown**.
2. Run the end-to-end suite. Set each checked cell to **Yes**, **Partial**, **No**, or **Not applicable** from the results. Keep the cells that the suite does not check at **Unknown**.
3. For each cell that is not **Yes**, add a line to the "Why not Yes" list of the table.
4. Add a row to "How each agent is hooked in", with the configuration file and the hooks.
5. Update the list of providers at the top of the page. If the agent had a "not supported yet" paragraph (Pi, Codex), remove it.

The suite reads the README tables (`e2e/expected.go`). A column name in the README, in `e2e/config.yaml`, and in the `harnesses` map must be the same.

## Checklist

Copy this list into the pull request and tick each item.

### Provider package (`app/cli/internal/trace/<agent>/`)

- [ ] `Name` constant and `var _ trace.Provider = (*Provider)(nil)`.
- [ ] `InstallHooks`: a second call changes nothing. The hooks of the user stay.
- [ ] `InstallHooksForTraceRun`: all hooks except session end.
- [ ] `SettingsFile`: the file that install writes.
- [ ] `UninstallHooks`: removes only the Chainloop hooks.
- [ ] `HooksInstalled`: true only for the Chainloop hooks.
- [ ] Hook events: session start, user prompt (if the agent has it), and session end.
- [ ] Hook events: before and after each file tool and shell tool, and after the skill tool (for skills tracking).
- [ ] `ReadHookInput`: typed struct, size limit, absolute `FilePath`, `Cwd` and `TranscriptPath`, `ToolUseID` and `AgentID`, `Command`, `AgentVersion` and `Model` when the transcript does not hold them.
- [ ] `IsFileWritingTool` and `IsCommandTool`: one Go slice is the source of each list (and of the plugin, if there is one).
- [ ] Attribution strategy selected: snapshot (`CaptureFileSnapshot`, `ResolveBeforeContent`, `CleanupAfterEdit`) or reverse edits (`HookInput.Edits`).
- [ ] Shell tool sends pre and post hooks with the same `ToolUseID`.
- [ ] `CopySessionData`: safe to repeat, checks paths from the payload, does not fail the hook when the agent or the transcript is missing.
- [ ] `ParseSession`: reads only the copy in `opts.SessionDir`.
- [ ] `CopySessionData` and `ParseSession`: no control plane and no network, so that local mode works.
- [ ] Trace state only through the `*state.Store`, so that `trace run --local` works outside a git repository.
- [ ] Evidence: agent name and version, session ID and times, primary model, models used, model provider.
- [ ] Evidence: token usage (input, output, total, cache read, cache write).
- [ ] Evidence: cost from the agent, or from an embedded price table, or zero with a warning.
- [ ] Evidence: tools used, conversation counts, subagents (if the agent has them).
- [ ] Evidence: `RawSession["main"]` is never `nil`. Each line is a JSON object.
- [ ] Evidence: a warning for each data item that cannot be read.
- [ ] Session-start, prompt-submit, and post-tool responses: one JSON document each, and nothing for an empty message.
- [ ] `Supports*` methods tell the truth about each channel.
- [ ] `AnnounceToUser` returns `trace.ErrAnnounceUnsupported` when there is no channel.
- [ ] Optional: `SkillTracker`, `SpecCopyReplacer`, `PastedImageFinder`, each with a compile-time check.
- [ ] Unit tests: hooks, parse with real `testdata/`, announce responses, copy.

### CLI wiring

- [ ] Registered in `providers/registry.go`.
- [ ] `traceProviderFlags` field, `any()`, `names()`.
- [ ] `--<agent>` flag in `trace init` (with help text) and in `trace run`.
- [ ] `chainloop trace hook <agent> …` subcommands in `cmd/trace_hook.go`, with `skipActionOptsInit`.
- [ ] Pre-tool response written first, if the agent blocks tools without one.
- [ ] `providers/capabilities_test.go` updated.
- [ ] `cmd/trace_init_providers_test.go` and `cmd/trace_init_test.go` updated.
- [ ] `make generate` run for the CLI reference.

### End-to-end tests and documentation

- [ ] `e2e/<agent>.go` implements `e2e.Harness`.
- [ ] Harness in the `harnesses` map of `e2e/e2e_test.go`.
- [ ] Version and models in `e2e/config.yaml`.
- [ ] Suite run for the new column, with the results checked.
- [ ] `README.md`: new column in each feature table, with values from the suite.
- [ ] `README.md`: a "Why not Yes" line for each cell that is not **Yes**.
- [ ] `README.md`: row in "How each agent is hooked in", and provider list at the top updated.
- [ ] Spec in `docs/specs/` for a large provider, with requirement IDs cited in the pull request.
- [ ] License header year updated in each changed file.
- [ ] `make -C app/cli lint` and `make -C app/cli test` pass.
