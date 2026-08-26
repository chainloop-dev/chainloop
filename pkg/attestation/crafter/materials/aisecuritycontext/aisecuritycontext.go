//
// Copyright 2026 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package aisecuritycontext defines the wire types of the CHAINLOOP_AI_SECURITY_CONTEXT
// material: a security context compiled from a repository's fix history, carrying
// recurring vulnerability fingerprints, the attack surfaces they share, ranked
// risks, and byte-verifiable evidence anchors backing each claim.
//
// These structs mirror the producer's contract and exist for annotation
// extraction only. Validation always runs on the generically decoded payload,
// because a typed decode drops unknown fields and would defeat the schema's
// additionalProperties: false.
package aisecuritycontext

const (
	// EvidenceID is the identifier for the AI security context material type.
	EvidenceID = "CHAINLOOP_AI_SECURITY_CONTEXT"
	// EvidenceSchemaURL is the namespace label of the JSON schema for the AI
	// security context payload. Nothing fetches it; the producer writes this
	// exact string into the envelope and the validator binds it to an embedded
	// schema at compile time.
	EvidenceSchemaURL = "https://schemas.chainloop.dev/aisecuritycontext/0.1/ai-security-context.schema.json"
)

// RepoRef identifies the scanned repository at a specific revision. Downstream
// this is a cross-check against the dispatch record that requested the scan,
// never a join key.
type RepoRef struct {
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Ref     string `json:"ref"`
	HeadSHA string `json:"head_sha"`
}

// Provenance records what produced the artifact. The output is model-dependent,
// so a scan is not reproducible or auditable without the tool version and the
// per-phase model, prompt, and profile identity.
type Provenance struct {
	Tool                 string `json:"tool"`
	ToolVersion          string `json:"tool_version"`
	Protocol             string `json:"protocol"`
	TriageModel          string `json:"triage_model,omitempty"`
	AdjudicationModel    string `json:"adjudication_model,omitempty"`
	TriagePromptID       string `json:"triage_prompt_id,omitempty"`
	AdjudicationPromptID string `json:"adjudication_prompt_id,omitempty"`
	InputProfile         string `json:"input_profile,omitempty"`
	DecisionProfile      string `json:"decision_profile,omitempty"`
	CWECatalogVersion    string `json:"cwe_catalog_version,omitempty"`
	AnchorVerification   string `json:"anchor_verification,omitempty"`
}

// ScanWindow pins the history the scan covered, so a reader knows what was not
// examined.
type ScanWindow struct {
	FromSHA string `json:"from_sha,omitempty"`
	ToSHA   string `json:"to_sha,omitempty"`
	LastN   int    `json:"last_n,omitempty"`
}

// Unresolved is one commit inside the covered window that produced no answer.
type Unresolved struct {
	SHA    string `json:"sha"`
	Reason string `json:"reason"`
}

// ScanStats is the funnel every stage reports into, so that a silent failure
// cannot masquerade as a clean scan. Reconciles asserts
//
//	adjudicated == findings + abstained + rejected + no_finding + adjudication_errors
//
// and a false value means the scan must not be read as complete.
type ScanStats struct {
	Window              ScanWindow   `json:"window"`
	CommitsScanned      int          `json:"commits_scanned"`
	CommitsTriaged      int          `json:"commits_triaged"`
	CommitsSkipped      int          `json:"commits_skipped"`
	TriageCandidates    int          `json:"triage_candidates"`
	Adjudicated         int          `json:"adjudicated"`
	Findings            int          `json:"findings"`
	Abstained           int          `json:"abstained"`
	Rejected            int          `json:"rejected"`
	NoFinding           int          `json:"no_finding"`
	TriageErrors        int          `json:"triage_errors"`
	AdjudicationErrors  int          `json:"adjudication_errors"`
	AnchorsVerified     int          `json:"anchors_verified"`
	AnchorsRelocated    int          `json:"anchors_relocated"`
	AnchorsRejected     int          `json:"anchors_rejected"`
	InputTokens         int64        `json:"input_tokens"`
	OutputTokens        int64        `json:"output_tokens"`
	WallClockS          float64      `json:"wall_clock_s"`
	Reconciles          bool         `json:"reconciles"`
	DuplicatesMerged    int          `json:"duplicates_merged,omitempty"`
	Unresolved          []Unresolved `json:"unresolved,omitempty"`
	TruncatedUnresolved int          `json:"truncated_unresolved,omitempty"`
}

// TopRisk is a component with a security-fix history, ranked by severity mass
// then recency-weighted fix count.
type TopRisk struct {
	Component     string   `json:"component"`
	Kind          string   `json:"kind"`
	Classes       []string `json:"classes"`
	Severity      string   `json:"severity"`
	FixCount      int      `json:"fix_count"`
	Recurring     bool     `json:"recurring"`
	SeverityMass  int      `json:"severity_mass"`
	RecencyWeight float64  `json:"recency_weight"`
	CWE           []string `json:"cwe"`
	Evidence      []string `json:"evidence"`
}

// SharedSurface is one invariant that must hold at every entry point reaching a
// shared sink, corroborated by two or more independent fixes.
//
// Class is not a single taxonomy token: surfaces cluster on shared sink symbols,
// so a cluster spanning several classes carries them slash-joined.
type SharedSurface struct {
	Surface     string   `json:"surface"`
	Class       string   `json:"class"`
	Guard       []string `json:"guard"`
	GuardKind   string   `json:"guard_kind"`
	SinkSymbols []string `json:"sink_symbols"`
	EntryPoints []string `json:"entry_points"`
	CheckHint   string   `json:"check_hint"`
	Support     int      `json:"support"`
	OriginFixes []string `json:"origin_fixes"`
}

// CommitRef names a commit and what it did there, with the producer's proof
// attached. Verified means the commit was proved to be an ancestor of the fix —
// not that it is where the flaw actually entered.
type CommitRef struct {
	CommitSHA   string `json:"commit_sha"`
	Description string `json:"description,omitempty"`
	CommittedAt string `json:"committed_at,omitempty"`
	Verified    bool   `json:"verified"`
}

// Anchor is a byte-verifiable evidence span. A consumer can re-verify it
// independently: git show <revision_sha>:<path> sliced at the line range must
// hash to SpanSHA256.
type Anchor struct {
	Revision    string   `json:"revision"`
	RevisionSHA string   `json:"revision_sha"`
	Path        string   `json:"path"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	QuotedSpan  string   `json:"quoted_span"`
	SpanSHA256  string   `json:"span_sha256"`
	Verified    bool     `json:"verified"`
	Relocated   bool     `json:"relocated"`
	Supports    []string `json:"supports,omitempty"`
}

// Severity carries the level together with how it was derived, so a consumer can
// tell a CVSS-backed rating from a model estimate.
type Severity struct {
	Level  string   `json:"level"`
	Source string   `json:"source"`
	Score  *float64 `json:"score,omitempty"`
	Vector string   `json:"vector,omitempty"`
}

// Reachability records how a fix changed what an attacker can reach. Before and
// After are the evidence for the verdict, not a restatement of it.
type Reachability struct {
	Verdict string `json:"verdict"`
	Before  string `json:"before,omitempty"`
	After   string `json:"after,omitempty"`
}

// Fingerprint is one adjudicated past fix, the atom the security context is
// built from.
type Fingerprint struct {
	ID            string `json:"id"`
	PatchID       string `json:"patch_id,omitempty"`
	CommitSHA     string `json:"commit_sha"`
	CommitDate    string `json:"commit_date"`
	CommitSubject string `json:"commit_subject"`

	Class string   `json:"class"`
	CWE   []string `json:"cwe"`

	// Components are the paths the fix modified; ReachableFrom are paths the
	// vulnerability was reachable through but which the fix did not change.
	// Derived sections key off Components, so folding the two together would
	// manufacture hot components out of untouched files.
	Components    []string `json:"components"`
	ReachableFrom []string `json:"reachable_from,omitempty"`

	SinkSymbols  []string `json:"sink_symbols,omitempty"`
	GuardSymbols []string `json:"guard_symbols,omitempty"`
	Sink         string   `json:"sink,omitempty"`
	FixKind      string   `json:"fix_kind,omitempty"`
	FixShape     string   `json:"fix_shape,omitempty"`

	Severity   Severity `json:"severity"`
	Confidence string   `json:"confidence,omitempty"`

	Summary               string `json:"summary"`
	POC                   string `json:"poc,omitempty"`
	RootCause             string `json:"root_cause,omitempty"`
	AttackerPreconditions string `json:"attacker_preconditions,omitempty"`

	Invariant       string `json:"invariant,omitempty"`
	FixCompleteness string `json:"fix_completeness"`

	Anchors []Anchor `json:"anchors,omitempty"`

	Reachability       Reachability `json:"reachability"`
	FailureContainment string       `json:"failure_containment,omitempty"`

	// IntroducedBy lists commits that each independently introduced the flaw —
	// a set of co-introductions, never candidates the producer was choosing
	// between. Absent, never empty, means the archaeology was inconclusive.
	IntroducedBy             []CommitRef `json:"introduced_by,omitempty"`
	IntroducedToFixedSeconds int64       `json:"introduced_to_fixed_seconds,omitempty"`

	// Status is empty when the invariant the fix established still holds, and
	// "reverted" when a later commit undid it. A reverted fingerprint is
	// annotated rather than dropped.
	Status     string `json:"status,omitempty"`
	RevertedBy string `json:"reverted_by,omitempty"`
}

// Data is the AI security context payload: the object the producer writes under
// the envelope's `data` field, and what the JSON schema validates.
type Data struct {
	SchemaVersion string     `json:"schema_version"`
	GeneratedAt   string     `json:"generated_at"`
	Repo          RepoRef    `json:"repo"`
	Provenance    Provenance `json:"provenance"`
	Scan          ScanStats  `json:"scan"`

	ClassCounts map[string]int `json:"class_counts"`

	// MinSupport is how many origin fixes a derived claim needs before it is
	// stated as a pattern, emitted so a reader can tell a suppressed claim from
	// an absent one.
	MinSupport int `json:"min_support"`

	TopRisks       []TopRisk       `json:"top_risks"`
	SharedSurfaces []SharedSurface `json:"shared_surfaces"`
	Fingerprints   []Fingerprint   `json:"fingerprints"`
}

// Evidence is the Chainloop material envelope around a security context.
//
// The producer writes it: the crafter streams the file as it sits on disk and
// records the digest of those exact bytes, so a wrapper added downstream would
// mean attesting a digest for content that was never written. The envelope is
// also what makes the blob self-identifying once it is in CAS with none of the
// surrounding attestation context.
type Evidence struct {
	ID     string `json:"chainloop.material.evidence.id"`
	Schema string `json:"schema"`
	Data   Data   `json:"data"`
}

// NewEvidence wraps a payload in the material envelope.
func NewEvidence(data Data) *Evidence {
	return &Evidence{
		ID:     EvidenceID,
		Schema: EvidenceSchemaURL,
		Data:   data,
	}
}
