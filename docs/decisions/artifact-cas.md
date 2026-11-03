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

# Artifact CAS: Current Decisions

> Written by `eng-adr`. To change a decision, write a spec or run `eng-adr new`. Do not edit this page by hand.

**Purpose:** Content-addressable storage of evidence blobs, with its upload and download paths and its caches. **Owner:** migmartri. **Does not own:** the listeners, ports, TLS, and ingress of the CAS (Service Operations), and the download tokens that the control plane makes (Authentication). **Specs:** [issue-3543-cas-existence-cache](../specs/issue-3543-cas-existence-cache.md), [issue-3549-cas-download-browser-cache](../specs/issue-3549-cas-download-browser-cache.md). **Last refresh:** 2026-11-01, commit 4e150ca7.

## Language

- **Existence cache**: the server-side cache of "this blob is in the backend" results, keyed per organization and backend. _Avoid_: "upload cache", because the cache holds no content.
- **Client cache**: the in-process cache of backend clients and their credentials. _Avoid_: "credentials cache", because the credentials are part of the client and never stored alone.

## Current decisions

### CAS-001: Existence cache in NATS KV when connected, in memory otherwise
The existence cache uses the shared cache library. It uses NATS KV when the CAS has a NATS connection. It uses an in-memory LRU when it has no connection, or when the NATS bucket cannot be set up at startup. A NATS failure at runtime is a cache miss, not a change to memory.
- Why: replicas share hits when NATS is there, and the CAS gets no new dependency when it is not.
- Rejected: memory only (the hit rate drops as replicas grow). NATS required (a hard dependency for a speed gain).
- Deciders: migmartri, javirln
- Source: [issue-3543-cas-existence-cache D-001](../specs/issue-3543-cas-existence-cache.md)
- Code: verified: `pkg/cache/casexistence/casexistence.go`, `app/artifact-cas/cmd/wire.go`.

### CAS-002: The existence cache lives in the CAS, server side
The CAS ByteStream write checks the cache and answers "already exists" with no backend call. The CLI has no cache.
- Why: it works for all CLI versions and all clients, with no API change.
- Rejected: a cache in the CLI (each command is a new process, so nothing is reused).
- Deciders: migmartri, javirln
- Source: [issue-3543-cas-existence-cache D-003](../specs/issue-3543-cas-existence-cache.md)
- Code: verified: `app/artifact-cas/internal/service/bytestream.go`, `app/artifact-cas/internal/service/cache.go`.

### CAS-003: The cache key starts with the organization
The key is the organization, then the backend type, then a hash of the secret reference, then the digest. The cache has its own bucket.
- Why: the organization prefix keeps the entries of one tenant away from all other tenants, also on shared storage. The secret reference separates backends in one organization.
- Rejected: the digest alone (the upload of one tenant answers for another tenant, and shows that the other tenant has the file). The backend and the digest without the organization (a shared backend leaks the same information).
- Deciders: migmartri, javirln
- Source: [issue-3543-cas-existence-cache D-004](../specs/issue-3543-cas-existence-cache.md)
- Code: verified: `pkg/cache/casexistence/casexistence.go`. `app/artifact-cas/internal/service/cache.go` skips the cache when the organization is empty.

### CAS-004: Credentials never go to a shared store
The client cache, which holds backend credentials, stays in process memory. It never uses NATS.
- Why: credentials must not leave the process.
- Deciders: migmartri, javirln
- Source: [issue-3543-cas-existence-cache D-006](../specs/issue-3543-cas-existence-cache.md)
- Code: verified: `app/artifact-cas/internal/service/cache.go` (in-process expirable LRU).

### CAS-005: Downloads use the browser HTTP cache
The CAS sets cache headers on downloads and answers 304 when the browser has the content. The control plane keeps no copy of the bytes.
- Why: the browser downloads straight from the CAS, so a cache there skips the backend copy and the transfer.
- Rejected: a server cache in the control plane (the control plane never sees the bytes, and its message size limit is too small).
- Deciders: jiparis, migmartri
- Source: [issue-3549-cas-download-browser-cache D-001](../specs/issue-3549-cas-download-browser-cache.md)
- Code: verified: `app/artifact-cas/internal/service/download.go`.

### CAS-006: The download token goes in the Authorization header
The CAS accepts the download token in an `Authorization: Bearer` header. When a client moves the token from the `?t=` query to this header, the download URL stays the same between calls.
- Why: the browser cache uses the URL as its key, and the token changes on each call.
- Rejected: one token for each user and digest (the URL stays the same only for the life of the token).
- Deciders: jiparis, migmartri
- Source: [issue-3549-cas-download-browser-cache D-003](../specs/issue-3549-cas-download-browser-cache.md)
- Code: verified on the CAS side: `pkg/middlewares/http/jwt.go`. When `cors.allow_origins` is set, the preflight in `app/artifact-cas/internal/server/cors.go` allows `Authorization`. The control plane `GetDownloadURL` still puts the token in `?t=` (`app/controlplane/internal/service/casredirect.go`). The client that moves the token to the header is not in this repository.

### CAS-007: The header token wins, with no fallback to the query
When a request has a token in the header and in the query, the CAS uses only the header. A bad header fails with 401.
- Why: a client that sends a header sets its intent.
- Rejected: reject a request with both (the viewer must then strip the query). The query wins (the header has no effect for a URL that still holds a token).
- Deciders: jiparis, migmartri
- Source: [issue-3549-cas-download-browser-cache D-004](../specs/issue-3549-cas-download-browser-cache.md)
- Code: verified: `pkg/middlewares/http/jwt.go`.

### CAS-008: The CAS checks that the object exists before a 304
Before it answers 304, the CAS makes a metadata call to the backend. A deleted object gives 404.
- Why: a deleted object stops showing at once, and the metadata call is cheap.
- Rejected: a 304 from the digest alone (a browser can show a deleted object until it drops its copy).
- Deciders: jiparis, migmartri
- Source: [issue-3549-cas-download-browser-cache D-005](../specs/issue-3549-cas-download-browser-cache.md)
- Code: verified: `app/artifact-cas/internal/service/download.go`.

### CAS-009: The existence cache lifetime is fixed, with no off switch
Replaces [issue-3543-cas-existence-cache D-007](../specs/issue-3543-cas-existence-cache.md) and the settings of R-004. An existence entry expires after 24 hours. No setting changes the lifetime or turns the cache off.
- Why: blobs cannot be deleted through Chainloop, so a fixed 24-hour bound covers the only risk. A setting adds configuration that no operator needs.
- Rejected: a configurable lifetime and an operator switch that turns the cache off (spec D-007 and R-004).
- Deciders: migmartri
- Source: recorded directly, 2026-11-03
- Code: verified: `pkg/cache/casexistence/casexistence.go` (`ttl` constant), `app/artifact-cas/cmd/wire.go` (the cache is always on).

Not shown: 5 smaller decisions of [issue-3543-cas-existence-cache](../specs/issue-3543-cas-existence-cache.md), 2 of [issue-3549-cas-download-browser-cache](../specs/issue-3549-cas-download-browser-cache.md).

## Non-Confirmed

None.

## Superseded

None.

## Open conflicts

None.
