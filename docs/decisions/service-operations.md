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

# Service Operations: Current Decisions

> Written by `eng-adr`. To change a decision, write a spec or run `eng-adr new`. Do not edit this page by hand.

**Purpose:** The network endpoints of the control plane and the CAS: listeners, protocols, TLS, internal ports, and the Helm chart ingress. **Owner:** migmartri. **Does not own:** what the endpoints serve (Artifact CAS, Authentication). **Specs:** [issue-3628-single-endpoint](../specs/issue-3628-single-endpoint.md), [issue-3628-controlplane-http-dispatch](../specs/issue-3628-controlplane-http-dispatch.md). **Last refresh:** 2026-11-01, commit 4e150ca7.

## Language

- **Shared endpoint**: the HTTP port of a service (`server.http.addr`). It serves native gRPC, plain HTTP, and, on the control plane, gRPC-Web. _Avoid_: "single port", the older word for it, because the service still has other ports. Also avoid "gRPC port" for the shared endpoint.
- **Legacy gRPC listener**: the separate native gRPC port (`server.grpc.addr`), with its own TLS settings. It is on by default, and `server.legacy_grpc_listener: false` turns it off. It is deprecated. _Avoid_: "the gRPC endpoint", because the shared endpoint also serves gRPC.

## Current decisions

### OPS-002: Metrics and profiler ports stay separate
The process metrics endpoint (`/metrics`) and the opt-in profiler of the control plane (`enable_profiler`) keep their own ports. They are not on the shared endpoint. The authenticated metrics route of each organization (`/prom/{org_name}/metrics`) is on the control plane shared endpoint.
- Why: they are internal and must not be on the public hostname.
- Deciders: migmartri, matiasinsaurralde
- Source: [issue-3628-single-endpoint D-006](../specs/issue-3628-single-endpoint.md). For the control plane: the Constraints of [issue-3628-controlplane-http-dispatch](../specs/issue-3628-controlplane-http-dispatch.md).
- Code: verified: `app/artifact-cas/internal/server/httpmetrics.go` (separate metrics server), `app/controlplane/internal/server/profiler.go` (pprof on its own port).

### OPS-003: TLS covers all protocols on the shared endpoint
When the operator configures TLS, all protocols on the shared endpoint use it. The ingress must use the encrypted HTTP/2 backend setting of its controller (nginx `GRPCS`, Traefik `https`). A shared endpoint with only a certificate or only a key fails at startup. The legacy gRPC listener keeps its own TLS settings, and a partial configuration keeps it plaintext. The chart sets both from one secret.
- Why: one certificate and one setting for each service.
- Rejected: plain text and TLS on the same port (a second classification step, and a weaker TLS setting).
- Deciders: migmartri, matiasinsaurralde
- Source: [issue-3628-single-endpoint D-008](../specs/issue-3628-single-endpoint.md). For the control plane: R-001 and R-003 of [issue-3628-controlplane-http-dispatch](../specs/issue-3628-controlplane-http-dispatch.md).
- Code: verified: `app/artifact-cas/internal/server/http.go`. The legacy listener keeps `server.grpc.tls_config` (`deployment/chainloop/templates/cas/configmap.yaml`).

### OPS-004: The current HTTP port is the shared endpoint
The HTTP port of each service becomes the shared endpoint. The gRPC port is the legacy listener.
- Why: the human-facing hostname, with the status, download, and login endpoints, is already on the HTTP port. Clients that use ingress hostnames do not break. The legacy port serves during the deprecation period. After that, only the backend port of the gRPC ingress changes. Direct clients, for example the control plane connection to the CAS, must move to the shared endpoint before the legacy listener stops.
- Rejected: the gRPC port (TLS settings stay in place, but the human-facing hostname stays on a second port). A new port (every client changes).
- Deciders: migmartri, matiasinsaurralde
- Source: [issue-3628-single-endpoint D-012](../specs/issue-3628-single-endpoint.md). For the control plane: R-001 of [issue-3628-controlplane-http-dispatch](../specs/issue-3628-controlplane-http-dispatch.md).
- Code: verified: `app/artifact-cas/internal/server/http.go`, `app/artifact-cas/internal/server/grpc.go` (legacy listener on by default).

### OPS-005: The server process does the multiplexing, with no external proxy or sidecar
Each service serves all protocols on its shared endpoint in its own process. It runs on a VM, in Docker Compose, and in Kubernetes with no separate proxy process or sidecar. The CAS uses an in-process reverse proxy to a private loopback gRPC server ([issue-3628-single-endpoint D-020](../specs/issue-3628-single-endpoint.md)).
- Why: no extra process, configuration, or lifecycle in any deployment.
- Rejected: a chart-level proxy or Gateway API routes (extra infrastructure, and two ports stay). A mandatory sidecar or a separate proxy process (a process and its lifecycle in every deployment).
- Deciders: migmartri, jiparis
- Source: [issue-3628-single-endpoint D-015](../specs/issue-3628-single-endpoint.md). For the control plane: [issue-3628-controlplane-http-dispatch D-001](../specs/issue-3628-controlplane-http-dispatch.md).
- Code: verified: `app/controlplane/internal/server/http.go`, `app/artifact-cas/internal/server/http.go`.
- Partly replaced: the spec decision also kept native gRPC and plain HTTP on separate upstream connections. [issue-3628-controlplane-http-dispatch D-001](../specs/issue-3628-controlplane-http-dispatch.md) replaced that part for the control plane. For the CAS, R-010 and D-020 of [issue-3628-single-endpoint](../specs/issue-3628-single-endpoint.md) replaced it.

Not shown: 15 smaller or replaced decisions of [issue-3628-single-endpoint](../specs/issue-3628-single-endpoint.md), 1 of [issue-3628-controlplane-http-dispatch](../specs/issue-3628-controlplane-http-dispatch.md).

## Non-Confirmed

### OPS-001: The CAS keeps its native gRPC transport and separate endpoints
- Why: a paused-reader probe showed that the HTTP adapter can read ahead faster than the application receives.
- Deciders: migmartri, jiparis
- Source: [issue-3628-controlplane-http-dispatch D-003](../specs/issue-3628-controlplane-http-dispatch.md)
- Code: not confirmed. ByteStream still uses the native gRPC transport, through a private loopback gRPC server (`app/artifact-cas/internal/server/grpc.go`). But the CAS also has a shared endpoint (`app/artifact-cas/internal/server/http.go`). See Open conflicts. The parent ticket is open.

## Superseded

None.

## Open conflicts

- OPS-001 ([issue-3628-controlplane-http-dispatch D-003](../specs/issue-3628-controlplane-http-dispatch.md)) and [issue-3628-single-endpoint D-020](../specs/issue-3628-single-endpoint.md) disagree about whether the CAS gets a shared endpoint. Both keep the native gRPC transport. Neither has a "replaces" note. The code follows D-020. Proposed: write a spec change that records D-020 as the replacement of dispatch D-003.
