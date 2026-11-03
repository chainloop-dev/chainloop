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

# Attestation Process: Current Decisions

> Written by `eng-adr`. To change a decision, write a spec or run `eng-adr new`. Do not edit this page by hand.

**Purpose:** The attestation lifecycle of the CLI (`init`, `add`, `status`, `push`, `reset`, `verify`), and how it selects its organization and project. **Owner:** migmartri. **Does not own:** AI coding session evidence (Chainloop Trace), and login and tokens (Authentication). **Specs:** [issue-3063-attestation-repo-config](../specs/issue-3063-attestation-repo-config.md). **Last refresh:** 2026-11-01, commit 4e150ca7.

## Language

- **Repository config**: the nearest `.chainloop.yml` (or `.chainloop.yaml`), from the current directory up to the repository root. It pins `organization` and `projectName`. _Avoid_: "workflow file", because the attestation commands do not read a workflow from it.
- **Saved default**: the organization that the CLI keeps in its own configuration for the user. _Avoid_: "default org" without a qualifier, because the repository config can also give an organization.

## Current decisions

### ATT-001: Organization order is flag, environment variable, repository config, saved default
The `--org` flag and its environment variable win. For `init`, `add`, `push`, `status`, and `reset`, the repository config applies only when neither is set, and it comes before the saved default.
- Why: the file pins the organization for the repository.
- Rejected: the saved default before the file (most users have a saved default, so the file would almost never apply).
- Deciders: migmartri, javirln
- Source: [issue-3063-attestation-repo-config D-002](../specs/issue-3063-attestation-repo-config.md)
- Code: verified: `app/cli/cmd/attestation.go`, `app/cli/cmd/root.go`. For a command other than init, when the repository config gives no organization, the organization of the local attestation state comes before the saved default (`attestation.go`).

### ATT-002: init, add, push, status, and reset read the organization from the file
The attestation commands that contact the control plane read the organization from the repository config. They are `init`, `add`, `push`, `status`, and `reset`.
- Why: with remote state, a command must connect to the correct organization before it can load the state.
- Deciders: migmartri, javirln
- Source: [issue-3063-attestation-repo-config D-004](../specs/issue-3063-attestation-repo-config.md)
- Code: verified: the five commands that R-003 lists read the file (`app/cli/cmd/attestation.go`). `attestation verify` does not read it. It calls only the trusted root service, which needs no organization (`app/cli/pkg/action/attestation_verify.go`).

### ATT-003: One reader of `.chainloop.yml` for attestation and trace
Attestation commands and Trace read the repository config through one package.
- Why: two readers already disagreed on the file order.
- Rejected: a second change to the attestation reader only.
- Deciders: migmartri, javirln
- Source: [issue-3063-attestation-repo-config D-007](../specs/issue-3063-attestation-repo-config.md)
- Code: verified for reads: `app/cli/internal/repositoryconfig/config.go`, used by `app/cli/cmd/attestation.go` and `app/cli/internal/trace/config/config.go`. Writes still use their own lookup.

### ATT-004: The repository config organization acts as `--org`, with a check for API tokens
The organization from the file has the same effect as `--org` for the run. It is not saved as the default. When an API token belongs to a different organization, the command stops with an error that names both.
- Why: one rule for users: the file is a `--org` that the repository keeps.
- Rejected: an exact copy of `--org` (with an API token, it ignores a different organization with no message). A warning only (CI logs often hide it).
- Deciders: migmartri, javirln
- Source: [issue-3063-attestation-repo-config D-010](../specs/issue-3063-attestation-repo-config.md)
- Code: verified: `app/cli/cmd/attestation.go`, `app/cli/internal/token/token.go`, `app/cli/cmd/root.go`.

Not shown: 6 smaller decisions of [issue-3063-attestation-repo-config](../specs/issue-3063-attestation-repo-config.md).

## Non-Confirmed

None.

## Superseded

None.

## Open conflicts

None.
