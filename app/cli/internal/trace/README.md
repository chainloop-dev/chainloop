# Chainloop Trace: agent support

Chainloop Trace records the sessions of AI coding agents as `CHAINLOOP_AI_CODING_SESSION` evidence. Each agent is a provider in this package: `claude/`, `cursor/`, `opencode/` and `codex/`. The interfaces are in `provider.go`, and the registry is in `providers/`. To add a provider, follow [PROVIDERS.md](PROVIDERS.md). One provider covers OpenCode 1.x and OpenCode 2: it detects the version at runtime.

This page lists the features of Trace, by area, and tells which agents support each one. **Keep it current.** When you add or change a provider, or add or change a feature of Trace, update this page in the same pull request.

In the tables, **Yes** means supported, **Partial** means supported with limits, **No** means not supported, and **Unknown** means not verified. Below each table, "Why not Yes" tells the reason for each cell that is not **Yes**.

Claude Code has the most complete support.

Trace does not support Pi yet (spec issue-3520): `chainloop trace init --pi` and `trace run --pi` do not exist, so no Pi session is recorded. Each Pi cell is **No** for this reason, and the "Why not Yes" lists do not repeat it. When Pi support is added, set the Pi cells to **Unknown** and run the end-to-end suite to find the values.

Codex uses native hooks in `.codex/hooks.json` (Codex CLI 0.163.0). Run `chainloop trace init --codex` or `chainloop trace run --codex -- codex exec …`. Review and trust the installed hooks in Codex before starting a session. See the [official hooks documentation](https://learn.chatgpt.com/docs/hooks). Changing the hooks requires a new trust review.

Codex was tested with CLI 0.163.0 and `openrouter/openai/gpt-6-luna`: 19 feature checks passed with no failures. The tables keep unverified features at **Unknown**. Subagent transcripts, skills tracking, spec source pointers, and pasted image pointers are not implemented.

Codex token usage comes from cumulative rollout counters. Cost is an estimate from the embedded standard API prices, with a warning for unknown models. Subscription billing and third-party provider prices can differ. Trace reads rollouts from `$CODEX_HOME/sessions` (default `~/.codex/sessions`) or `archived_sessions`. Their JSONL format is not a stable Codex interface.

## Setup

How Trace is added to a repository and to an agent.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 | Pi | Codex |
|---------|---------------|-------------|--------|--------------|------------|---|---|
| Hook installation and deletion | `chainloop trace init` installs the hooks of the selected agent. A second init changes nothing. `chainloop trace uninstall` deletes only the Chainloop hooks and keeps the other hooks of the agent. | Yes | Yes | Unknown | Yes | No | Yes |
| Trace run wrapper | `chainloop trace run -- <agent command>` gives the agent temporary hooks for one session. When the command exits, the configuration of the agent is the same as before. | Yes | Yes | Unknown | Yes | No | Yes |
| More than one agent in one repository | Two agents initialized in the same repository record their sessions separately, and each commit names the sessions of the agent that made it. | Unknown | Unknown | Unknown | Unknown | No | Unknown |
