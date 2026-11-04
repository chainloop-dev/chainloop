<!--
AGENT INSTRUCTIONS. Keep this comment. Read it before you change this file.

The br:eng-adr skill writes this page. Do not edit it by hand, and do not change it as part of other work. A person approves each change before the skill writes it, and the area owner reviews it in a PR.

Structure. The index (README.md) has the parts Areas (the area map), Relationships, Repository-wide decisions, Non-Confirmed, Superseded, and Open conflicts. An area page has the parts Language, Current decisions, Non-Confirmed, Superseded, and Open conflicts. Each decision is a "### <PFX>-NNN: <title>" entry with the choice, then the lines Why, Rejected, Deciders, Source, and Code.
- Current decisions and Repository-wide decisions: the rules that hold today. Follow them. The page holds only choices that a reader of the code would likely get wrong. It does not describe how the system works: for that, read the code or the specs.
- Non-Confirmed: decisions that the code does not show yet, for example a plan in a spec. They are not rules yet.
- Superseded: old decisions, struck through, each with the ID of the decision that replaced it. They are history only.
- Open conflicts: drift and disagreements that a person must settle.
- Language: the words of the area, and the words to avoid.

- Before you change the behavior of an area, read its current decisions and its Language. The page leaves out smaller decisions (the "Not shown" line), so also check the Decision Records of the specs that the page lists. Read the linked spec for the reasons.
- To record a decision that the team made and built: run br:eng-adr new.
- To change a decision, or to make one that is not built yet: write a spec with br:eng-spec. Its Decision Record entry says "replaces <PFX>-NNN".
- To refresh this page from the specs and the code: run br:eng-adr reconcile.
- Never renumber or reuse a decision ID. Never delete a decision: supersede it.
- When your code goes against a current decision, say so in your PR. Do not edit this page to match the code.
-->

# Authentication: Current Decisions

> Written by `eng-adr`. To change a decision, write a spec or run `eng-adr new`. Do not edit this page by hand.

**Purpose:** User login through the control plane, the login callback, and the hand-off of the session token to the CLI and other clients. **Owner:** migmartri. **Does not own:** the CAS download token header (Artifact CAS), and the organization that a command uses (Attestation Process). **Specs:** [issue-3626-login-code-exchange](../specs/issue-3626-login-code-exchange.md). **Last refresh:** 2026-11-01, commit 4e150ca7.

## Language

- **Handoff code**: the single-use code that the login callback gets in place of the session token. The client exchanges it for the token. _Avoid_: "auth code" or "code", because a callback can also be an OAuth client that gets its own `code`.
- **Legacy callback**: the callback that carries the session token in `?t=`. _Avoid_: "old login", because headless login is a different flow.

## Current decisions

### AUT-001: The callback gets a single-use handoff code, not the token
When the client sends a challenge, the login callback gets a single-use code. The client exchanges the code for the session token.
- Why: it takes the token out of the URL.
- Rejected: keep `?t=` and add checks (the token is still in the URL). The token in the URL fragment (still in history, and fixation stays). A full OAuth server with refresh tokens (a much larger change).
- Deciders: migmartri, jiparis
- Source: [issue-3626-login-code-exchange D-001](../specs/issue-3626-login-code-exchange.md)
- Code: verified: `app/controlplane/internal/service/auth.go`, `app/controlplane/pkg/biz/loginhandoff.go`, `app/cli/cmd/auth_login.go`.

### AUT-002: The code is bound to an S256 challenge and the callback URL
The client sends a PKCE-style S256 challenge. The exchange checks the verifier and the callback URL.
- Why: it works for the CLI and for a browser app, and needs no cookie at exchange time.
- Rejected: bind to the callback URL only (a web app has a fixed callback, so fixation stays). Require the OIDC state cookie at exchange (the CLI and a cross-origin web app do not have it).
- Deciders: migmartri, jiparis
- Source: [issue-3626-login-code-exchange D-002](../specs/issue-3626-login-code-exchange.md)
- Code: verified: `app/controlplane/pkg/biz/loginhandoff.go` (`Issue`, `Exchange`), `app/cli/cmd/auth_login.go`.

### AUT-003: The code store is a database table with no token at rest
The table holds the code hash, the user, the expiry, and the challenge. One conditional update consumes the code. The control plane makes the token at exchange time.
- Why: it works with many replicas and enforces single use.
- Rejected: store the token with the code (a token at rest in the database). Memory (fails with more than one replica). A signed code with no store (cannot enforce single use).
- Deciders: migmartri, jiparis
- Source: [issue-3626-login-code-exchange D-004](../specs/issue-3626-login-code-exchange.md)
- Code: verified: `app/controlplane/pkg/data/loginhandoff.go`, `app/controlplane/internal/service/auth.go`.

### AUT-004: Opt-in per request for client callbacks, with flags on the info endpoint
For a client callback, the server issues a code only when the request has a challenge. A download callback (`/download/<digest>`) must send a challenge. The info endpoint reports whether the exchange is supported and whether the legacy callback is allowed. Legacy is allowed unless the operator sets `allow_legacy_login_callback: false`.
- Why: old CLIs need no change. A new CLI knows what to expect and can detect a downgrade.
- Rejected: opt-in with no flag (the CLI cannot tell a downgrade from an old server). A version check (fails with forks and development builds).
- Deciders: migmartri, jiparis
- Source: [issue-3626-login-code-exchange D-005](../specs/issue-3626-login-code-exchange.md)
- Code: verified: `app/controlplane/api/controlplane/v1/status.proto`, `app/controlplane/internal/service/status.go`, `app/cli/cmd/auth_login.go`.

### AUT-005: After it sends a challenge, the CLI accepts `?t=` only while the server allows the legacy callback
When the CLI sent a challenge and gets `?t=`, it accepts the token with a warning while the server reports legacy allowed. It refuses the token when legacy is denied.
- Why: a login page that does not forward the new parameters yet must not break login. The loopback listener has a random port and a short life, so the fixation risk is low.
- Rejected: refuse `?t=` from the first CLI release (it breaks login behind such a login page).
- Deciders: migmartri, jiparis
- Source: [issue-3626-login-code-exchange D-007](../specs/issue-3626-login-code-exchange.md)
- Code: verified: `app/cli/cmd/auth_login.go`.

### AUT-006: One error to the client for each exchange failure
Each exchange failure gives the client the same error. The specific reason goes to the server log only. A request that fails field validation (an empty or too long field) gets the validation error before the exchange runs.
- Why: a uniform client error gives no hint to a person who guesses codes. The server log reason helps operators debug failed logins.
- Rejected: a uniform error with no server log reason (hard to debug).
- Deciders: migmartri, jiparis
- Source: [issue-3626-login-code-exchange D-012](../specs/issue-3626-login-code-exchange.md)
- Code: verified: `app/controlplane/internal/service/auth.go`.

### AUT-007: The exchange endpoint is gRPC only
The exchange is an unauthenticated RPC on the auth service, with no JSON/REST mapping. Clients call it over gRPC or gRPC-Web.
- Why: the CLI uses gRPC and the web UI uses gRPC-Web, so no client needs a plain HTTP route. With no HTTP route, the public surface of the unauthenticated endpoint is smaller, and the web UI makes no cross-origin preflight.
- Rejected: keep an HTTP mapping for a future non-gRPC client (no such client exists).
- Deciders: migmartri, matiasinsaurralde
- Source: [issue-3626-login-code-exchange D-013](../specs/issue-3626-login-code-exchange.md)
- Code: verified: `app/controlplane/api/controlplane/v1/auth.proto` has no `google.api.http` option. `app/controlplane/internal/server/grpc.go`.

Not shown: 6 smaller decisions of [issue-3626-login-code-exchange](../specs/issue-3626-login-code-exchange.md).

## Non-Confirmed

None.

## Superseded

None.

## Open conflicts

- AUT-004 and [issue-3626-login-code-exchange D-008](../specs/issue-3626-login-code-exchange.md) disagree about the download login. D-008 keeps it on the legacy callback. The code requires a challenge for a `/download/` callback (`app/controlplane/internal/service/auth.go`). Proposed: write a spec change that records the current download flow.
