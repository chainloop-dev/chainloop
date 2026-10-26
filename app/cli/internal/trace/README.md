# Chainloop Trace: agent support

Chainloop Trace records the sessions of AI coding agents as `CHAINLOOP_AI_CODING_SESSION` evidence. Each agent is a provider in this package: `claude/`, `cursor/`, `opencode/`, `pi/` and `codex/`. The interfaces are in `provider.go`, and the registry is in `providers/`. To add a provider, follow [PROVIDERS.md](PROVIDERS.md). One provider covers OpenCode 1.x and OpenCode 2: it detects the version at runtime.

This page lists the features of Trace, by area, and tells which agents support each one. **Keep it current.** When you add or change a provider, or add or change a feature of Trace, update this page in the same pull request.

In the tables, **Yes** means supported, **Partial** means supported with limits, **No** means not supported, and **Unknown** means not verified. Below each table, "Why not Yes" tells the reason for each cell that is not **Yes**.

Claude Code has the most complete support.

Pi uses the project-local `.pi/extensions/chainloop-trace.ts` extension. Run `chainloop trace init --pi` or `chainloop trace run --local --pi -- pi …`. Review the extension before trusting the project. Interactive Pi asks before loading project-local files. A non-interactive run needs saved trust or `--approve`; Chainloop does not bypass this check. Pi must save a session, so `--no-session` cannot produce session evidence.

Pi supports the JSONL v3 format used by Pi 0.80.10. The compatibility suite pins Pi 1.3.0 with `openrouter/anthropic/claude-haiku-5.5`. Pi cells begin at **Unknown** and move only from end-to-end results or an explicit manual check.

Pi 1.3.0 passed 25 feature checks in the local end-to-end suite. Pi 0.80.10 also completed targeted TUI and JSON-headless smokes that exercised session start, `write`, `edit`, `bash`, commit attribution and session shutdown; the TUI showed the local-mode welcome message. Subagents, skills tracking, spec source pointers and pasted image pointers are not implemented. The headless suite cannot verify the UI-only welcome message, cache accounting without cache activity, the control-plane credentials warning, the session link after a control-plane push or control-plane attestation. The multi-agent feature has no registered check.

Codex uses native hooks in `.codex/hooks.json` (Codex CLI 0.163.0). Run `chainloop trace init --codex` or `chainloop trace run --local --codex -- codex exec …`. Review and trust the installed hooks in Codex before starting a session. See the [official hooks documentation](https://learn.chatgpt.com/docs/hooks). Installed hooks have a `Chainloop Trace` status label in Codex. Re-running init adds labels to existing Chainloop hooks. Changing the hooks requires a new trust review.

Codex was tested with CLI 0.163.0 and `openrouter/openai/gpt-6-luna`: all 25 tested feature checks passed in the full Codex suite. The tables keep unverified features at **Unknown**. Subagent transcripts are not implemented.

Milestone 2 live checks passed with the same CLI and model: skills in 3/3 sessions, spec pointers in 3/3 sessions, and pasted images in 1/1 session. The image scenario attaches the same image twice and requires one stored material. Transcript rendering and media redaction checks also passed. Provider and assembly regressions cover malformed images, storage failures, and reuse of an existing image material.

Codex token usage comes from unique response records, with cumulative rollout counters as a fallback for older transcripts. Codex evidence with `schema_version: v2` keeps cache reads and writes in separate fields; input and total tokens exclude cache, as they do for Claude Code. Legacy Codex `v1` counts include cache and must not be aggregated with `v2` counts without normalization. Cost includes all token categories and is an estimate from the embedded standard API prices, with a warning for unknown models. Subscription billing and third-party provider prices can differ. See [token accounting](codex/TOKEN_ACCOUNTING.md) for source selection, normalization, and the provider audit. Trace reads rollouts from `$CODEX_HOME/sessions` (default `~/.codex/sessions`) or `archived_sessions`. Their JSONL format is not a stable Codex interface.

In a linked Git worktree, Codex 0.163 reads project hooks from the primary checkout. Install the Codex hooks in that checkout too, then review and trust them in Codex.

Codex evidence converts native messages, tool calls, and tool results to the message shape used by the existing session viewer. Text uses `message.content`, calls use `tool_use` blocks, and results use `tool_result` blocks with matching IDs. Each converted entry retains its event in `source_event`. Spec and pasted-image pointers replace copies in viewer messages, retained events, and supported native timeline records. Other native metadata stays unchanged. Codex regression and E2E checks require renderable messages. When tools are used, they also require a completed exchange with readable tool output. A nonempty native transcript alone does not pass. These checks apply to Codex; `raw_session` remains open in the shared evidence schema.

## Setup

How Trace is added to a repository and to an agent.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi | Codex |
|---------|---------------|-------------|--------|--------------|------------|---|---|
| Hook installation and deletion | `chainloop trace init` installs the hooks of the selected agent. A second init changes nothing. `chainloop trace uninstall` deletes only the Chainloop hooks and keeps the other hooks of the agent. | Yes | Yes | Unknown | Yes | Yes | Yes |
| Trace run wrapper | `chainloop trace run -- <agent command>` gives the agent temporary hooks for one session. When the command exits, the configuration of the agent is the same as before. | Yes | Yes | Unknown | Yes | Yes | Yes |
| More than one agent in one repository | Two agents initialized in the same repository record their sessions separately, and each commit names the sessions of the agent that made it. | Unknown | Unknown | Unknown | Unknown | Unknown | Unknown |

Why not Yes:

- **Hook installation and deletion, and Trace run wrapper, OpenCode 1.x:** not verified yet. The end-to-end suite finds the values (spec issue-3635).
- **More than one agent in one repository, all agents:** not verified yet. The end-to-end suite needs a second agent to test it (spec issue-3635).

## Session collection

What the session evidence holds about the conversation.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi | Codex |
|---------|---------------|-------------|--------|--------------|------------|---|---|
| Transcript | The evidence holds the conversation (`raw_session`): the messages and the tool calls of the agent. | Yes | Yes | Partial | Partial | Yes | Yes |
| Token usage and cost | The evidence holds the tokens that the session used and their cost in USD. | Yes | No | Yes | Yes | Yes | Yes |
| Cache token accounting | Cache reads and writes stay in separate evidence fields. Base input and total exclude cache. The suite compares stored token counts with the raw usage records. | Unknown | No | Unknown | Unknown | Unknown | Unknown |
| Model detection | The evidence names the primary model and the other models that the session used. | Yes | Partial | Yes | Yes | Yes | Yes |
| Subagents | The evidence holds the conversations of the subagents, and their token usage, as part of the session. | Yes | No | No | No | No | No |
| Missed session start | A session whose session-start hook did not fire is still recorded, from its first tool hook. | Yes | Unknown | Unknown | Yes | Yes | Unknown |

Why not Yes:

- **Subagents, Pi:** the provider records the selected root session only. Child session transcripts are not collected.
- **Missed session start, Codex:** the scenario produces evidence, but its check also needs hook messages that Codex does not report in headless mode.
- **Transcript, OpenCode:** OpenCode has no transcript file. The provider runs `opencode export` (1.x) or `opencode session export` (2) and rebuilds the transcript from the export. It keeps the text and the inputs of the tool calls. It drops the tool outputs, the reasoning and the file parts. OpenCode 2 exports also have no agent version and no session slug.
- **Token usage and cost, Cursor:** the Cursor transcript does not hold token usage. Usage and cost are zero, and the evidence has a warning that tells so. (For OpenCode, the cost is the value that OpenCode reports. For Claude Code, Chainloop computes it from its price table.)
- **Cache token accounting, Cursor:** the collected transcript does not expose usage or cache counts.
- **Cache token accounting, Claude Code and Codex:** the new check has regression coverage, but has not run in the live agent matrix. Claude's source comparison also sums repeated assistant records; it does not establish that those records are unique.
- **Cache token accounting, OpenCode:** separate cache fields are collected, but comparison against the export has not been implemented in the end-to-end check.
- **Model detection, Cursor:** the primary model comes from the session-start hook. The transcript does not tell the models, so the evidence does not list the other models. In auto mode, the hook names the model `default`, not the model that ran.
- **Subagents, Codex:** the initial provider records only the root session transcript. Child-session transcripts are not collected.
- **Subagents, Cursor:** the provider does not read subagent conversations.
- **Subagents, OpenCode:** a subagent runs in a child session with its own export. It is not part of the evidence of its parent session.
- **Missed session start, Cursor and OpenCode 1.x:** not verified yet. The end-to-end suite finds the values (spec issue-3635).

The cache accounting check reuses the edit-and-commit session, so it adds no agent runs. It compares the stored evidence with source usage, including unique Codex responses and Claude subagents, and fails if input or total includes cache or if a cache category is lost. The Codex check also rejects inflated usage by comparing with deduplicated responses or accepted cumulative snapshots. The Claude check sums all assistant records, so matching duplicated records in both source and evidence will pass; Claude deduplication remains a gap. A session with no cache activity is reported as not covered. Regression fixtures cover the compaction overcount, duplicate response IDs, resumed sessions, cumulative fallbacks, reasoning, and cache pricing. The live agent matrix must be rerun to record the new check's result.

## Code attribution

Which lines of a commit the AI wrote.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi | Codex |
|---------|---------------|-------------|--------|--------------|------------|---|---|
| Line attribution | Each changed line of a commit is attributed to the AI or to a human, from a snapshot of each file before and after the agent edits it. An added line is AI when its content matches a line that an agent edit of the session wrote. A line that a human changed is human, also when a later agent edit keeps it. A whitespace-only change keeps the owner of the line. A removed line is AI only when an agent edit removed it. In a commit of more than one session, a line that only another session wrote or removed counts for neither side in this session's evidence (spec issue-3639). | Yes | Partial | Yes | Yes | Yes | Yes |
| Partial commits | A commit that holds only part of the AI lines of a file credits the session with those lines. The other AI lines stay pending, and the next commit with them is credited to the session too (spec issue-3639). | Yes | Partial | Yes | Yes | Yes | Yes |
| Generated line counts | The trace state keeps the lines that each agent edit added and removed, also the lines that never reach a commit. The evidence reports the totals of the session in `ai_generated_lines_added` and `ai_generated_lines_removed`. A session with no commit sends no evidence, so it reports no totals (spec issue-3639). | Yes | Partial | Yes | Yes | Yes | Yes |
| Shell changes attribution | Changes that the agent makes with shell commands (for example `sed`) are attributed to the AI too, not only changes made with the file tools. | Partial | Partial | Partial | Partial | Yes | Partial |
| Same-call commit attribution | A commit made in the same shell tool call that changed and staged its files carries that session's trace trailer. | Partial | No | No | No | No | No |
| Net contribution | The added lines of a session are the lines that its commits added and that are still in the commit that the push sends. A line that a later commit replaced does not count, and each line counts one time. The removed lines are the removals of the session's commits, less the lines that a commit of the session wrote. The line counts of each file are the sum of its AI and human lines, and a file that the session created stays created when a later commit of the session changes it (spec issue-3639, issue 3674). | Partial | Partial | Partial | Partial | Yes | Partial |
| Session updates | A later commit that replaces lines of a session sends the evidence of the session again at the next push, with the new numbers. A commit in no session that replaces lines of a session joins it: the commit list of the session holds the commit, and its replacing lines count as human (spec issue-3639). | Partial | Partial | Partial | Partial | Yes | Partial |
| Line ranges | The line ranges of each file are positions in the file at the commit that the push sends. They cover the lines of the session that are still there, and each range says if its lines are AI or human. The AI ranges hold as many lines as the net AI lines added, and the human ranges as many lines as the net human lines added. A file that the pushed commit no longer holds has no ranges. A range with no attribution is an AI range at edit time. Older CLIs write only these ranges. This CLI writes them for a file that it cannot blame at the pushed commit, and for a file with edits from before line fingerprints (spec issue-3639, line ranges). | Partial | Partial | Partial | Partial | Yes | Partial |
| Renames and rewrites | The AI/human split survives a file rename, an amend, a rebase, a squash, and a cherry-pick. The lines match the trace state by content, and blame follows a renamed file to its lines from before the rename (spec issue-3639). | Partial | Partial | Partial | Partial | Yes | Partial |

Why not Yes:

- **Line attribution, partial commits and generated line counts, Cursor:** Cursor has only the `afterFileEdit` hook, which gives the edits but no snapshot before the edit. The provider rebuilds the content before the edit from the edits.
- **Net contribution and line ranges, all agents:** the push finds the last writer of each line with `git blame`. A later commit that only re-indents a line takes the line, because blame sees a change. For Cursor, the limits of line attribution apply too.
- **Session updates, all agents:** the update works only in the clone where the work happened, while the CLI keeps the trace state of the session (30 days), and only when the commits of the session are on the branch that the push sends. The push reads only the commits that are not on the default branch of the remote (`origin/HEAD`, or else `origin/main` or `origin/master`). After the commits of a session are pushed to that branch, a later commit does not update the session.
- **Renames and rewrites, all agents:** this works only in the clone where the work happened. Git finds a rename only when the file keeps at least half of its content. Two pushed commits can rename a file one after the other. A later change to the file under its middle name does not send the session again. A rename inside an amend or a squash of the session commit loses the AI lines: the new commit creates the file under its new name, so no line of it matches the trace state (issue 3686). The limits of session updates apply too. For Cursor, the limits of line attribution apply too.
- **Shell changes attribution, all agents:** the hook before a shell command records only a hash of each file, not its content. The file at the `HEAD` commit from before the command stands in for the content before the command. Uncommitted lines that an earlier agent edit recorded are not recorded again, but uncommitted human lines of a file that the command changes count as AI. Tracked files below ignored directories can be omitted from the snapshot ([issue 3700](https://github.com/chainloop-dev/chainloop/issues/3700)).
- **Same-call commit attribution, Claude Code:** supported for foreground Bash calls that use the repository's normal index. Temporary indexes such as `GIT_INDEX_FILE` and `git commit --only` are tracked in [issue 3698](https://github.com/chainloop-dev/chainloop/issues/3698). Worktree content is compared with the staged blob as raw bytes; Git attributes, line-ending normalization and clean filters are tracked in [issue 3699](https://github.com/chainloop-dev/chainloop/issues/3699). Background calls, competing input-rewriting hooks, narrow permissions and other providers are tracked in [issue 3701](https://github.com/chainloop-dev/chainloop/issues/3701).
- **Same-call commit attribution, Cursor, OpenCode, Pi and Codex:** these providers do not tag a shell command with the per-call token that git hooks use to load its open snapshot.

## User experience

What the user sees in the agent.

The Codex welcome message and session link after push were verified manually in the interactive CLI 0.163.0. Codex runs its session-start hook after the first user message, so the welcome message appears then. The local-mode E2E suite does not verify these interactive messages.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi | Codex |
|---------|---------------|-------------|--------|--------------|------------|---|---|
| Welcome message | At session start, the user sees a message that tells that Chainloop records the session, and where the evidence goes: the Chainloop project, or the on-disk path in local mode (spec issue-3582). | Yes | No | No | No | Unknown | Yes |
| Session link after push | After a `git push` that the agent runs, the user sees a link to the session in Chainloop. | Yes | No | No | No | Unknown | Yes |
| Credentials warning | At session start, in control-plane mode, the user is told when the control plane does not accept the CLI credentials (for example, an expired token), so the push will not upload the session. The hook uses the same check as `chainloop trace init`. An unreachable control plane, or a check that takes more than 2 seconds, shows no warning. | Yes | Partial | Partial | Partial | Unknown | Unknown |

Why not Yes:

- **Welcome message, Pi:** an interactive Pi 0.80.10 smoke showed the banner, but the headless suite cannot verify Pi's UI-only notification.
- **Session link after push and credentials warning, Pi:** the local end-to-end suite cannot cover these control-plane features.
- **Credentials warning, Codex:** this check needs a control plane. The suite uses local storage only, and the warning has not been verified manually.
- **Welcome message, Cursor and OpenCode:** the provider gives the session-start context to the model only. It has no channel that shows a message to the user at session start, so the welcome message is dropped.
- **Credentials warning, Cursor and OpenCode:** the provider has no channel to the user at session start. The model is told to tell the user, so the warning depends on the model.
- **Session link after push, Cursor:** the `postToolUse` hook runs after a shell command, but its response has no field that Cursor shows to the user. The link stays pending until it expires.
- **Session link after push, OpenCode:** the plugin hook fires after a shell command, but the response that shows a message to the user is not verified yet. The link stays pending until it expires.

## Specs and skills

What the session was asked to build, and which skills it used.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi | Codex |
|---------|---------------|-------------|--------|--------------|------------|---|---|
| Spec capture instruction | At session start, the agent is told to capture the specs of the session (tickets, documents, plans, images) in the spec folder. A plan is captured only when it comes from an external source or from the plan mode of the agent: an agreement in the chat is not a plan. A pasted image is not captured, also when the agent has a file path for it. The agent tells the user in one line when it captures a file. | Yes | Yes | Yes | Yes | Yes | Yes |
| Spec capture reminder | At each user prompt, the agent is reminded to capture new or changed specs. | Yes | No | Yes | Yes | Yes | Unknown |
| Local spec sources read at push | A capture whose source is a local file holds only a header and a placeholder. Each push reads the file and records its current content, so the evidence follows the edits of the file. Only a regular text file of up to 1 MiB is read. Otherwise the push keeps the body that the agent wrote, or drops the capture with a warning when there is no body (spec issue-3561). | Yes | Yes | Yes | Yes | Yes | Yes |
| Spec source role, title, and description | Each captured spec source holds the role, the title, and the description that the agent wrote in its header (spec issue-3531). | Yes | Yes | Unknown | Yes | Yes | Yes |
| HTML spec sources | A captured text spec source whose body is an HTML page (it starts with a doctype or an `html` element) is stored as an `.html` file that holds only the page, without the header. The annotations and the spec entry keep the values of the header. A body that only contains HTML tags stays as the agent wrote it (issue 3707). | Yes | Yes | Unknown | Yes | Yes | Unknown |
| Skills tracking | The evidence lists the skills that the session used, with a copy of each skill as it ran. | Yes | No | Partial | Partial | No | Partial |
| Spec source pointers | Exact copies of spec sources in the transcript (full file reads and file writes) are replaced with a pointer to the spec material, so the evidence does not hold the same content two or three times (spec issue-3556). This includes the copies of pasted images and spec sources in the `api-request-blob` records of a Claude Code transcript (issue 3654). | Yes | No | No | No | No | Partial |
| Pasted image pointers | Each distinct image that the user pasted into the session, including images in prompts sent while the agent was busy, is stored once as an image spec entry, from the data in the transcript, with the title "Pasted image N" and no role. A repeated paste of the same image adds no entry. The inline image in the transcript is replaced with a pointer to that material. An image whose data cannot be decoded, or that the push could not store, stays inline, and the push continues. When the agent captured the same image, its entry is kept and no second entry is added (spec issue-3569). | Yes | No | No | No | No | Yes |

Why not Yes:

- **Skills tracking, Pi:** the provider does not track Pi skills, so the evidence has no skill entries.
- **Spec source pointers and pasted image pointers, Pi:** the provider has no finder for Pi transcript copies or pasted-image records.
- **Spec capture reminder, Codex:** Codex does not report hook messages in its headless output, so the suite cannot inspect the reminder.
- **Spec capture reminder, Cursor:** Cursor has no hook at each user prompt. The agent gets the session-start instruction only.
- **Spec source role, title, and description, OpenCode 1.x:** not verified yet. The end-to-end suite finds the values (spec issue-3635).
- **HTML spec sources, OpenCode 1.x and Codex:** not verified yet. The end-to-end suite finds the values.
- **Skills tracking, Codex:** tracks filesystem skills loaded through explicit Codex skill instructions or successful shell reads of one identified `SKILL.md`. The definition and supporting files use the shared skill snapshot and packaging pipeline. Multiple distinct skill paths in one tool call cannot be attributed reliably and are not tracked. Skills without a filesystem folder and child sessions are not captured. A snapshot first made at push carries a warning.
- **Spec source pointers, Codex:** supports exact text results, native shell envelopes, recognized code-mode JSON envelopes, and complete file creation through native patches or literal patches in code mode. A code-mode write must be a single awaited `tools.apply_patch` call, optionally wrapped in `text`, with a literal argument or an immediately preceding `const` literal. Stored source files read with `cat` can also match within combined command output and native file-change records. Mixed or computed patches, other scripts, arbitrary output wrappers, transformed content, and other unsupported formats remain inline, so the table remains Partial. Earlier live sessions demonstrated this limit when the model trimmed the source's trailing newline.
- **Pasted image pointers, Codex:** extracts base64 image data URLs from user input, deduplicates their decoded bytes, and replaces stored copies in both viewer messages and retained events. Tool-result images are not pastes. Invalid data, remote URLs, and path-only image records remain inline.
- **Skills tracking, Cursor:** the provider does not track skills. The evidence has no skill entries, which means "not recorded", not "no skill used".
- **Skills tracking, OpenCode:** only the skills that the model starts with the `skill` tool are counted. Skills that the user starts, and skills used in subagents, are not. A plugin installed by an earlier version does not send the skill folder for OpenCode 2: install the hooks again to update it.
- **Spec source pointers, Cursor and OpenCode:** the provider has no finder for the copies in its transcript yet. For OpenCode, the rebuilt transcript holds no tool outputs, so only the write copies would apply.
- **Pasted image pointers, Cursor and OpenCode:** the provider has no finder for pasted images yet. Each agent puts pasted images in a different block shape. The pasted images stay inline in the transcript. The agent is told not to capture them in the spec folder.

## Security

How the evidence is protected before it leaves the machine.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi | Codex |
|---------|---------------|-------------|--------|--------------|------------|---|---|
| Secret redaction | Secrets in the session evidence and in the spec materials are replaced with a placeholder before upload. | Yes | Yes | Yes | Yes | Yes | Yes |
| Media skipped by redaction | The secret redaction does not scan base64 media (images, PDFs) in the transcript, so it is faster and does not break the media. | Yes | Unknown | Not applicable | Not applicable | Yes | Yes |
| Allow markers in redaction | A secret on a line that carries an inline `gitleaks:allow` or `betterleaks:allow` marker, such as a test fixture the agent read, is not redacted. The marker covers only its own line. | Yes | Yes | Yes | Yes | Yes | Yes |

Why not Yes:

- **Media skipped by redaction, Cursor:** the Cursor transcript holds no tool outputs, so a file that the agent reads leaves no media in it. A pasted image stays inline in the transcript, but it is not verified that it is in the base64 media shape that the redaction skips. The headless Cursor CLI cannot attach an image, so the end-to-end suite cannot check it.
- **Media skipped by redaction, OpenCode:** the rebuilt transcript holds no media.

## Git integration

How the sessions are linked to the commits and attested.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi | Codex |
|---------|---------------|-------------|--------|--------------|------------|---|---|
| Commit trailer | Each commit gets a `Chainloop-Trace-Sessions` trailer with the sessions that contributed to it. | Yes | Yes | Yes | Yes | Yes | Yes |
| Attestation at push | In the default control-plane storage mode, the `pre-push` hook attests the sessions of the pushed commits to the control plane. | Yes | Yes | Yes | Yes | Unknown | Unknown |
| Local storage at push | In a repository with `traceStorage: LOCAL` in `.chainloop.yml`, the `pre-push` hook runs the same assembly as an attestation. It writes the session evidence set to the `local-sessions` folder of the trace directory and sends no attestation. It needs no control plane, login, or network. The stored set is equal to the upload of a push, except for the signature. A `statement.json` file indexes the set: the in-toto statement that a push would sign, with the `chainloop.dev/attestation/v0.2` predicate. It has no envelope and no signature, so it is not an attestation. `trace init --local` sets up the mode. `trace run --local` stores one wrapped session the same way, with no `.chainloop.yml` setting. Redaction is on by default. Set `CHAINLOOP_TRACE_NO_REDACT` to turn it off, for a trusted local store only (spec issue-3582). | Yes | Yes | Yes | Yes | Yes | Yes |
| Shell commands in another checkout | A session can work in a checkout other than the one it started in through shell commands only, for example in a fresh clone. The checkout is found from the command text: the target of `cd` or `pushd`, the path of `git -C`, and the folder that `git clone` or `gh repo clone` creates. The git hooks are installed there and the session is recorded there, the same as for a file tool edit in that checkout. A clone gets the git hooks only after the command that created it, so a commit in that same command does not carry the session. For OpenCode, a plugin installed by an earlier version does not send the command text: install the hooks again to update it. | Yes | Partial | Yes | Yes | Yes | Yes |

### Switching storage modes

`traceStorage` in `.chainloop.yml` is a committed repository policy. It is the same for every clone. When a repository uses `LOCAL`, Trace stores every session of every user on disk. To change the mode:

- **To local:** run `chainloop trace init --local`. It writes `traceStorage: LOCAL`, installs the hooks, and does not connect to a control plane. Commit `.chainloop.yml` to share the change.
- **Back to the control plane:** run `chainloop trace init` without `--local`. It writes `traceStorage: CONTROL_PLANE` and sets up the workflow. The command tells you about the change. Commit `.chainloop.yml`.

`tracePath` in `.chainloop.yml` sets the trace directory, where Chainloop Trace keeps its data in every mode. The default is `.git/chainloop-trace`. A relative path starts at the repository root. `chainloop trace init --trace-path <dir>` writes it. A directory in the working tree must be in `.gitignore`.

Run `chainloop trace init` one time for each repository. Git does not clone the git hooks in `.git`. In a clone, the first traced session installs them, so a teammate does not run init.

Why not Yes:

- **Attestation at push, Pi:** the suite uses local storage only and cannot verify control-plane uploads.
- **Attestation at push, Codex:** the suite uses local storage only and cannot verify control-plane uploads.
- **Shell commands in another checkout, Cursor:** a relative path in the command resolves against the first workspace root, not against the folder where the shell runs.

## How each agent is hooked in

| Agent | Hook installation | Hooks |
|-------|-------------------|-------|
| Claude Code | `.claude/settings.json` | Session start and end, user prompt, and before and after each file and shell tool and the Skill tool. |
| Cursor | `.cursor/hooks.json` | Session start and end, `afterFileEdit`, and `preToolUse` and `postToolUse` for the `Shell` tool. The `preToolUse` hook always allows the command. The provider installs no prompt hook. |
| Codex | `.codex/hooks.json` | `SessionStart`, `SessionEnd`, `UserPromptSubmit`, `PreToolUse` and `PostToolUse` for `Bash` and `apply_patch`. Patches use one snapshot per affected file, including both paths of a move. |
| OpenCode | `.opencode/plugins/chainloop-trace.ts` | Session start and end, user prompt, and before and after each tool. One plugin covers both versions: `server()` for 1.x (1.3.4 or later) and `setup()` for 2. The provider picks the export command from `opencode --version`. |
| Pi | `.pi/extensions/chainloop-trace.ts` | Session start and selected shutdown reasons, session-tree changes, every user prompt, and before and after the `write`, `edit` and `bash` tools. |

All agents share the git hooks `commit-msg`, `post-commit` and `pre-push`.

## End-to-end tests

The `e2e/` package checks the tables against real agents and models (spec issue-3635). It creates throwaway repositories and uses Trace in local storage mode. It currently runs the Claude Code, Cursor, OpenCode 2, Pi, and Codex columns.

The harnesses run as follows:

- OpenCode runs on a private server (`opencode run --standalone`) with `PWD` set to the workspace.
- Cursor runs with `cursor-agent -p --output-format stream-json --force --trust` and an isolated `HOME`. Shell startup files restore the test CLI to the front of `PATH`, including after macOS `path_helper` runs. The isolated home has no stored login, so the run needs `CURSOR_API_KEY`. The headless CLI cannot attach a pasted image or report hook messages.
- Pi runs with `pi -p --mode json --approve`.
- Codex runs with `codex exec --json`, an isolated `CODEX_HOME`, no sandbox, and no hook trust prompt. The isolated configuration disables personal and bundled skills while keeping project skills available. For bundled OpenAI models on OpenRouter, the harness preserves native instructions and patch tools by aliasing the model metadata to OpenRouter's prefixed model ID.

The configured OpenCode 2, Pi, and Codex models use OpenRouter and need `OPENROUTER_API_KEY`. A Codex model without the `openrouter/` prefix uses the default Codex provider and needs `CODEX_API_KEY`. The pinned Codex model is `openrouter/openai/gpt-6-luna`. GPT-5 Mini frequently ended the scripted task before committing and pushing, so it is not the default test model.

Some scenarios add the git commands of a human after the session, for example a commit that replaces an AI line, an amend, or a rename, and then push again. They work on a feature branch, because the push reads only the commits that are not on the default branch of the remote.

A feature row maps to its checks by the feature name. When you rename a feature in a table, rename it in `e2e/checks.go` too. The suite tests a cell marked **Yes**, **Partial**, or **Unknown** only when the row has a registered check. It reports the other cells as follows:

- A cell marked **No** or **Not applicable** is `skipped`.
- A feature that needs a control plane is `not covered`, because the suite uses local storage mode only.
- A row with no registered check is `no check`.

`e2e/config.yaml` sets the version of each agent that the suite expects, and the models. The suite does not install the agent. It runs the installed version, and it logs a warning when that version is not the one in the configuration. The suite uses model tokens. To run it:

```bash
make -C app/cli test-trace-e2e
```

To run Codex only, set `OPENROUTER_API_KEY` in your environment, then run:

```bash
CHAINLOOP_E2E_HARNESS=Codex make -C app/cli test-trace-e2e
```

To run Pi only, set `OPENROUTER_API_KEY` in your environment, then run:

```bash
CHAINLOOP_E2E_HARNESS=Pi make -C app/cli test-trace-e2e
```

To retain the workspace and agent output for a failure:

```bash
CHAINLOOP_E2E_HARNESS=Codex \
CHAINLOOP_E2E_WORK_DIR="$(mktemp -d)" \
make -C app/cli test-trace-e2e
```

Each scenario saves `harness-output.jsonl` next to its repository. The process fails when a **Yes** or **Partial** cell fails. Failures in **Unknown** cells appear in the result matrix without failing the process.

`CHAINLOOP_E2E_HARNESS`, `CHAINLOOP_E2E_MODEL`, and `CHAINLOOP_E2E_FEATURE` limit a run to some columns, models, or features. Separate the values with `;`, because feature names can hold commas. Set `CHAINLOOP_E2E_WORK_DIR` to keep the workspaces and the agent output. The results go to `e2e/results/`, or to `CHAINLOOP_E2E_RESULTS_DIR`. Only a run with no filter becomes the previous result of the next comparison.
