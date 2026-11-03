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

# Current Decisions

> Written by `eng-adr`. To change a decision, write a spec or run `eng-adr new`. Do not edit this page by hand.

Last refresh: 2026-11-01, commit 4e150ca7.

## Areas

### Artifact CAS
- **Page:** [artifact-cas.md](artifact-cas.md)
- **Prefix:** CAS
- **Owner:** migmartri
- **Purpose:** Content-addressable storage of evidence blobs, with its upload and download paths and its caches.
- **Does not own:** the listeners, ports, TLS, and ingress of the CAS (Service Operations), and the download tokens that the control plane makes (Authentication).
- **Specs:** [issue-3543-cas-existence-cache](../specs/issue-3543-cas-existence-cache.md), [issue-3549-cas-download-browser-cache](../specs/issue-3549-cas-download-browser-cache.md)
- **Code:** `app/artifact-cas/`, `pkg/blobmanager/`, `pkg/cache/casexistence/`, `pkg/middlewares/http/`

### Attestation Process
- **Page:** [attestation-process.md](attestation-process.md)
- **Prefix:** ATT
- **Owner:** migmartri
- **Purpose:** The attestation lifecycle of the CLI (`init`, `add`, `status`, `push`, `reset`, `verify`), and how it selects its organization and project.
- **Does not own:** AI coding session evidence (Chainloop Trace), and login and tokens (Authentication).
- **Specs:** [issue-3063-attestation-repo-config](../specs/issue-3063-attestation-repo-config.md)
- **Code:** `app/cli/cmd/attestation*.go`, `app/cli/pkg/action/attestation_*.go`, `app/cli/internal/repositoryconfig/`, `pkg/attestation/`

### Authentication
- **Page:** [authentication.md](authentication.md)
- **Prefix:** AUT
- **Owner:** migmartri
- **Purpose:** User login through the control plane, the login callback, and the hand-off of the session token to the CLI and other clients.
- **Does not own:** the CAS download token header (Artifact CAS), and the organization that a command uses (Attestation Process).
- **Specs:** [issue-3626-login-code-exchange](../specs/issue-3626-login-code-exchange.md)
- **Code:** `app/controlplane/internal/service/auth.go`, `app/controlplane/pkg/biz/loginhandoff.go`, `app/controlplane/pkg/data/loginhandoff.go`, `app/cli/cmd/auth_login.go`

### Service Operations
- **Page:** [service-operations.md](service-operations.md)
- **Prefix:** OPS
- **Owner:** migmartri
- **Purpose:** The network endpoints of the control plane and the CAS: listeners, protocols, TLS, internal ports, and the Helm chart ingress.
- **Does not own:** what the endpoints serve (Artifact CAS, Authentication).
- **Specs:** [issue-3628-single-endpoint](../specs/issue-3628-single-endpoint.md), [issue-3628-controlplane-http-dispatch](../specs/issue-3628-controlplane-http-dispatch.md)
- **Code:** `app/controlplane/internal/server/`, `app/artifact-cas/internal/server/`, `deployment/chainloop/`

### Chainloop Trace
- **Page:** [chainloop-trace.md](chainloop-trace.md)
- **Prefix:** TRC
- **Owner:** jiparis
- **Purpose:** Recording AI coding sessions as evidence: providers, spec and skill capture, transcript pointers, line attribution, local mode, and the e2e test matrix.
- **Does not own:** the attestation lifecycle and the organization order of `.chainloop.yml` (Attestation Process), and blob storage (Artifact CAS).
- **Specs:** [issue-3495-session-spec-capture](../specs/issue-3495-session-spec-capture.md), [issue-3515-spec-capture-during-session](../specs/issue-3515-spec-capture-during-session.md), [issue-3520-pi-trace-provider](../specs/issue-3520-pi-trace-provider.md), [issue-3531-spec-source-role](../specs/issue-3531-spec-source-role.md), [issue-3540-trace-skills-used](../specs/issue-3540-trace-skills-used.md), [issue-3556-session-image-placeholders](../specs/issue-3556-session-image-placeholders.md), [issue-3561-up-to-date-local-spec-evidence](../specs/issue-3561-up-to-date-local-spec-evidence.md), [issue-3569-pasted-image-pointers](../specs/issue-3569-pasted-image-pointers.md), [issue-3582-trace-evidence-export](../specs/issue-3582-trace-evidence-export.md), [issue-3635-trace-e2e-matrix](../specs/issue-3635-trace-e2e-matrix.md), [issue-3639-line-ranges](../specs/issue-3639-line-ranges.md), [issue-3639-trace-line-attribution](../specs/issue-3639-trace-line-attribution.md), [issue-3657-codex-trace-provider](../specs/issue-3657-codex-trace-provider.md)
- **Code:** `app/cli/internal/trace/`, `app/cli/cmd/trace*.go`, `app/cli/pkg/action/trace_*.go`, `pkg/attestation/crafter/materials/aicodingsession/`

## Relationships

- **Attestation Process → Chainloop Trace:** both read `.chainloop.yml` through one reader (ATT-003, attestation-process.md). A change to the file order or keys changes both areas.
- **Chainloop Trace → Artifact CAS:** On a push, Trace uploads each spec source, skill package, and pasted image as a material. Local mode uses neither. A transcript pointer replaces a copy only when its material is stored (TRC-016, chainloop-trace.md).
- **Service Operations → Artifact CAS, Authentication:** the shared endpoints carry the CAS ByteStream and download routes and the control plane login endpoints. A change to their dispatch or TLS affects all of them. It does not affect ByteStream traffic on the legacy gRPC listener, which the chart uses by default for the control plane.

## Repository-wide decisions

None.

## Non-Confirmed

None.

## Superseded

None.

## Open conflicts

None.
