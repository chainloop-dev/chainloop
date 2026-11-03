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

# Chainloop Trace: Current Decisions

> Written by `eng-adr`. To change a decision, write a spec or run `eng-adr new`. Do not edit this page by hand.

**Purpose:** Recording AI coding sessions as evidence: providers, spec and skill capture, transcript pointers, line attribution, local mode, and the e2e test matrix. **Owner:** jiparis. **Does not own:** the attestation lifecycle and the organization order of `.chainloop.yml` (Attestation Process), and blob storage (Artifact CAS). **Specs:** [issue-3495-session-spec-capture](../specs/issue-3495-session-spec-capture.md), [issue-3515-spec-capture-during-session](../specs/issue-3515-spec-capture-during-session.md), [issue-3520-pi-trace-provider](../specs/issue-3520-pi-trace-provider.md), [issue-3531-spec-source-role](../specs/issue-3531-spec-source-role.md), [issue-3540-trace-skills-used](../specs/issue-3540-trace-skills-used.md), [issue-3556-session-image-placeholders](../specs/issue-3556-session-image-placeholders.md), [issue-3561-up-to-date-local-spec-evidence](../specs/issue-3561-up-to-date-local-spec-evidence.md), [issue-3569-pasted-image-pointers](../specs/issue-3569-pasted-image-pointers.md), [issue-3582-trace-evidence-export](../specs/issue-3582-trace-evidence-export.md), [issue-3635-trace-e2e-matrix](../specs/issue-3635-trace-e2e-matrix.md), [issue-3639-line-ranges](../specs/issue-3639-line-ranges.md), [issue-3639-trace-line-attribution](../specs/issue-3639-trace-line-attribution.md), [issue-3657-codex-trace-provider](../specs/issue-3657-codex-trace-provider.md). **Last refresh:** 2026-11-01, commit 4e150ca7.

## Language

- **Provider**: the Trace integration for one AI coding agent (Claude Code, Cursor, OpenCode, Pi, Codex). _Avoid_: "harness" in new code and in evidence. The e2e suite and the `trace init` help text and output still use "harness" for the agent.
- **Spec source**: an input that the session was built from (a ticket, a document, a plan, an image, a skill). It is recorded as a spec entry and its own material. _Avoid_: "attachment", because a source is evidence with a digest, not a file on a message.
- **Capture**: a file that the agent writes into the spec folder for one source. The CLI also captures pasted images (TRC-018). _Avoid_: "export", which was the old name of local mode.
- **Pointer**: the `chainloop.replaced` object that takes the place of a copy of a spec source in the transcript. _Avoid_: "placeholder", which is the fixed line in a local-file capture.
- **Local mode**: the storage mode (`traceStorage: LOCAL`, or `trace run --local` for one run) that stores sessions on disk with no control plane. _Avoid_: "offline", because the agent and the model are still online. Also avoid "export mode".
- **Net contribution**: the lines that a session added and that no later commit replaced. _Avoid_: "authored lines", which counts lines that later commits replaced.

## Current decisions

### TRC-001: The agent decides what the spec is
An instruction at session start tells the agent to resolve the spec of the session and write it into the spec folder.
- Why: only the agent knows which input sets the task, and it reaches sources that the transcript never holds.
- Rejected: extraction from the transcript by the CLI. It needs rules to pick the spec, misses sources that the agent never opened, and needs one extractor for each agent. Extraction on the server after ingestion (the spec is not in the attestation). The spec keeps both as a possible fallback if the capture rate is poor. Neither is built.
- Deciders: jiparis, migmartri
- Source: [issue-3495-session-spec-capture D-001](../specs/issue-3495-session-spec-capture.md)
- Code: verified: `app/cli/pkg/action/trace_spec.go`, `app/cli/pkg/action/trace_agent_hook.go`.
- Partly replaced by TRC-009 and TRC-018: the CLI makes the skill entries and the pasted image entries.

### TRC-002: A capture is Markdown with a short header
A text capture is a Markdown body under a frontmatter header (kind, uri, role, title, description). The CLI copies a binary file as it is, with no header. With no header or an unterminated header, the whole file is content. The CLI skips a header line with an unknown key. An unknown kind becomes `text`. A file with an empty body or only the placeholder gives no capture.
- Why: a model writes long Markdown reliably. Long text escaped inside JSON fails completely when one character is wrong. A bad header still keeps the content.
- Rejected: JSON files.
- Deciders: jiparis, migmartri
- Source: [issue-3495-session-spec-capture D-003](../specs/issue-3495-session-spec-capture.md)
- Code: verified: `app/cli/internal/trace/spec/parse.go`.

### TRC-003: The agent does not capture a pasted image
The instructions tell the agent not to capture an image pasted into the conversation, also when it has a file path for it.
- Why: the agent cannot copy the bytes of a pasted image, and a description that the agent writes is not the image.
- Rejected: a description of kind `image` (the earlier rule of issue-3495 D-010).
- Deciders: migmartri, jiparis
- Source: [issue-3515-spec-capture-during-session D-009](../specs/issue-3515-spec-capture-during-session.md)
- Code: verified: `app/cli/pkg/action/trace_spec.go`, `app/cli/internal/trace/README.md`. See Open conflicts.

### TRC-004: Pi integration is a project-local TypeScript extension
`trace init` installs a marked extension at `.pi/extensions/chainloop-trace.ts`, which sends Pi lifecycle and tool events to the Trace hooks.
- Why: Pi documents lifecycle and tool events for extensions.
- Rejected: a Pi fork, or a file watcher.
- Deciders: waveywaves, migmartri, jiparis
- Source: [issue-3520-pi-trace-provider D-001](../specs/issue-3520-pi-trace-provider.md)
- Code: verified: `app/cli/internal/trace/pi/hooks.go`.

### TRC-005: The Pi provider parses a copy of the Pi JSONL v3 transcript
The provider keeps a private copy of the Pi session file and parses only that copy.
- Why: the Pi transcript is the authoritative record of provider, model, usage, tool, and conversation data.
- Rejected: a duplicate transcript that the extension writes.
- Deciders: waveywaves, migmartri, jiparis
- Source: [issue-3520-pi-trace-provider D-006](../specs/issue-3520-pi-trace-provider.md)
- Code: verified: `app/cli/internal/trace/pi/provider.go`, `app/cli/internal/trace/pi/parse.go`.

### TRC-006: File snapshot keys include the tool call ID
The hook input field `tool_use_id` is part of the file snapshot key when the provider sends it. Without it, the key is the session and the path.
- Why: it prevents an overwrite between concurrent tool calls, without a second name for one value and without the subagent identity.
- Deciders: waveywaves, migmartri, jiparis
- Source: [issue-3520-pi-trace-provider D-012](../specs/issue-3520-pi-trace-provider.md)
- Code: verified: `app/cli/internal/trace/state/snapshot.go`. Pi, Claude Code, and Codex send it. The OpenCode key has no tool call ID (`app/cli/internal/trace/opencode/provider.go`).

### TRC-007: A binary source gets its role from a companion file
For a binary file, the agent writes `<name>.meta.yaml` next to it, with the role, the title, and the description.
- Why: the agent decides the role of every source, also of an image.
- Rejected: the CLI sets `reference` for each image (a guess, and a mockup can be the spec). No role for binary files.
- Deciders: jiparis, migmartri
- Source: [issue-3531-spec-source-role D-003](../specs/issue-3531-spec-source-role.md)
- Code: verified: `app/cli/internal/trace/spec/spec.go`, `app/cli/pkg/action/trace_spec_materials.go`.

### TRC-008: A used skill is a spec entry of kind `skill`
Each skill that the session used is an entry of kind `skill` in the spec list of the session, up to 25 skills. A skill with no folder gets no entry. This applies to the providers with skill tracking.
- Why: a skill is a source of the session, like a ticket or a plan. It gets the same storage, redaction, and annotations.
- Rejected: a separate `skills_used` list in the session material.
- Deciders: jiparis, migmartri
- Source: [issue-3540-trace-skills-used D-001](../specs/issue-3540-trace-skills-used.md)
- Code: verified: `pkg/attestation/crafter/materials/aicodingsession/aicodingsession.go`, `app/cli/pkg/action/trace_skills.go`.

### TRC-009: The CLI makes the skill entries from the transcript
The CLI finds skill use in the transcript and the hook events. The agent does not capture skills.
- Why: skill use is a fact in the transcript, and no rule must pick it. The reason of TRC-001 (the CLI cannot tell which input sets the task) does not apply to skills.
- Deciders: jiparis, migmartri
- Source: [issue-3540-trace-skills-used D-002](../specs/issue-3540-trace-skills-used.md)
- Code: verified for Claude Code, Codex, and OpenCode: `app/cli/internal/trace/claude/skills.go`, `app/cli/pkg/action/trace_skills.go`. Cursor and Pi have no skill tracking.

### TRC-010: The whole skill folder is recorded
The CLI copies the folder of a used skill, not only the text that the agent loaded. It leaves out links out of the folder and version-control folders.
- Why: a skill can run its own scripts, and a reviewer needs that code. The attestation must not depend on the `ai-agent-config` material.
- Rejected: the `SKILL.md` text from the transcript (no scripts, and Claude Code deletes the frontmatter).
- Deciders: jiparis, migmartri
- Source: [issue-3540-trace-skills-used D-004](../specs/issue-3540-trace-skills-used.md)
- Code: verified: `app/cli/pkg/action/trace_skills.go`, `app/cli/internal/trace/skill/skill.go`.

### TRC-011: One reproducible archive for each skill
Each skill gives one archive with fixed file times, stored as one package material. The CLI does not upload a package over 5 MB, and shows a warning. The entry and its counts stay.
- Why: one package digest for each skill, and the same skill version gives the same digest in all sessions.
- Rejected: one material for each file (it fills the attestation for a skill with many files).
- Deciders: jiparis, migmartri
- Source: [issue-3540-trace-skills-used D-006](../specs/issue-3540-trace-skills-used.md)
- Code: verified: `app/cli/internal/trace/skill/skill.go`, `app/cli/pkg/action/trace_skills.go`.

### TRC-012: Copies of a source become digest pointers in the transcript
The content stays in the spec material. The CLI replaces each exact copy that it finds in the transcript, in place, with a pointer with the digest. A copy that does not match stays inline.
- Why: the transcript gets small and keeps the position of each copy. The content stays in the evidence.
- Rejected: drop the copies (a reader loses the position). Delete only the second copy of read files (the transcript is still large, and the redaction still scans the bytes).
- Deciders: migmartri, javirln
- Source: [issue-3556-session-image-placeholders D-001](../specs/issue-3556-session-image-placeholders.md)
- Code: verified for Claude Code and Codex: `app/cli/internal/trace/pointer/pointer.go`, `app/cli/internal/trace/claude/speccopies.go`. Cursor, OpenCode, and Pi have no replacer.

### TRC-013: The CLI replaces copies at push time, before redaction
The replacement runs in the CLI at push time, before the CLI records and redacts the session material.
- Why: only the CLI can read the file on disk for the match, and the redaction then never scans the replaced bytes.
- Rejected: in the control plane after the upload (the bytes are already uploaded and scanned, and the file is not on disk).
- Deciders: migmartri, javirln
- Source: [issue-3556-session-image-placeholders D-002](../specs/issue-3556-session-image-placeholders.md)
- Code: verified: `app/cli/pkg/action/trace_hook_handler.go`.

### TRC-014: A pointer replaces the content in place and keeps the block type
The pointer takes the place of the content inside the same transcript block, which keeps its type and position.
- Why: a transcript view finds each copy where it was, with no join to other data.
- Rejected: a separate list of replacements in the session material (the view must join the list to the transcript).
- Deciders: migmartri, javirln
- Source: [issue-3556-session-image-placeholders D-003](../specs/issue-3556-session-image-placeholders.md)
- Code: verified: `app/cli/internal/trace/claude/pastedimages.go`, `app/cli/internal/trace/pointer/pointer.go`.

### TRC-015: One finder for each kind of copy, with optional pointer fields
Each kind of copy (file read, file write, pasted image) has its own finder, and all use the same pointer. A later kind adds optional fields.
- Why: other inputs can use the same mechanism later.
- Rejected: an image-only pointer (each new kind needs a new format and a new view). A version and a kind field in each pointer (optional fields are enough, and the material holds the kind).
- Deciders: migmartri, javirln
- Source: [issue-3556-session-image-placeholders D-008](../specs/issue-3556-session-image-placeholders.md)
- Code: verified: `app/cli/internal/trace/pointer/pointer.go`.

### TRC-016: Replace a copy only on an exact digest match
The CLI replaces a copy only when two conditions hold. The source is stored as a material. The digest of the source file on disk equals the digest of the copy. For a file read, the CLI uses the digest of the file at the read path.
- Why: the transcript is evidence, and a wrong pointer loses content with no way back.
- Rejected: a best-effort match, for example by size or file name (a wrong match deletes content that is stored nowhere else).
- Deciders: migmartri, javirln
- Source: [issue-3556-session-image-placeholders D-010](../specs/issue-3556-session-image-placeholders.md)
- Code: verified: `app/cli/internal/trace/pointer/pointer.go`, `app/cli/internal/trace/claude/speccopies.go`.

### TRC-017: Each exact copy of a spec source is replaced, for all file types
The replacement covers each exact copy of a source in the spec list, from file reads and file writes, for all file types. Other media stay inline.
- Why: one rule covers images and text sources. Each pointer resolves in the session itself, and the CLI never decides on its own that content is evidence.
- Rejected: images only (the write copies of tickets stay, and the redaction still scans them). Upload each other media block too (the agent did not choose it).
- Deciders: migmartri, javirln
- Source: [issue-3556-session-image-placeholders D-013](../specs/issue-3556-session-image-placeholders.md)
- Code: verified: `app/cli/internal/trace/claude/speccopies.go`, `app/cli/pkg/action/trace_spec_materials.go`.

### TRC-018: The CLI captures pasted images at push time
At push time, the CLI makes an image spec entry for each distinct pasted image whose digest is not already a spec source. When the material is stored, the CLI replaces the inline copies with a pointer.
- Why: the bytes are in the transcript, so the capture depends on neither the agent nor a file on disk.
- Rejected: replace only the images that the agent captured. Images then stay inline when the agent does not copy them, and the agent can copy only from a file path.
- Deciders: migmartri, matiasinsaurralde
- Source: [issue-3569-pasted-image-pointers D-001](../specs/issue-3569-pasted-image-pointers.md)
- Code: verified for Claude Code and Codex: `app/cli/pkg/action/trace_pasted_images.go`, `app/cli/pkg/action/trace_hook_handler.go`, `app/cli/internal/trace/codex/materials.go`.

### TRC-020: Pasted image bytes come from the transcript image block
The CLI decodes the image data from the transcript block that the model got.
- Why: the data is what the model got, and it is always there.
- Rejected: read the clipboard file that the agent names. It is a temporary file that can be gone at push time, and its format is specific to one agent.
- Deciders: migmartri, matiasinsaurralde
- Source: [issue-3569-pasted-image-pointers D-005](../specs/issue-3569-pasted-image-pointers.md)
- Code: verified: `app/cli/internal/trace/provider.go`, `app/cli/internal/trace/claude/pastedimages.go`, `app/cli/internal/trace/codex/materials.go`.

### TRC-021: Local mode writes an evidence set, not a signed attestation
Local mode writes the evidence record and its materials, with no envelope and no signature.
- Why: a developer and a test need the evidence, not a signature.
- Rejected: a signed bundle with no control plane (it needs a contract and a signing key, a larger and separate effort).
- Deciders: matiasinsaurralde, migmartri
- Source: [issue-3582-trace-evidence-export D-002](../specs/issue-3582-trace-evidence-export.md)
- Code: verified: `app/cli/pkg/action/trace_local_store.go`.

### TRC-022: `trace init --local` needs no control plane
`trace init --local` sets up local mode with no login and no workflow, and writes `traceStorage: LOCAL`.
- Why: without it, init needs a login and creates a workflow on the control plane, so a developer cannot use Trace without a backend.
- Rejected: a run-only flag with no local init (init still needs the control plane).
- Deciders: matiasinsaurralde, migmartri
- Source: [issue-3582-trace-evidence-export D-009](../specs/issue-3582-trace-evidence-export.md)
- Code: verified: `app/cli/cmd/trace_init.go`.

### TRC-023: In local mode, the pre-push hook stores the session
When the storage mode is local, the pre-push hook stores the sessions on disk and contacts no control plane.
- Why: the normal flow already runs at push time, so storage there needs no new wrapper.
- Rejected: `trace run --export` as the surface (a single-run command, not the normal push-time path).
- Deciders: matiasinsaurralde, migmartri
- Source: [issue-3582-trace-evidence-export D-017](../specs/issue-3582-trace-evidence-export.md)
- Code: verified: `app/cli/cmd/trace_hook.go`, `app/cli/pkg/action/trace_hook_handler.go`.

### TRC-024: The index of a stored session is an unsigned in-toto statement
A stored session has `statement.json`, rendered by the same Chainloop renderer as a push, with no envelope.
- Why: readers and policies that read a Chainloop statement also read a stored session, and the missing envelope shows that it is unsigned.
- Rejected: a custom `manifest.json` (a second format that only Trace reads).
- Deciders: matiasinsaurralde, migmartri
- Source: [issue-3582-trace-evidence-export D-021](../specs/issue-3582-trace-evidence-export.md)
- Code: verified: `app/cli/pkg/action/trace_local_store.go`.

### TRC-025: The e2e suite runs the real agent CLI, headless, with a real model
- Why: only a real run shows whether the agent fires the hooks and shows the message, and whether the model follows the instruction.
- Rejected: replay recorded hook events into Trace (cheap and stable, but blind to the agent and the model). Both tiers (the replay tier adds little over the unit tests).
- Deciders: jiparis, javirln, migmartri
- Source: [issue-3635-trace-e2e-matrix D-001](../specs/issue-3635-trace-e2e-matrix.md)
- Code: verified: `app/cli/internal/trace/e2e/claude.go`, `app/cli/internal/trace/e2e/codex.go`.

### TRC-026: The README feature table is the expected matrix
The e2e suite reads the feature tables in the Trace README as its expected results. It keeps no second list.
- Why: one place to keep up to date, and the table must already change in the same PR as a provider change.
- Rejected: a separate data file that generates the table (two formats, and more tooling). No expected matrix (a feature that never worked gives no signal).
- Deciders: jiparis, javirln, migmartri
- Source: [issue-3635-trace-e2e-matrix D-002](../specs/issue-3635-trace-e2e-matrix.md)
- Code: verified: `app/cli/internal/trace/e2e/expected.go`.

### TRC-027: The e2e suite uses local mode only
- Why: no test organization, no long-lived token in a public repository, and no dependency on a running service.
- Rejected: local mode plus a test organization (a secret and a dependency). A control plane started in the job (slow, and more to maintain).
- Deciders: jiparis, javirln, migmartri
- Source: [issue-3635-trace-e2e-matrix D-007](../specs/issue-3635-trace-e2e-matrix.md)
- Code: verified: `app/cli/internal/trace/e2e/workspace.go`, `app/cli/internal/trace/e2e/runner.go`.

### TRC-029: Agent versions are pinned and bumped by hand
The suite configuration pins one version for each agent that the suite runs (Claude Code, Cursor, OpenCode 2, Pi, Codex). OpenCode 1.x has no pin and no adapter. A maintainer bumps a pin on purpose.
- Why: the job has access to secrets and must not run an agent release that nobody selected.
- Rejected: the latest release on each run (runs code that nobody selected in a job with secrets). Automated bump pull requests (more tooling for the first version).
- Deciders: jiparis, javirln, migmartri
- Source: [issue-3635-trace-e2e-matrix D-013](../specs/issue-3635-trace-e2e-matrix.md)
- Code: verified: `app/cli/internal/trace/e2e/config.yaml`. The suite does not install the agent. It warns when the installed version is not the pinned one.

### TRC-030: Each line range gets an attribution in the existing list
The ranges stay in the one list of each file entry, and each range gets an attribution, AI or human. When the CLI cannot blame a file at the pushed commit, that file keeps its AI ranges at edit time, with no attribution.
- Why: one list keeps the evidence small and gives readers one place to look. The new property is additive.
- Rejected: a new list of AI ranges next to the old one (two lists for the same lines). Two new lists for AI and human (a reader must merge them to show a file).
- Deciders: jiparis, migmartri
- Source: [issue-3639-line-ranges D-001](../specs/issue-3639-line-ranges.md)
- Code: verified: `pkg/attestation/crafter/materials/aicodingsession/aicodingsession.go`, `app/cli/internal/trace/attribution/enrich.go`.

### TRC-031: Range positions are in the pushed commit
Ranges are positions at the pushed commit. When the CLI cannot blame a file at the pushed commit, that file keeps its AI ranges at edit time, with no attribution.
- Why: the lines that later commits replaced are gone at that commit, so the ranges agree with the net counts. A reader of the pull request sees the same version.
- Rejected: the last commit of the session (a later commit can move or replace the lines). Ranges for each commit (a reader must combine them).
- Deciders: jiparis, migmartri
- Source: [issue-3639-line-ranges D-002](../specs/issue-3639-line-ranges.md)
- Code: verified: `app/cli/internal/trace/attribution/enrich.go`.

### TRC-032: A range with no attribution is an AI range
Evidence from an older CLI has ranges with no attribution. A file that the CLI cannot blame at the pushed commit also has them. Readers treat these as AI ranges at edit time.
- Why: platforms that store evidence from older CLIs must read both forms.
- Deciders: jiparis, migmartri
- Source: [issue-3639-line-ranges D-004](../specs/issue-3639-line-ranges.md)
- Code: verified as a documented contract: `pkg/attestation/crafter/materials/aicodingsession/aicodingsession.go`. No reader is in this repository.

### TRC-033: The reported split is the net contribution
The AI and human numbers for each file and session are the lines that the session added and that no later commit replaced. When the CLI cannot blame a file at the pushed commit, its numbers are the sum of its commits. A file with edits from before line fingerprints keeps the earlier rule.
- Why: it gives the expected split, and no line counts twice across commits.
- Rejected: authored lines for each commit, with no net (a replaced AI line still counts as AI).
- Deciders: jiparis, migmartri
- Source: [issue-3639-trace-line-attribution D-001](../specs/issue-3639-trace-line-attribution.md)
- Code: verified: `app/cli/internal/trace/attribution/enrich.go`.

### TRC-034: A replacement sends the owning session again
When a later commit replaces lines of a session, the next push sends the evidence of that session again. This occurs only when the own commits of the session are on the pushed branch, and for 30 days after the session starts (D-012, D-013).
- Why: the platform already replaces session data with newer evidence, so it needs no new concept.
- Rejected: separate evidence with no session for human commits (a new concept on the platform). No tracking of commits outside sessions (the human work stays uncounted).
- Deciders: jiparis, migmartri
- Source: [issue-3639-trace-line-attribution D-002](../specs/issue-3639-trace-line-attribution.md)
- Code: verified: `app/cli/pkg/action/trace_replacements.go`, `app/cli/pkg/action/trace_hook_handler.go`.

### TRC-035: Line owners are rebuilt from blame and the ledger
The line fingerprints go into the existing AI line ledger. At push time, the CLI rebuilds the owner of each line from git blame, the commit records, and the ledger. It stores no owner data for each line. The exception is the record of a commit that joined a session: it keeps the fingerprints of the replacing lines.
- Why: it reuses the existing storage and pushes nothing to the remote. No second store can disagree with it.
- Rejected: git notes on a dedicated ref (push traffic, ref contention, and git configuration). An owner map for each commit (it duplicates the ledger and needs its own rewrite handling).
- Deciders: jiparis, migmartri
- Source: [issue-3639-trace-line-attribution D-003](../specs/issue-3639-trace-line-attribution.md)
- Code: verified: `app/cli/internal/trace/state/ailines.go`, `app/cli/internal/trace/git/lines.go`.

### TRC-036: Two categories, human by default
A line that the CLI cannot link to an observed agent edit is human. Exception: for a file with edits from before line fingerprints, all deleted lines count as AI.
- Why: no change to the evidence categories or the platform aggregates.
- Rejected: a third "unknown" category (more exact, but it changes the schema and every aggregate).
- Deciders: jiparis, migmartri
- Source: [issue-3639-trace-line-attribution D-005](../specs/issue-3639-trace-line-attribution.md)
- Code: verified: `app/cli/internal/trace/attribution/enrich.go`.

### TRC-037: A human commit with no session joins the sessions it changes
At push time, a commit with no session joins each session whose lines it replaced. Only the replacing hunks count, as human lines. This applies only to sessions whose own commits are on the pushed branch, within the 30 days of D-012.
- Why: the human work gets credit where it changed the code of the session, and the evidence needs no new field.
- Rejected: a new field for the replaced lines of the session (the human lines are in no session total). Count the whole commit in the session (unrelated human work joins the session).
- Deciders: jiparis, migmartri
- Source: [issue-3639-trace-line-attribution D-009](../specs/issue-3639-trace-line-attribution.md)
- Code: verified: `app/cli/pkg/action/trace_replacements.go`, `app/cli/internal/trace/attribution/enrich.go`.

### TRC-038: Codex uses its native hooks
Native Codex hooks connect Codex to the session lifecycle: session start and end, prompts, patches, and shell tools.
- Why: they cover the lifecycle with no separate extension.
- Rejected: a notification-only integration (misses the state before an edit). A separate extension (duplicates lifecycle support).
- Deciders: jiparis, migmartri, matiasinsaurralde
- Source: [issue-3657-codex-trace-provider D-001](../specs/issue-3657-codex-trace-provider.md)
- Code: verified: `app/cli/internal/trace/codex/hooks.go`.

### TRC-039: The Codex provider parses a copy of the rollout
The provider keeps a private copy of the native rollout. The evidence parser reads only that copy, and estimates cost from embedded prices. During the session, a hook reads the live rollout only to find skill loads.
- Why: the evidence stays available after Codex moves or deletes its original transcript.
- Rejected: live transcript parsing (depends on agent storage during assembly). Notification payloads (no conversation and usage details).
- Deciders: jiparis, migmartri, matiasinsaurralde
- Source: [issue-3657-codex-trace-provider D-003](../specs/issue-3657-codex-trace-provider.md)
- Code: verified: `app/cli/internal/trace/codex/copy.go`, `app/cli/internal/trace/codex/parse.go`, `app/cli/internal/trace/codex/pricing.go`.

Not shown: the smaller or replaced decisions of each spec.
- 10 of [issue-3495-session-spec-capture](../specs/issue-3495-session-spec-capture.md)
- 8 of [issue-3515-spec-capture-during-session](../specs/issue-3515-spec-capture-during-session.md)
- 12 of [issue-3520-pi-trace-provider](../specs/issue-3520-pi-trace-provider.md)
- 10 of [issue-3531-spec-source-role](../specs/issue-3531-spec-source-role.md)
- 12 of [issue-3540-trace-skills-used](../specs/issue-3540-trace-skills-used.md)
- 10 of [issue-3556-session-image-placeholders](../specs/issue-3556-session-image-placeholders.md)
- 10 of [issue-3561-up-to-date-local-spec-evidence](../specs/issue-3561-up-to-date-local-spec-evidence.md)
- 5 of [issue-3569-pasted-image-pointers](../specs/issue-3569-pasted-image-pointers.md)
- 20 of [issue-3582-trace-evidence-export](../specs/issue-3582-trace-evidence-export.md)
- 8 of [issue-3635-trace-e2e-matrix](../specs/issue-3635-trace-e2e-matrix.md)
- 3 of [issue-3639-line-ranges](../specs/issue-3639-line-ranges.md)
- 8 of [issue-3639-trace-line-attribution](../specs/issue-3639-trace-line-attribution.md)
- 2 of [issue-3657-codex-trace-provider](../specs/issue-3657-codex-trace-provider.md)

## Non-Confirmed

### TRC-019: The agent keeps capturing pasted images, and its entry wins
The capture instruction for pasted images stays. When the agent and the CLI capture the same image, one entry is kept for the digest, and the entry of the agent wins.
- Why: the agent adds a role, a title, and a description that the CLI cannot know.
- Deciders: migmartri, matiasinsaurralde
- Source: [issue-3569-pasted-image-pointers D-004](../specs/issue-3569-pasted-image-pointers.md)
- Code: not confirmed: the digest rule is built (`app/cli/pkg/action/trace_pasted_images.go`), but the capture instruction is not. See Open conflicts. The parent ticket is done.

### TRC-028: The e2e suite runs nightly on main and by hand, not on pull requests
- Why: secrets are not available to pull requests from forks, and the cost of a run is high.
- Deciders: jiparis, javirln, migmartri
- Source: [issue-3635-trace-e2e-matrix D-011](../specs/issue-3635-trace-e2e-matrix.md)
- Code: not confirmed: no workflow in `.github/workflows` runs the suite. It runs only by hand with `make test-trace-e2e` (`app/cli/Makefile`). The parent ticket is done.

## Superseded

None.

## Open conflicts

- TRC-003 ([issue-3515-spec-capture-during-session D-009](../specs/issue-3515-spec-capture-during-session.md)) and TRC-019 ([issue-3569-pasted-image-pointers D-004](../specs/issue-3569-pasted-image-pointers.md)) disagree about the agent capture of pasted images, and neither has a "replaces" note. The instructions follow TRC-003 (`app/cli/pkg/action/trace_spec.go`). The CLI captures pasted images itself for Claude Code and Codex (TRC-018). For Cursor, OpenCode, and Pi, nothing records a pasted image as a source. Proposed: write a spec change that records the current rule.
- TRC-028: the parent ticket is done, but the nightly job does not exist. Proposed: write a spec change that drops the nightly run, so the suite stays manual.
