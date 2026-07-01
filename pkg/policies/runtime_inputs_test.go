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
		got, matched := ri.forPolicy("p", "p")
		assert.Nil(t, got)
		assert.Nil(t, matched)
	})

	t.Run("global inputs apply to every policy", func(t *testing.T) {
		ri := &RuntimeInputs{Global: map[string]string{"ignored_paths": "a"}}
		got, matched := ri.forPolicy("some-policy", "some-policy")
		assert.Equal(t, map[string]string{"ignored_paths": "a"}, got)
		assert.Empty(t, matched)
	})

	t.Run("scoped input applies only to the matching policy", func(t *testing.T) {
		ri := &RuntimeInputs{Scoped: map[string]map[string]string{
			"trusted-binaries-signed": {"ignored_paths": "a"},
		}}

		got, matched := ri.forPolicy("trusted-binaries-signed", "chainloop://trusted-binaries-signed@sha256:abc")
		assert.Equal(t, map[string]string{"ignored_paths": "a"}, got)
		assert.ElementsMatch(t, []string{"trusted-binaries-signed"}, matched)

		got, matched = ri.forPolicy("trusted-binaries-vendor-keys", "chainloop://trusted-binaries-vendor-keys")
		assert.Empty(t, got)
		assert.Empty(t, matched)
	})

	t.Run("global and scoped merge additively for the same input", func(t *testing.T) {
		ri := &RuntimeInputs{
			Global: map[string]string{"ignored_paths": "g"},
			Scoped: map[string]map[string]string{
				"trusted-binaries-signed": {"ignored_paths": "s"},
			},
		}
		got, matched := ri.forPolicy("trusted-binaries-signed", "trusted-binaries-signed")
		assert.Equal(t, map[string]string{"ignored_paths": "g\ns"}, got)
		assert.ElementsMatch(t, []string{"trusted-binaries-signed"}, matched)
	})

	t.Run("does not mutate the global map", func(t *testing.T) {
		ri := &RuntimeInputs{
			Global: map[string]string{"ignored_paths": "g"},
			Scoped: map[string]map[string]string{"p": {"ignored_paths": "s"}},
		}
		_, _ = ri.forPolicy("p", "p")
		assert.Equal(t, map[string]string{"ignored_paths": "g"}, ri.Global)
	})
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tracker := newScopeTracker()
			tracker.mark(tc.matched...)
			assert.Equal(t, tc.want, tracker.unmatched(tc.ri))
		})
	}
}
