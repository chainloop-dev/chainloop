# Chainloop Trace: agent support

Chainloop Trace records the sessions of AI coding agents as `CHAINLOOP_AI_CODING_SESSION` evidence. Each agent is a provider in this package: `claude/`, `cursor/` and `opencode/`. The interfaces are in `provider.go`, and the registry is in `providers/`. One provider covers OpenCode 1.x and OpenCode 2: it detects the version at runtime.

This page lists the features of Trace, by area, and tells which agents support each one. **Keep it current.** When you add or change a provider, or add or change a feature of Trace, update this page in the same pull request.

In the tables, **Yes** means supported, **Partial** means supported with limits, **No** means not supported, and **Unknown** means not verified. Below each table, "Why not Yes" tells the reason for each cell that is not **Yes**.

Claude Code has the most complete support.

Trace does not support Pi yet (spec issue-3520): `chainloop trace init --pi` and `trace run --pi` do not exist, so no Pi session is recorded. Each Pi cell is **No** for this reason, and the "Why not Yes" lists do not repeat it. When Pi support is added, set the Pi cells to **Unknown** and run the end-to-end suite to find the values.

## Setup

How Trace is added to a repository and to an agent.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi |
|---------|---------------|-------------|--------|--------------|------------|---|
| Hook installation and deletion | `chainloop trace init` installs the hooks of the selected agent. A second init changes nothing. `chainloop trace uninstall` deletes only the Chainloop hooks and keeps the other hooks of the agent. | Yes | Unknown | Unknown | Yes | No |
| Trace run wrapper | `chainloop trace run -- <agent command>` gives the agent temporary hooks for one session. When the command exits, the configuration of the agent is the same as before. | Yes | Unknown | Unknown | Yes | No |
| More than one agent in one repository | Two agents initialized in the same repository record their sessions separately, and each commit names the sessions of the agent that made it. | Unknown | Unknown | Unknown | Unknown | No |

Why not Yes:

- **Hook installation and deletion, and Trace run wrapper, Cursor and OpenCode:** not verified yet. The end-to-end suite finds the values (spec issue-3635).
- **More than one agent in one repository, all agents:** not verified yet. The end-to-end suite needs a second agent to test it (spec issue-3635).

## Session collection

What the session evidence holds about the conversation.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi |
|---------|---------------|-------------|--------|--------------|------------|---|
| Transcript | The evidence holds the conversation (`raw_session`): the messages and the tool calls of the agent. | Yes | Yes | Partial | Partial | No |
| Token usage and cost | The evidence holds the tokens that the session used and their cost in USD. | Yes | No | Yes | Yes | No |
| Model detection | The evidence names the primary model and the other models that the session used. | Yes | Partial | Yes | Yes | No |
| Subagents | The evidence holds the conversations of the subagents, and their token usage, as part of the session. | Yes | No | No | No | No |
| Missed session start | A session whose session-start hook did not fire is still recorded, from its first tool hook. | Yes | Unknown | Unknown | Yes | No |

Why not Yes:

- **Transcript, OpenCode:** OpenCode has no transcript file. The provider runs `opencode export` (1.x) or `opencode session export` (2) and rebuilds the transcript from the export. It keeps the text and the inputs of the tool calls. It drops the tool outputs, the reasoning and the file parts. OpenCode 2 exports also have no agent version and no session slug.
- **Token usage and cost, Cursor:** the Cursor transcript does not hold token usage. Usage and cost are zero, and the evidence has a warning that tells so. (For OpenCode, the cost is the value that OpenCode reports. For Claude Code, Chainloop computes it from its price table.)
- **Model detection, Cursor:** the primary model comes from the session-start hook. The transcript does not tell the models, so the evidence does not list the other models.
- **Subagents, Cursor:** the provider does not read subagent conversations.
- **Subagents, OpenCode:** a subagent runs in a child session with its own export. It is not part of the evidence of its parent session.
- **Missed session start, Cursor and OpenCode 1.x:** not verified yet. The end-to-end suite finds the values (spec issue-3635).

## Code attribution

Which lines of a commit the AI wrote.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi |
|---------|---------------|-------------|--------|--------------|------------|---|
| Line attribution | Each changed line of a commit is attributed to the AI or to a human, from a snapshot of each file before and after the agent edits it. An added line is AI when its content matches a line that an agent edit of the session wrote. A line that a human changed is human, also when a later agent edit keeps it. A whitespace-only change keeps the owner of the line. A removed line is AI only when an agent edit removed it. In a commit of more than one session, a line that only another session wrote or removed counts for neither side in this session's evidence (spec issue-3639). | Yes | Partial | Yes | Yes | No |
| Partial commits | A commit that holds only part of the AI lines of a file credits the session with those lines. The other AI lines stay pending, and the next commit with them is credited to the session too (spec issue-3639). | Yes | Partial | Yes | Yes | No |
| Generated line counts | The trace state keeps the lines that each agent edit added and removed, also the lines that never reach a commit. The evidence does not report them yet (spec issue-3639). | Yes | Partial | Yes | Yes | No |
| Shell changes attribution | Changes that the agent makes with shell commands (for example `sed`) are attributed to the AI too, not only changes made with the file tools. | Partial | Partial | Partial | Partial | No |
| Net contribution | The added lines of a session are the lines that its commits added and that are still in the commit that the push sends. A line that a later commit replaced does not count, and each line counts one time. The removed lines are the sum of the removals of the session's commits (spec issue-3639). | Partial | Partial | Partial | Partial | No |
| Session updates | A later commit that replaces lines of a session sends the evidence of the session again at the next push, with the new numbers. A commit in no session that replaces lines of a session joins it: the commit list of the session holds the commit, and its replacing lines count as human (spec issue-3639). | Partial | Partial | Partial | Partial | No |
| Line ranges | The line ranges of each file are positions in the file at the commit that the push sends. They cover the lines of the session that are still there, and each range says if its lines are AI or human. The AI ranges hold as many lines as the net AI lines added, and the human ranges as many lines as the net human lines added. A file that the pushed commit no longer holds has no ranges. A range with no attribution is an AI range at edit time. Older CLIs write only these ranges. This CLI writes them for a file that it cannot blame at the pushed commit, and for a file with edits from before line fingerprints (spec issue-3639, line ranges). | Partial | Partial | Partial | Partial | No |
| Renames and rewrites | The AI/human split survives a file rename, an amend, a rebase, a squash, and a cherry-pick. The lines match the trace state by content, and blame follows a renamed file to its lines from before the rename (spec issue-3639). | Partial | Partial | Partial | Partial | No |

Why not Yes:

- **Line attribution, partial commits and generated line counts, Cursor:** Cursor has only the `afterFileEdit` hook, which gives the edits but no snapshot before the edit. The provider rebuilds the content before the edit from the edits.
- **Net contribution and line ranges, all agents:** the push finds the last writer of each line with `git blame`. A later commit that only re-indents a line takes the line, because blame sees a change. For Cursor, the limits of line attribution apply too.
- **Session updates, all agents:** the update works only in the clone where the work happened, while the CLI keeps the trace state of the session (30 days), and only when the commits of the session are on the branch that the push sends.
- **Renames and rewrites, all agents:** this works only in the clone where the work happened. Git finds a rename only when the file keeps at least half of its content. Two pushed commits can rename a file one after the other. A later change to the file under its middle name does not send the session again. For Cursor, the limits of line attribution apply too.
- **Shell changes attribution, all agents:** the hook before a shell command records only a hash of each file, not its content. The file at the `HEAD` commit from before the command stands in for the content before the command. Uncommitted lines that an earlier agent edit recorded are not recorded again, but uncommitted human lines of a file that the command changes count as AI.
- **Shell changes attribution, Cursor:** the provider installs the `preToolUse` and `postToolUse` hooks for the `Shell` tool. These hooks are verified for the Cursor IDE only. It is not verified that the Cursor CLI sends them.

## User experience

What the user sees in the agent.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi |
|---------|---------------|-------------|--------|--------------|------------|---|
| Welcome message | At session start, the user sees a message that tells that Chainloop records the session, and where the evidence goes: the Chainloop project, or the on-disk path in local mode (spec issue-3582). | Yes | No | No | No | No |
| Session link after push | After a `git push` that the agent runs, the user sees a link to the session in Chainloop. | Yes | No | No | No | No |
| Credentials warning | At session start, in control-plane mode, the user is told when the control plane does not accept the CLI credentials (for example, an expired token), so the push will not upload the session. The hook uses the same check as `chainloop trace init`. An unreachable control plane, or a check that takes more than 2 seconds, shows no warning. | Yes | Partial | Partial | Partial | No |

Why not Yes:

- **Welcome message, Cursor and OpenCode:** the provider gives the session-start context to the model only. It has no channel that shows a message to the user at session start, so the welcome message is dropped.
- **Credentials warning, Cursor and OpenCode:** the provider has no channel to the user at session start. The model is told to tell the user, so the warning depends on the model.
- **Session link after push, Cursor:** the `postToolUse` hook runs after a shell command, but its response has no field that Cursor shows to the user. The link stays pending until it expires.
- **Session link after push, OpenCode:** the plugin hook fires after a shell command, but the response that shows a message to the user is not verified yet. The link stays pending until it expires.

## Specs and skills

What the session was asked to build, and which skills it used.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi |
|---------|---------------|-------------|--------|--------------|------------|---|
| Spec capture instruction | At session start, the agent is told to capture the specs of the session (tickets, documents, plans, images) in the spec folder. A plan is captured only when it comes from an external source or from the plan mode of the agent: an agreement in the chat is not a plan. A pasted image is not captured, also when the agent has a file path for it. The agent tells the user in one line when it captures a file. | Yes | Yes | Yes | Yes | No |
| Spec capture reminder | At each user prompt, the agent is reminded to capture new or changed specs. | Yes | No | Yes | Yes | No |
| Local spec sources read at push | A capture whose source is a local file holds only a header and a placeholder. Each push reads the file and records its current content, so the evidence follows the edits of the file. Only a regular text file of up to 1 MiB is read. Otherwise the push keeps the body that the agent wrote, or drops the capture with a warning when there is no body (spec issue-3561). | Yes | Yes | Yes | Yes | No |
| Spec source role, title, and description | Each captured spec source holds the role, the title, and the description that the agent wrote in its header (spec issue-3531). | Yes | Unknown | Unknown | Yes | No |
| Skills tracking | The evidence lists the skills that the session used, with a copy of each skill as it ran. | Yes | No | Partial | Partial | No |
| Spec source pointers | Exact copies of spec sources in the transcript (full file reads and file writes) are replaced with a pointer to the spec material, so the evidence does not hold the same content two or three times (spec issue-3556). This includes the copies of pasted images and spec sources in the `api-request-blob` records of a Claude Code transcript (issue 3654). | Yes | No | No | No | No |
| Pasted image pointers | Each distinct image that the user pasted into the session, including images in prompts sent while the agent was busy, is stored once as an image spec entry, from the data in the transcript, with the title "Pasted image N" and no role. A repeated paste of the same image adds no entry. The inline image in the transcript is replaced with a pointer to that material. An image whose data cannot be decoded, or that the push could not store, stays inline, and the push continues. When the agent captured the same image, its entry is kept and no second entry is added (spec issue-3569). | Yes | No | No | No | No |

Why not Yes:

- **Spec capture reminder, Cursor:** Cursor has no hook at each user prompt. The agent gets the session-start instruction only.
- **Spec source role, title, and description, Cursor and OpenCode 1.x:** not verified yet. The end-to-end suite finds the values (spec issue-3635).
- **Skills tracking, Cursor:** the provider does not track skills. The evidence has no skill entries, which means "not recorded", not "no skill used".
- **Skills tracking, OpenCode:** only the skills that the model starts with the `skill` tool are counted. Skills that the user starts, and skills used in subagents, are not. A plugin installed by an earlier version does not send the skill folder for OpenCode 2: install the hooks again to update it.
- **Spec source pointers, Cursor and OpenCode:** the provider has no finder for the copies in its transcript yet. For OpenCode, the rebuilt transcript holds no tool outputs, so only the write copies would apply.
- **Pasted image pointers, Cursor and OpenCode:** the provider has no finder for pasted images yet. Each agent puts pasted images in a different block shape. The pasted images stay inline in the transcript. The agent is told not to capture them in the spec folder.

## Security

How the evidence is protected before it leaves the machine.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi |
|---------|---------------|-------------|--------|--------------|------------|---|
| Secret redaction | Secrets in the session evidence and in the spec materials are replaced with a placeholder before upload. | Yes | Yes | Yes | Yes | No |
| Media skipped by redaction | The secret redaction does not scan base64 media (images, PDFs) in the transcript, so it is faster and does not break the media. | Yes | Unknown | Not applicable | Not applicable | No |
| Allow markers in redaction | A secret on a line that carries an inline `gitleaks:allow` or `betterleaks:allow` marker, such as a test fixture the agent read, is not redacted. The marker covers only its own line. | Yes | Yes | Yes | Yes | No |

Why not Yes:

- **Media skipped by redaction, Cursor:** the redaction skips base64 media blocks of the shape `{"type":"base64","data":...}`. The Cursor transcript is stored as it is, and it is not verified whether it holds media in that shape.
- **Media skipped by redaction, OpenCode:** the rebuilt transcript holds no media.

## Git integration

How the sessions are linked to the commits and attested.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi |
|---------|---------------|-------------|--------|--------------|------------|---|
| Commit trailer | Each commit gets a `Chainloop-Trace-Sessions` trailer with the sessions that contributed to it. | Yes | Yes | Yes | Yes | No |
| Attestation at push | In the default control-plane storage mode, the `pre-push` hook attests the sessions of the pushed commits to the control plane. | Yes | Yes | Yes | Yes | No |
| Local storage at push | In a repository with `traceStorage: LOCAL` in `.chainloop.yml`, the `pre-push` hook runs the same assembly as an attestation. It writes the session evidence set to the `local-sessions` folder of the trace directory and sends no attestation. It needs no control plane, login, or network. The stored set is equal to the upload of a push, except for the signature. A `statement.json` file indexes the set: the in-toto statement that a push would sign, with the `chainloop.dev/attestation/v0.2` predicate. It has no envelope and no signature, so it is not an attestation. `trace init --local` sets up the mode. `trace run --local` stores one wrapped session the same way, with no `.chainloop.yml` setting. Redaction is on by default. Set `CHAINLOOP_TRACE_NO_REDACT` to turn it off, for a trusted local store only (spec issue-3582). | Yes | Yes | Yes | Yes | No |
| Shell commands in another checkout | A session can work in a checkout other than the one it started in through shell commands only, for example in a fresh clone. The checkout is found from the command text: the target of `cd` or `pushd`, the path of `git -C`, and the folder that `git clone` or `gh repo clone` creates. The git hooks are installed there and the session is recorded there, the same as for a file tool edit in that checkout. A clone gets the git hooks only after the command that created it, so a commit in that same command does not carry the session. For OpenCode, a plugin installed by an earlier version does not send the command text: install the hooks again to update it. | Yes | Partial | Yes | Yes | No |

### Switching storage modes

`traceStorage` in `.chainloop.yml` is a committed repository policy. It is the same for every clone. When a repository uses `LOCAL`, Trace stores every session of every user on disk. To change the mode:

- **To local:** run `chainloop trace init --local`. It writes `traceStorage: LOCAL`, installs the hooks, and does not connect to a control plane. Commit `.chainloop.yml` to share the change.
- **Back to the control plane:** run `chainloop trace init` without `--local`. It writes `traceStorage: CONTROL_PLANE` and sets up the workflow. The command tells you about the change. Commit `.chainloop.yml`.

`tracePath` in `.chainloop.yml` sets the trace directory, where Chainloop Trace keeps its data in every mode. The default is `.git/chainloop-trace`. A relative path starts at the repository root. `chainloop trace init --trace-path <dir>` writes it. A directory in the working tree must be in `.gitignore`.

Run `chainloop trace init` one time for each repository. Git does not clone the git hooks in `.git`. In a clone, the first traced session installs them, so a teammate does not run init.

Why not Yes:

- **Shell commands in another checkout, Cursor:** the shell hooks are verified for the Cursor IDE only. A relative path in the command resolves against the first workspace root, not against the folder where the shell runs.
## How each agent is hooked in

| Agent | Hook installation | Hooks |
|-------|-------------------|-------|
| Claude Code | `.claude/settings.json` | Session start and end, user prompt, and before and after each file and shell tool and the Skill tool. |
| Cursor | `.cursor/hooks.json` | Session start and end, `afterFileEdit`, and `preToolUse` and `postToolUse` for the `Shell` tool. The `preToolUse` hook always allows the command. The provider installs no prompt hook. |
| OpenCode | `.opencode/plugins/chainloop-trace.ts` | Session start and end, user prompt, and before and after each tool. One plugin covers both versions: `server()` for 1.x (1.3.4 or later) and `setup()` for 2. The provider picks the export command from `opencode --version`. |

All agents share the git hooks `commit-msg`, `post-commit` and `pre-push`.

## End-to-end tests

The `e2e/` package checks the tables of this page against real agents (spec issue-3635). It runs each agent in headless mode with real models, in throwaway repositories with Trace in local storage mode. For now, it runs the Claude Code, OpenCode 2, and Pi columns. The Pi cells are **No**, so the suite skips them until Pi support is added. OpenCode runs on a private server (`opencode run --standalone`) with `PWD` set to the workspace, because OpenCode takes its project folder from `PWD`. Pi runs with `pi -p --mode json --approve`, with its configuration and session folders next to the workspace. The OpenCode 2 and Pi models in `e2e/config.yaml` are OpenRouter models, so the run needs `OPENROUTER_API_KEY`.

A feature row maps to its checks by the feature name. When you rename a feature in a table, rename it in `e2e/checks.go` too. The suite tests a cell marked **Yes**, **Partial**, or **Unknown** only when the row has a registered check. It reports the other cells as follows:

- A cell marked **No** or **Not applicable** is `skipped`.
- A feature that needs a control plane is `not covered`, because the suite uses local storage mode only.
- A row with no registered check is `no check`.

`e2e/config.yaml` sets the version of each agent that the suite expects, and the models. The suite does not install the agent. It runs the installed version, and it logs a warning when that version is not the one in the configuration. The suite uses model tokens. To run it:

```bash
make -C app/cli test-trace-e2e
```

`CHAINLOOP_E2E_HARNESS`, `CHAINLOOP_E2E_MODEL`, and `CHAINLOOP_E2E_FEATURE` limit a run to some columns, models, or features. Separate the values with `;`, because feature names can hold commas. Set `CHAINLOOP_E2E_WORK_DIR` to keep the workspaces and the agent output. The results go to `e2e/results/`, or to `CHAINLOOP_E2E_RESULTS_DIR`. Only a run with no filter becomes the previous result of the next comparison.
