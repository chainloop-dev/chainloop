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

package policies

import (
	"maps"
	"slices"
	"strings"
	"sync"
)

// RuntimeInputs holds policy input values supplied at runtime (e.g. via
// --policy-input-from-file, --policy-input-from-file-replace or --policy-input).
// Inputs are either global (applied to every policy attachment that declares
// them) or scoped to a specific policy (applied only to the attachment whose
// metadata name or ref matches the scope key). Independently, each input is
// merged onto the contract value either additively (append) or by replacing it
// (override); the two modes live in separate maps so the same input name can be
// appended by one flag and never collides with an override of another.
type RuntimeInputs struct {
	// Global holds append-mode inputs, keyed by input name.
	Global map[string]string
	// Scoped holds append-mode inputs, keyed by policy scope (a policy name or
	// ref) then input name.
	Scoped map[string]map[string]string
	// GlobalOverride holds replace-mode inputs, keyed by input name.
	GlobalOverride map[string]string
	// ScopedOverride holds replace-mode inputs, keyed by policy scope then input
	// name.
	ScopedOverride map[string]map[string]string
}

// empty reports whether ri carries no inputs at all. Nil-safe.
func (ri *RuntimeInputs) empty() bool {
	return ri == nil || (len(ri.Global) == 0 && len(ri.Scoped) == 0 &&
		len(ri.GlobalOverride) == 0 && len(ri.ScopedOverride) == 0)
}

// forPolicy returns the runtime inputs that apply to a policy attachment
// identified by its metadata name and raw ref, together with the scope keys
// that matched. Append-mode inputs (appendInputs) merge the global inputs with
// any scoped entries whose key matches the attachment (additively when they
// share an input name). Replace-mode inputs (replaceInputs) merge the global
// overrides with any matching scoped overrides, the scoped ones winning. The
// caller layers appendInputs onto the contract additively and then applies
// replaceInputs on top, so a replace always wins over an append for the same
// key. Returns (nil, nil, nil) when nothing applies. Nil-safe.
func (ri *RuntimeInputs) forPolicy(name, ref string) (appendInputs, replaceInputs map[string]string, matched []string) {
	if ri.empty() {
		return nil, nil, nil
	}

	seen := make(map[string]struct{})
	markMatch := func(scope string) {
		if _, ok := seen[scope]; !ok {
			seen[scope] = struct{}{}
			matched = append(matched, scope)
		}
	}

	// Apply matching scopes least-specific first so that, when several scopes
	// target the same attachment and set the same input, the most specific one
	// is applied last and wins — deterministically, regardless of Go's map
	// iteration order.
	appendInputs = ri.Global
	for _, scope := range matchingScopes(ri.Scoped, name, ref) {
		markMatch(scope)
		appendInputs = MergeRuntimeInputs(appendInputs, ri.Scoped[scope])
	}

	replaceInputs = ri.GlobalOverride
	for _, scope := range matchingScopes(ri.ScopedOverride, name, ref) {
		markMatch(scope)
		replaceInputs = OverrideRuntimeInputs(replaceInputs, ri.ScopedOverride[scope])
	}

	return appendInputs, replaceInputs, matched
}

// matchingScopes returns the keys of scoped that target the attachment
// identified by (name, ref), ordered so more specific scopes come last. Callers
// apply them in that order, so a later (more specific) scope wins when several
// set the same input. Ties break on the scope string, so the order — and thus
// the merged result — is deterministic regardless of Go's randomized map
// iteration.
func matchingScopes[V any](scoped map[string]V, name, ref string) []string {
	var out []string
	for scope := range scoped {
		if policyScopeMatches(scope, name, ref) {
			out = append(out, scope)
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		if d := scopeSpecificity(a) - scopeSpecificity(b); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	return out
}

// scopeSpecificity scores how narrowly a scope key targets a policy: a scope
// that pins a digest is the most specific, and a scope carrying anything beyond
// the bare policy name — a scheme (chainloop://), an org path (org/name) or a
// provider prefix (provider:name) — is more specific than a bare name.
func scopeSpecificity(scope string) int {
	_, digest := splitPolicyRef(scope)

	// Drop the "@sha256:<digest>" suffix so its own ':' does not count as
	// provider/scheme qualification below.
	head := scope
	if digest != "" {
		if i := strings.Index(scope, "@"); i >= 0 {
			head = scope[:i]
		}
	}

	score := 0
	if digest != "" {
		score += 2
	}
	// A scheme (chainloop://), org path (org/name) or provider prefix
	// (provider:name) all introduce a ':' or '/' beyond the bare name.
	if strings.ContainsAny(head, ":/") {
		score++
	}
	return score
}

// policyScopeMatches reports whether a runtime-input scope key targets the
// policy attachment identified by its metadata name and raw ref. A scope
// matches when it equals the name or the ref exactly, or when its bare name
// (scheme, org and @sha256: digest stripped) matches; if the scope pins a
// digest, the ref must carry the same digest, otherwise any version matches.
func policyScopeMatches(scope, name, ref string) bool {
	if scope == "" {
		return false
	}
	if scope == name || scope == ref {
		return true
	}

	scopeName, scopeDigest := splitPolicyRef(scope)
	if scopeName == "" {
		return false
	}
	refName, refDigest := splitPolicyRef(ref)
	if scopeName != refName && scopeName != name {
		return false
	}
	if scopeDigest != "" {
		return scopeDigest == refDigest
	}
	return true
}

// splitPolicyRef normalizes a policy reference to its bare name and digest,
// stripping any scheme, org scope and @sha256: version using the same parsers
// the loaders use.
func splitPolicyRef(ref string) (name, digest string) {
	return ExtractDigest(ProviderParts(ref).Name)
}

// MergeRuntimeInputs returns the contract arguments with the runtime inputs
// merged in additively: when both define the same key, the runtime value is
// appended after the contract value (newline-separated) so file-sourced
// exemptions add to, rather than replace, contract-declared ones. The input
// maps are not mutated. Exported so callers assembling runtime inputs (e.g. the
// CLI's --policy-input-from-file handling) reuse the same multi-value encoding.
func MergeRuntimeInputs(with, runtimeInputs map[string]string) map[string]string {
	if len(runtimeInputs) == 0 {
		return with
	}

	merged := make(map[string]string, len(with)+len(runtimeInputs))
	maps.Copy(merged, with)
	for k, v := range runtimeInputs {
		if existing := merged[k]; existing != "" {
			merged[k] = existing + "\n" + v
		} else {
			merged[k] = v
		}
	}

	return merged
}

// OverrideRuntimeInputs returns the given arguments with the override inputs
// applied by replacement: each override key's value replaces whatever the
// arguments held for that key (a contract value or an appended runtime value),
// rather than being newline-appended. This is what makes a scalar input
// overridable at run time: the value no longer collapses into a multi-value
// list through append-time newline joining. The value is still normalized like
// any other input downstream (getInputArguments splits it on newlines and
// commas), so a single comma-free value such as "10" stays a scalar while a
// value that itself contains commas expands into a list; a literal comma must
// be escaped as "\,". The input maps are not mutated.
func OverrideRuntimeInputs(with, overrides map[string]string) map[string]string {
	if len(overrides) == 0 {
		return with
	}

	merged := make(map[string]string, len(with)+len(overrides))
	maps.Copy(merged, with)
	maps.Copy(merged, overrides)

	return merged
}

// scopeTracker records, concurrency-safely, which runtime-input scope keys were
// matched by at least one policy attachment during a material evaluation.
type scopeTracker struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func newScopeTracker() *scopeTracker {
	return &scopeTracker{seen: make(map[string]struct{})}
}

func (t *scopeTracker) mark(keys ...string) {
	if t == nil || len(keys) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, k := range keys {
		t.seen[k] = struct{}{}
	}
}

// unmatched returns the sorted scope keys declared in ri (across both append-
// and replace-mode scoped inputs) that were never marked (i.e. matched no policy
// attachment), so the caller can warn about likely typos. Nil-safe.
func (t *scopeTracker) unmatched(ri *RuntimeInputs) []string {
	if t == nil || ri == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	scopes := make(map[string]struct{}, len(ri.Scoped)+len(ri.ScopedOverride))
	for scope := range ri.Scoped {
		scopes[scope] = struct{}{}
	}
	for scope := range ri.ScopedOverride {
		scopes[scope] = struct{}{}
	}

	var out []string
	for scope := range scopes {
		if _, ok := t.seen[scope]; !ok {
			out = append(out, scope)
		}
	}
	slices.Sort(out)
	return out
}
