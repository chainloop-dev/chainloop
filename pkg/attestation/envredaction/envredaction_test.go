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

package envredaction

import (
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	actorVar = "GITHUB_ACTOR"
	shaVar   = "GITHUB_SHA"
	fooVar   = "FOO"
	userVar  = "USER"
	shaValue = "abc"
	userName = "jdoe"
)

func TestNames(t *testing.T) {
	testCases := []struct {
		name    string
		mode    Mode
		custom  []string
		want    []string
		wantErr bool
	}{
		{name: "none", mode: ModeNone, want: nil},
		{name: "none ignores custom list", mode: ModeNone, custom: []string{fooVar}, want: nil},
		{
			name: "known identity",
			mode: ModeKnownIdentity,
			want: runners.KnownIdentityEnvVars(),
		},
		{name: "custom", mode: ModeCustom, custom: []string{fooVar, "BAR"}, want: []string{fooVar, "BAR"}},
		{name: "custom does not add known identity", mode: ModeCustom, custom: []string{fooVar}, want: []string{fooVar}},
		{name: "custom requires a variable", mode: ModeCustom, wantErr: true},
		{name: "custom rejects empty names", mode: ModeCustom, custom: []string{fooVar, " "}, wantErr: true},
		{name: "custom rejects names with surrounding whitespace", mode: ModeCustom, custom: []string{" " + fooVar}, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Names(tc.mode, tc.custom)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestApply(t *testing.T) {
	testCases := []struct {
		name  string
		env   map[string]string
		names []string
		want  map[string]string
	}{
		{
			name:  "redacts present values and keeps the key",
			env:   map[string]string{actorVar: userName, shaVar: shaValue},
			names: []string{actorVar},
			want:  map[string]string{actorVar: RedactedValue, shaVar: shaValue},
		},
		{
			name:  "does not add absent variables",
			env:   map[string]string{shaVar: shaValue},
			names: []string{actorVar},
			want:  map[string]string{shaVar: shaValue},
		},
		{
			name:  "no names leaves env unchanged",
			env:   map[string]string{actorVar: userName},
			names: nil,
			want:  map[string]string{actorVar: userName},
		},
		{
			name:  "exact names only",
			env:   map[string]string{"GITHUB_ACTOR_ID": "123"},
			names: []string{actorVar},
			want:  map[string]string{"GITHUB_ACTOR_ID": "123"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			Apply(tc.env, tc.names)
			assert.Equal(t, tc.want, tc.env)
		})
	}
}

func TestViolations(t *testing.T) {
	testCases := []struct {
		name  string
		env   map[string]string
		names []string
		want  []string
	}{
		{
			name:  "redacted values are fine",
			env:   map[string]string{actorVar: RedactedValue, shaVar: shaValue},
			names: []string{actorVar},
			want:  nil,
		},
		{
			name:  "absent values are fine",
			env:   map[string]string{shaVar: shaValue},
			names: []string{actorVar},
			want:  nil,
		},
		{
			name:  "clear values are reported sorted",
			env:   map[string]string{userVar: userName, actorVar: userName, "HOME": RedactedValue},
			names: []string{userVar, actorVar, "HOME"},
			want:  []string{actorVar, userVar},
		},
		{
			name:  "empty values reveal nothing and are fine",
			env:   map[string]string{actorVar: ""},
			names: []string{actorVar},
			want:  nil,
		},
		{
			name:  "nil env",
			env:   nil,
			names: []string{userVar},
			want:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Violations(tc.env, tc.names))
		})
	}
}
