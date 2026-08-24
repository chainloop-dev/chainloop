# `chainloop-trace-claude` — Docker Sandboxes kit

Runs Claude Code inside a [Docker Sandbox](https://docs.docker.com/ai/sandboxes/) with `chainloop trace`
already wired in, so a session working on this repo is recorded as an
[AI coding session](https://docs.chainloop.dev/concepts/ai-coding-sessions) — model, token usage and cost,
tools and MCP servers called, AI-vs-human line attribution — and attested to Chainloop without the developer
setting anything up.

`spec.yaml` here is the kit — self-contained, no secrets, and heavily commented; read it for the design
rationale, the `extends: claude` inheritance notes, and the gRPC-vs-egress-proxy analysis. It started life in
the `chainloop-trace-docker-sandbox` PoC repo, which additionally carries the long-form write-up and the
running list of upstream Docker bugs.

## Prerequisites

- **An `sbx@nightly` build — not the v0.39.0 stable release.** This is a hard requirement; see
[below](#why-nightly) for why, and for the one command that tells you whether your build is affected.
  ```bash
  brew uninstall --cask sbx           # conflicts with the nightly cask
  brew install --cask docker/tap/sbx@nightly
  sbx daemon restart                  # so the daemon matches the CLI
  ```
- **Docker running**, for the sandbox VMs.
- **Chainloop credentials** — either an org-scoped API token
(`chainloop organization api-token create`) or an existing `chainloop auth login` session. One or the other;
see [Two ways to run it](#two-ways-to-run-it).
- **Anthropic credentials** for Claude Code itself — a host-side secret (`sbx secret set -g anthropic`) or an
interactive login inside the sandbox. Independent of everything Chainloop.

## Two ways to run it

Every command here runs **from the repository root**, and both ways end up in the same place: this repo is
already initialized for `chainloop trace` (`.chainloop.yml` + the hooks in `.claude/settings.json`), so the
kit runs in **persistent** mode, takes its identity — org, project, workflow — from `.chainloop.yml`, and
pushes the attestation on `git push`.

### 1. Through the environment file

The repo's `sbxenv.yaml` declares the agent, the kit, the clone-mode workspace and the trace mode, so the
only thing left to pass is the token:

```bash
sbx env run --env-arg chainloopToken="$CHAINLOOP_TOKEN"
```

### 2. Driving the kit directly

No environment file. Either let an **exported token** be picked up automatically — a bare `-e KEY` takes the
value from your shell, and it overrides the kit's own default:

```bash
export CHAINLOOP_TOKEN=cl_...

sbx run --clone --kit ./devel/sandbox-kit -e CHAINLOOP_TOKEN chainloop-trace-claude
```

…or skip the token entirely and **authenticate from your existing `chainloop auth login` session**, by
mounting the config the CLI already wrote on your machine:

```bash
CFG="$HOME/Library/Application Support/chainloop"     # macOS
# CFG="$HOME/.config/chainloop"                       # Linux

sbx run --clone --kit ./devel/sandbox-kit \
  --kit-arg chainloopConfig="$CFG/config.toml" \
  chainloop-trace-claude \
  . "${CFG}:ro"
```

The trailing line is positional: `.` is the workspace (must be read-write), `"${CFG}:ro"` the extra read-only
mount. `sbx` mounts it at the same absolute path it has on the host, and the kit copies it to
`/home/agent/.config/chainloop/config.toml` — where the in-VM CLI looks, per `chainloop config view`. On
attach:

```
[chainloop-trace] Adopted chainloop config from /Users/…/chainloop/config.toml
[chainloop-trace] Repo already initialized for chainloop trace - persistent mode
```

### Which to use

|  | exported `$CHAINLOOP_TOKEN` | API token, explicit | `config.toml` |
| --- | --- | --- | --- |
| `sbx env run` | not supported — no `-e` flag, and nothing interpolates in the file | `--env-arg chainloopToken=…` | needs an overlay file (below) |
| `sbx run --kit` | `-e CHAINLOOP_TOKEN` | `--kit-arg chainloopToken=…` | `--kit-arg chainloopConfig=…` + a `:ro` mount |

Supply one of them. With no token **and** no config the sandbox refuses to start rather than run an untraced
session — a session that records nothing is worse than one that never began, because you only find out when
the attestation you expected isn't there.

---

## Details and sharp edges

**Why the token is passed in, and not stored in `sbx`'s secret manager.** The obvious approach —
`sbx secret set` / `sbx secret set-custom`, where the sandbox only ever holds a placeholder and the egress
proxy swaps in the real secret per request — cannot work for Chainloop today. To rewrite a header the proxy
has to terminate TLS, and its intercepted path does not negotiate the `h2` ALPN; `chainloop trace` talks to
the control plane over gRPC, and grpc-go ≥1.67 aborts with `missing selected ALPN property` rather than fall
back to HTTP/1.1. So declaring the Chainloop hosts for injection would *break* tracing rather than secure it,
and the kit keeps those hosts on `NO_PROXY` and takes the real token as a value. Until Docker Sandboxes
supports HTTP/2 gRPC through its header-rewriting proxy, the token lives in the VM's environment.

That also means a `secrets:` entry in an `sbxenv.yaml` is the wrong tool here, even though it looks
purpose-built (`ref: op://…`, `command: gh auth token`): it is keyed by *service* and delivers through that
same injection path, so it would flip the Chainloop hosts to the intercepted path and kill attestation.

**Keep the token out of your shell history and `ps`** — put it in a file instead of on the command line:

```bash
printf 'chainloopToken=%s\n' "$CHAINLOOP_TOKEN" > ~/.config/chainloop/kit-args
chmod 600 ~/.config/chainloop/kit-args

sbx run --clone --kit ./devel/sandbox-kit \
  --kit-args-file ~/.config/chainloop/kit-args \
  chainloop-trace-claude
```

`sbx env run` has the equivalent `--env-args-file`.

**Brace the variable: `"${CFG}:ro"`, never `"$CFG:ro"`.** In zsh, `$VAR:r` is the remove-extension history
modifier and it fires *even inside double quotes*, so `"$CFG:ro"` becomes `…/chainloop` **`o`**. `sbx` then
offers to create that non-existent directory; accept and you get an empty read-only mount and
`chainloopConfig points at … not readable`.

```bash
$ CFG=/tmp/demo/chainloop; echo "$CFG:ro"; echo "${CFG}:ro"
/tmp/demo/chainloopo          # zsh ate the :r
/tmp/demo/chainloop:ro        # what you meant
```

**What you are mounting.** `config.toml`'s `[auth] token` is your interactive login session, not an API
token: audience `user-auth.chainloop`, carries your `user_id`, and expires in **days**. The sandbox
authenticates as *you* and silently stops attesting when the session lapses — in persistent mode you find out
at `git push`. Good for a local demo, or for picking up self-hosted control-plane/CAS/platform endpoints
without editing the kit. Wrong for anything scheduled or shared, which is why `sbxenv.yaml` does not do it.

**Passing both is safe and sometimes useful** — the config supplies the endpoints, the token authenticates.
The CLI prefers an exported `CHAINLOOP_TOKEN` and says so: `Both user credentials and $CHAINLOOP_TOKEN set.
Ignoring user credentials.`

**The `config.toml` route with `sbx env run`** needs an overlay file, because `sbx env run` takes environment
files as its positional arguments and has no mount flag — a mount can only come from `additionalWorkspaces:`:

```bash
cat > ~/.config/chainloop/sbx-mount.sbxenv.yaml <<'EOF'
schemaVersion: "1"
additionalWorkspaces:
  - path: /Users/YOU/Library/Application Support/chainloop
    readOnly: true
EOF

sbx env rm                    # mounts are fixed at creation; an existing sandbox will not adopt one
sbx env run . ~/.config/chainloop/sbx-mount.sbxenv.yaml \
  --kit-arg chainloopConfig="$CFG/config.toml"
```

Pass that same pair of paths to every lifecycle command (`sbx env create`/`exec`/`rm`) — the path set is what
identifies the environment. Do **not** put this in `~/.sbxenv.yaml`: that file is read on every `sbx env`
call, and in a directory with no project file it is validated alone and fails with `agent is required`,
breaking unrelated commands.

**Other things that will surprise you:**

- `sbx exec <sandbox> -- chainloop …` on a sandbox you have never attached to sees no config. The copy runs
in the entrypoint wrapper, so attach once first.
- `chainloopConfig` does not appear in `sbx env plan` — the plan only renders args pinned in a file. The
value still reaches the sandbox; its absence is not a failure.
- Mounts are fixed at creation. `sbx env run` on an existing sandbox re-attaches without re-provisioning.

## Why nightly

**Not** the v0.39.0 stable release. The kit inherits the built-in `claude` agent with `extends:`, and
[docker/sbx-releases#415](https://github.com/docker/sbx-releases/issues/415) — a child `setup:` block
silently dropping the parent's — was fixed in nightly and then **regressed in the v0.39.0 release**.

```
<=v0.38.1                        BROKEN
nightly-202608180320 (rc1-245)   ok
v0.39.0 stable (def8cb0)         BROKEN AGAIN
nightly-202608240324 (rc1-441)   ok   <- verified
```

On a broken build the sandbox still starts, but `~/.claude/*` stays owned by `root:root`, so the agent cannot
write the transcript `chainloop trace` reads — **the session records nothing, with no error**. Check before
blaming anything else:

```bash
sbx exec <sandbox> -- stat -c '%U:%G' /home/agent/.claude/projects   # want agent:agent
```
