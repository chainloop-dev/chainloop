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
	"testing"

	"github.com/stretchr/testify/assert"
)

// Repeated domain strings, extracted to satisfy goconst.
const (
	keyMinIterations     = "min_iterations"
	policyRadamsaMinIter = "radamsa-min-iterations"
	scopeShared          = "shared"
	refRadamsaDigest     = "chainloop://radamsa-min-iterations@sha256:abc"
)

func TestMergeRuntimeInputs(t *testing.T) {
	testCases := []struct {
		name          string
		with          map[string]string
		runtimeInputs map[string]string
		want          map[string]string
	}{
		{
			name:          "no runtime inputs returns contract args unchanged",
			with:          map[string]string{"ignored_paths": "a,b"},
			runtimeInputs: nil,
			want:          map[string]string{"ignored_paths": "a,b"},
		},
		{
			name:          "runtime input on empty contract key",
			with:          map[string]string{},
			runtimeInputs: map[string]string{"ignored_paths": "c\nd"},
			want:          map[string]string{"ignored_paths": "c\nd"},
		},
		{
			name:          "runtime input merges additively with contract value",
			with:          map[string]string{"ignored_paths": "a,b"},
			runtimeInputs: map[string]string{"ignored_paths": "c\nd"},
			want:          map[string]string{"ignored_paths": "a,b\nc\nd"},
		},
		{
			name:          "runtime input on a different key is added alongside",
			with:          map[string]string{"paths": "**"},
			runtimeInputs: map[string]string{"ignored_paths": "c"},
			want:          map[string]string{"paths": "**", "ignored_paths": "c"},
		},
		{
			name:          "empty contract value is replaced, not prefixed with newline",
			with:          map[string]string{"ignored_paths": ""},
			runtimeInputs: map[string]string{"ignored_paths": "c"},
			want:          map[string]string{"ignored_paths": "c"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := MergeRuntimeInputs(tc.with, tc.runtimeInputs)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestMergeRuntimeInputsDoesNotMutate ensures the input maps are left untouched.
func TestMergeRuntimeInputsDoesNotMutate(t *testing.T) {
	with := map[string]string{"ignored_paths": "a"}
	runtimeInputs := map[string]string{"ignored_paths": "b"}

	_ = MergeRuntimeInputs(with, runtimeInputs)

	assert.Equal(t, map[string]string{"ignored_paths": "a"}, with)
	assert.Equal(t, map[string]string{"ignored_paths": "b"}, runtimeInputs)
}

func TestPolicyScopeMatches(t *testing.T) {
	testCases := []struct {
		name  string
		scope string
		pname string // policy metadata name
		ref   string // attachment raw ref
		want  bool
	}{
		{
			name:  "exact metadata name",
			scope: "trusted-binaries-signed",
			pname: "trusted-binaries-signed",
			ref:   "chainloop://trusted-binaries-signed@sha256:abc",
			want:  true,
		},
		{
			name:  "exact raw ref",
			scope: "chainloop://trusted-binaries-signed@sha256:abc",
			pname: "",
			ref:   "chainloop://trusted-binaries-signed@sha256:abc",
			want:  true,
		},
		{
			name:  "bare name matches versioned ref when unversioned",
			scope: "trusted-binaries-signed",
			pname: "",
			ref:   "chainloop://trusted-binaries-signed@sha256:abc",
			want:  true,
		},
		{
			name:  "bare name matches plain ref",
			scope: "trusted-binaries-signed",
			pname: "",
			ref:   "trusted-binaries-signed",
			want:  true,
		},
		{
			name:  "org-scoped scope matches org-scoped ref",
			scope: "myorg/trusted-binaries-signed",
			pname: "trusted-binaries-signed",
			ref:   "chainloop://myorg/trusted-binaries-signed",
			want:  true,
		},
		{
			name:  "pinned digest matches same digest",
			scope: "trusted-binaries-signed@sha256:abc",
			pname: "trusted-binaries-signed",
			ref:   "chainloop://trusted-binaries-signed@sha256:abc",
			want:  true,
		},
		{
			name:  "pinned digest does not match different digest",
			scope: "trusted-binaries-signed@sha256:abc",
			pname: "trusted-binaries-signed",
			ref:   "chainloop://trusted-binaries-signed@sha256:xyz",
			want:  false,
		},
		{
			name:  "different policy name does not match",
			scope: "other-policy",
			pname: "trusted-binaries-signed",
			ref:   "chainloop://trusted-binaries-signed@sha256:abc",
			want:  false,
		},
		{
			name:  "empty scope never matches",
			scope: "",
			pname: "trusted-binaries-signed",
			ref:   "trusted-binaries-signed",
			want:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, policyScopeMatches(tc.scope, tc.pname, tc.ref))
		})
	}
}

func TestRuntimeInputsForPolicy(t *testing.T) {
	t.Run("nil receiver returns nothing", func(t *testing.T) {
		var ri *RuntimeInputs
		appendInputs, replaceInputs, matched := ri.forPolicy("p", "p")
		assert.Nil(t, appendInputs)
		assert.Nil(t, replaceInputs)
		assert.Nil(t, matched)
	})

	t.Run("global inputs apply to every policy", func(t *testing.T) {
		ri := &RuntimeInputs{Global: map[string]string{"ignored_paths": "a"}}
		appendInputs, replaceInputs, matched := ri.forPolicy("some-policy", "some-policy")
		assert.Equal(t, map[string]string{"ignored_paths": "a"}, appendInputs)
		assert.Nil(t, replaceInputs)
		assert.Empty(t, matched)
	})

	t.Run("scoped input applies only to the matching policy", func(t *testing.T) {
		ri := &RuntimeInputs{Scoped: map[string]map[string]string{
			"trusted-binaries-signed": {"ignored_paths": "a"},
		}}

		appendInputs, _, matched := ri.forPolicy("trusted-binaries-signed", "chainloop://trusted-binaries-signed@sha256:abc")
		assert.Equal(t, map[string]string{"ignored_paths": "a"}, appendInputs)
		assert.ElementsMatch(t, []string{"trusted-binaries-signed"}, matched)

		appendInputs, _, matched = ri.forPolicy("trusted-binaries-vendor-keys", "chainloop://trusted-binaries-vendor-keys")
		assert.Empty(t, appendInputs)
		assert.Empty(t, matched)
	})

	t.Run("global and scoped merge additively for the same input", func(t *testing.T) {
		ri := &RuntimeInputs{
			Global: map[string]string{"ignored_paths": "g"},
			Scoped: map[string]map[string]string{
				"trusted-binaries-signed": {"ignored_paths": "s"},
			},
		}
		appendInputs, replaceInputs, matched := ri.forPolicy("trusted-binaries-signed", "trusted-binaries-signed")
		assert.Equal(t, map[string]string{"ignored_paths": "g\ns"}, appendInputs)
		assert.Nil(t, replaceInputs)
		assert.ElementsMatch(t, []string{"trusted-binaries-signed"}, matched)
	})

	t.Run("global override applies to every policy", func(t *testing.T) {
		ri := &RuntimeInputs{GlobalOverride: map[string]string{keyMinIterations: "10"}}
		appendInputs, replaceInputs, matched := ri.forPolicy(policyRadamsaMinIter, policyRadamsaMinIter)
		assert.Nil(t, appendInputs)
		assert.Equal(t, map[string]string{keyMinIterations: "10"}, replaceInputs)
		assert.Empty(t, matched)
	})

	t.Run("scoped override applies only to the matching policy and matched is tracked", func(t *testing.T) {
		ri := &RuntimeInputs{ScopedOverride: map[string]map[string]string{
			policyRadamsaMinIter: {keyMinIterations: "10"},
		}}

		_, replaceInputs, matched := ri.forPolicy(policyRadamsaMinIter, "chainloop://radamsa-min-iterations@sha256:abc")
		assert.Equal(t, map[string]string{keyMinIterations: "10"}, replaceInputs)
		assert.ElementsMatch(t, []string{policyRadamsaMinIter}, matched)

		_, replaceInputs, matched = ri.forPolicy("other-policy", "other-policy")
		assert.Empty(t, replaceInputs)
		assert.Empty(t, matched)
	})

	t.Run("scoped override wins over global override for the same input", func(t *testing.T) {
		ri := &RuntimeInputs{
			GlobalOverride: map[string]string{keyMinIterations: "5"},
			ScopedOverride: map[string]map[string]string{
				policyRadamsaMinIter: {keyMinIterations: "10"},
			},
		}
		_, replaceInputs, matched := ri.forPolicy(policyRadamsaMinIter, policyRadamsaMinIter)
		assert.Equal(t, map[string]string{keyMinIterations: "10"}, replaceInputs)
		assert.ElementsMatch(t, []string{policyRadamsaMinIter}, matched)
	})

	t.Run("append and override for the same policy are returned separately", func(t *testing.T) {
		ri := &RuntimeInputs{
			Global:         map[string]string{"ignored_paths": "a"},
			GlobalOverride: map[string]string{keyMinIterations: "10"},
		}
		appendInputs, replaceInputs, _ := ri.forPolicy("p", "p")
		assert.Equal(t, map[string]string{"ignored_paths": "a"}, appendInputs)
		assert.Equal(t, map[string]string{keyMinIterations: "10"}, replaceInputs)
	})

	t.Run("most specific scoped override wins deterministically", func(t *testing.T) {
		// Both a bare-name scope and a digest-pinned ref scope match the same
		// attachment and set the same input to different values.
		ri := &RuntimeInputs{ScopedOverride: map[string]map[string]string{
			policyRadamsaMinIter: {keyMinIterations: "10"},
			refRadamsaDigest:     {keyMinIterations: "20"},
		}}

		// Run many times: Go map iteration is randomized, but the digest-pinned
		// (most specific) scope must win every time.
		for range 50 {
			_, replaceInputs, matched := ri.forPolicy(policyRadamsaMinIter, refRadamsaDigest)
			assert.Equal(t, map[string]string{keyMinIterations: "20"}, replaceInputs)
			assert.ElementsMatch(t, []string{policyRadamsaMinIter, refRadamsaDigest}, matched)
		}
	})

	t.Run("scoped appends apply least-specific first, deterministically", func(t *testing.T) {
		ri := &RuntimeInputs{Scoped: map[string]map[string]string{
			policyRadamsaMinIter: {keyMinIterations: "a"},
			refRadamsaDigest:     {keyMinIterations: "b"},
		}}

		for range 50 {
			appendInputs, _, _ := ri.forPolicy(policyRadamsaMinIter, refRadamsaDigest)
			// least specific (bare "a") first, most specific ("b") last
			assert.Equal(t, map[string]string{keyMinIterations: "a\nb"}, appendInputs)
		}
	})

	t.Run("does not mutate the global maps", func(t *testing.T) {
		ri := &RuntimeInputs{
			Global:         map[string]string{"ignored_paths": "g"},
			Scoped:         map[string]map[string]string{"p": {"ignored_paths": "s"}},
			GlobalOverride: map[string]string{keyMinIterations: "10"},
		}
		_, _, _ = ri.forPolicy("p", "p")
		assert.Equal(t, map[string]string{"ignored_paths": "g"}, ri.Global)
		assert.Equal(t, map[string]string{keyMinIterations: "10"}, ri.GlobalOverride)
	})
}

func TestMatchingScopesOrder(t *testing.T) {
	name := policyRadamsaMinIter
	scheme := "chainloop://" + policyRadamsaMinIter // scheme-qualified, no digest
	digest := policyRadamsaMinIter + "@sha256:abc"  // digest-pinned, no scheme
	ref := refRadamsaDigest                         // digest + scheme
	// Four scopes that all match the attachment, at increasing specificity:
	// bare name < scheme-qualified < digest-pinned < digest+scheme.
	scoped := map[string]struct{}{
		name:   {},
		scheme: {},
		digest: {},
		ref:    {},
	}

	got := matchingScopes(scoped, name, ref)
	assert.Equal(t, []string{name, scheme, digest, ref}, got)
}

func TestMatchingScopesProviderQualifiedOutranksBare(t *testing.T) {
	// A "provider:policy" scope carries no scheme "//" or org "/", only a ':';
	// it must still outrank a conflicting bare-name scope.
	name := policyRadamsaMinIter
	provider := "builtin:" + policyRadamsaMinIter

	got := matchingScopes(map[string]struct{}{name: {}, provider: {}}, name, name)
	assert.Equal(t, []string{name, provider}, got) // bare (score 0) then provider (score 1)
}

func TestOverrideRuntimeInputs(t *testing.T) {
	testCases := []struct {
		name      string
		with      map[string]string
		overrides map[string]string
		want      map[string]string
	}{
		{
			name:      "no overrides returns args unchanged",
			with:      map[string]string{keyMinIterations: "100"},
			overrides: nil,
			want:      map[string]string{keyMinIterations: "100"},
		},
		{
			name:      "override replaces the contract value",
			with:      map[string]string{keyMinIterations: "100"},
			overrides: map[string]string{keyMinIterations: "10"},
			want:      map[string]string{keyMinIterations: "10"},
		},
		{
			name:      "override on a different key is added alongside",
			with:      map[string]string{"paths": "**"},
			overrides: map[string]string{keyMinIterations: "10"},
			want:      map[string]string{"paths": "**", keyMinIterations: "10"},
		},
		{
			name:      "override replaces an appended multi-value with a scalar",
			with:      map[string]string{keyMinIterations: "100\n50"},
			overrides: map[string]string{keyMinIterations: "10"},
			want:      map[string]string{keyMinIterations: "10"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := OverrideRuntimeInputs(tc.with, tc.overrides)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestOverrideRuntimeInputsDoesNotMutate ensures the input maps are left untouched.
func TestOverrideRuntimeInputsDoesNotMutate(t *testing.T) {
	with := map[string]string{keyMinIterations: "100"}
	overrides := map[string]string{keyMinIterations: "10"}

	_ = OverrideRuntimeInputs(with, overrides)

	assert.Equal(t, map[string]string{keyMinIterations: "100"}, with)
	assert.Equal(t, map[string]string{keyMinIterations: "10"}, overrides)
}

func TestScopeTrackerUnmatched(t *testing.T) {
	testCases := []struct {
		name    string
		ri      *RuntimeInputs
		matched []string
		want    []string
	}{
		{
			name: "nil runtime inputs",
			ri:   nil,
			want: nil,
		},
		{
			name:    "all scopes matched",
			ri:      &RuntimeInputs{Scoped: map[string]map[string]string{"a": {}, "b": {}}},
			matched: []string{"a", "b"},
			want:    nil,
		},
		{
			name:    "unmatched scopes returned sorted",
			ri:      &RuntimeInputs{Scoped: map[string]map[string]string{"zebra": {}, "alpha": {}, "beta": {}}},
			matched: []string{"beta"},
			want:    []string{"alpha", "zebra"},
		},
		{
			name: "unmatched override scopes are reported too, deduped with append scopes",
			ri: &RuntimeInputs{
				Scoped:         map[string]map[string]string{"alpha": {}, scopeShared: {}},
				ScopedOverride: map[string]map[string]string{"zebra": {}, scopeShared: {}},
			},
			matched: []string{"alpha"},
			want:    []string{scopeShared, "zebra"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tracker := newScopeTracker()
			tracker.mark(tc.matched...)
			assert.Equal(t, tc.want, tracker.unmatched(tc.ri))
		})
	}
}
