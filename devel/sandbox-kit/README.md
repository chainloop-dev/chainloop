# `chainloop-trace-claude` — Docker Sandboxes kit

Runs Claude Code inside a [Docker Sandbox](https://docs.docker.com/ai/sandboxes/) with `chainloop trace`
already wired in, so a session working on this repo is recorded as an
[AI coding session](https://docs.chainloop.dev/concepts/ai-coding-sessions) — model, token usage and cost,
tools and MCP servers called, AI-vs-human line attribution — and attested to Chainloop without the developer
setting anything up.

Full guide: **https://docs.chainloop.dev/guides/docker-sandboxes**

`sbx-kit-claude/spec.yaml` is the kit — a mixin on Docker's built-in `claude` agent, self-contained, no secrets,
and heavily commented; read it for the design rationale. It started life in the
`chainloop-trace-docker-sandbox` PoC repo, which additionally carries the long-form write-up and the running
list of upstream Docker bugs.

## Prerequisites

- **`sbx` v0.55.0 or later.**
- **Chainloop credentials**, stored on the host — an org-scoped API token
(`chainloop organization api-token create`) or your `chainloop auth login` session; see
[Credentials](#credentials).
- **Anthropic credentials** for Claude Code itself — a host-side secret (`sbx secret set -g anthropic`) or an
interactive login inside the sandbox. Independent of everything Chainloop.

## Two ways to run it

Every command here runs **from the repository root**, and both ways end up in the same place: this repo is
already initialized for `chainloop trace` (`.chainloop.yml` + the hooks in `.claude/settings.json`), so the
kit takes its identity — org, project, workflow — from `.chainloop.yml` and pushes the attestation on
`git push`.

**The kit supports persistent tracing only, and refuses to create a sandbox without it.** Point it at a
repository that has not been initialized and `sbx create` fails with instructions to run `chainloop trace init`
there first. The CLI's other mode, `chainloop trace run`, is deliberately not offered: it ignores
`.chainloop.yml` by design and its teardown wipes `.git/chainloop-trace/` and strips the committed hooks —
destructive on exactly the repos this kit accepts. The trade-off is that **a session whose work is never pushed
attests nothing**, so push from inside the sandbox before it is reclaimed.

Store the token once, before either way:

```bash
sbx secret set chainloop
```

### 1. Through the environment file

```bash
sbx env run
```

### 2. Driving the kit directly

```bash
sbx run --clone claude --kit ./devel/sandbox-kit/sbx-kit-claude .
```

The first `sbx create` asks you to approve the `chainloop` credential for `cp.chainloop.dev` and
`api.app.chainloop.dev`.

## Credentials

The token never enters the sandbox: the sandbox sees a placeholder, and the `sbx` egress proxy adds the real
value to the requests.

| You have | Store it with |
| --- | --- |
| An org-scoped API token | `sbx secret set chainloop` |
| A `chainloop auth login` session | `sbx secret set chainloop --refresh on-demand --command …` ([below](#using-your-login-session)) |
| A self-hosted control plane | `sbx secret set-custom` ([below](#self-hosted)) |

With none of them the sandbox refuses to start rather than run an untraced session.

### Using your login session

```bash
CFG="$HOME/Library/Application Support/chainloop/config.toml"    # macOS
# CFG="$HOME/.config/chainloop/config.toml"                      # Linux

sbx secret set chainloop --refresh on-demand \
  --command "sed -n \"s/^token = '\\(.*\\)'\$/\\1/p\" '$CFG'"
```

`sbx` reads the token on the host for every request, so a new `chainloop auth login` applies at once. The
session authenticates the sandbox as **you** and expires in days: good for a local demo, wrong for anything
scheduled or shared.

### Self-hosted

```bash
printf '%s' "$CHAINLOOP_TOKEN" | sbx secret set-custom --placeholder 'sbx-cs-{rand}' \
  --host cp.chainloop.example.com --host api.chainloop.example.com --value-stdin

sbx run --clone \
  -e CHAINLOOP_TOKEN=<the placeholder printed above> \
  -e CHAINLOOP_CONTROL_PLANE_API=cp.chainloop.example.com:443 \
  -e CHAINLOOP_ARTIFACT_CAS_API=cas.chainloop.example.com:443 \
  -e CHAINLOOP_PLATFORM_API=api.chainloop.example.com:443 \
  claude --kit ./devel/sandbox-kit/sbx-kit-claude .
```

Allow those hosts with `sbx policy allow network`. A custom CA goes in `CHAINLOOP_CONTROL_PLANE_API_CA` (and the
`_ARTIFACT_CAS_` / `_PLATFORM_` equivalents), and the host must trust it too for the proxy to add the token. If
it does not, or the control plane has no TLS, pass the real token with `-e CHAINLOOP_TOKEN=…` instead; it is
then in the sandbox's environment.

---

## Details and sharp edges

- The token is added on `cp.` and `api.app.chainloop.dev` only. The CAS uses its own short-lived token, which
  an injected one would overwrite.
- On Chainloop cloud, the kit refuses a `CHAINLOOP_TOKEN` passed with `-e` when no `chainloop` secret is
  stored: `sbx` drops it on the way out. Store it with `sbx secret set chainloop` instead.
- The CLI warns `can't extract info for the auth token: … token is malformed` on every command, because it
  only sees the placeholder, and `chainloop organization describe` shows no org. Inside the sandbox,
  `chainloop trace init` needs `--org`.
- `sbx secret set chainloop` also updates running sandboxes, so an expired token needs no new sandbox.
- **Optional:** to sign the agent's commits (the `source-commit` policy expects a signature), add
  `--kit docker.io/sbx/git-ssh-sign:latest` (`sbxenv.yaml` already does) and load your key into the host's
  `ssh-agent`.
- **This kit is a mixin**, so it goes behind `--kit` and `claude` stays the agent. A kit takes its name from its
  directory, so the directory is named after the published image, not `claude`.
