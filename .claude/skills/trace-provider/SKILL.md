---
name: trace-provider
description: Use to add a Chainloop Trace provider for a new AI coding agent (harness), such as Codex or Pi. Also use to change a provider in app/cli/internal/trace (claude, cursor, opencode). Covers the provider interfaces, hooks, trace init flags and wizard, and code attribution. Also covers session parsing (models, tokens, cost), local mode, end-to-end tests, and the compatibility matrix.
---

# Chainloop Trace provider

Read `app/cli/internal/trace/PROVIDERS.md` before you write code. It is the guide to implement a provider. It tells:

- The session lifecycle, and which provider method runs at each step.
- The `trace.Provider` methods and the optional interfaces in `app/cli/internal/trace/provider.go`.
- The CLI wiring: the registry, the `--<agent>` flags of `trace init` and `trace run`, and the `trace hook <agent>` subcommands.
- The conditions for local storage mode.
- The end-to-end harness and the compatibility matrix in `app/cli/internal/trace/README.md`.

Follow the checklist at the end of `PROVIDERS.md`. Copy it into the pull request description.

Each provider must have an end-to-end harness in `app/cli/internal/trace/e2e/`. Each provider must also have a column in each feature table of `app/cli/internal/trace/README.md`, with values from the end-to-end suite.
