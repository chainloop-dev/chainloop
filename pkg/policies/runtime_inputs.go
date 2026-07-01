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
	"sync"
)

// RuntimeInputs holds policy input values supplied at runtime (e.g. via
// --policy-input-from-file). Inputs are either global (applied to every policy
// attachment that declares them) or scoped to a specific policy (applied only
// to the attachment whose metadata name or ref matches the scope key).
type RuntimeInputs struct {
	// Global inputs, keyed by input name.
	Global map[string]string
	// Scoped inputs, keyed by policy scope (a policy name or ref) then input name.
	Scoped map[string]map[string]string
}

// forPolicy returns the runtime inputs that apply to a policy attachment
// identified by its metadata name and raw ref, together with the scope keys
// that matched. The returned map merges the global inputs with any scoped
// entries whose key matches the attachment (additively when they share an
// input name). Returns (nil, nil) when nothing applies. Nil-safe.
func (ri *RuntimeInputs) forPolicy(name, ref string) (map[string]string, []string) {
	if ri == nil || (len(ri.Global) == 0 && len(ri.Scoped) == 0) {
		return nil, nil
	}

	effective := ri.Global
	var matched []string
	for scope, inputs := range ri.Scoped {
		if policyScopeMatches(scope, name, ref) {
			matched = append(matched, scope)
			effective = MergeRuntimeInputs(effective, inputs)
		}
	}

	return effective, matched
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

// unmatched returns the sorted scope keys declared in ri that were never marked
// (i.e. matched no policy attachment), so the caller can warn about likely
// typos. Nil-safe.
func (t *scopeTracker) unmatched(ri *RuntimeInputs) []string {
	if t == nil || ri == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	var out []string
	for scope := range ri.Scoped {
		if _, ok := t.seen[scope]; !ok {
			out = append(out, scope)
		}
	}
	slices.Sort(out)
	return out
}
