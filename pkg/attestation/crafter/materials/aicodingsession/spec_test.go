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

package aicodingsession

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSpecDigest = "sha256:3f786850e387550fdab836ed7e6dc881de23001b4a7b8f6d7e1a3d5c9b2e4f10"

func TestResolveSpecKind(t *testing.T) {
	testCases := []struct {
		name string
		kind string
		want string
	}{
		{name: "a ticket is kept", kind: SpecKindTicket, want: SpecKindTicket},
		{name: "a document is kept", kind: SpecKindDocument, want: SpecKindDocument},
		{name: "an image is kept", kind: SpecKindImage, want: SpecKindImage},
		{name: "text is kept", kind: SpecKindText, want: SpecKindText},
		{name: "an unset kind is text", kind: "", want: SpecKindText},
		{name: "case and padding are normalised", kind: "  Ticket ", want: SpecKindTicket},
		// Unlike a mode, this value comes out of a model rather than out of a
		// newer Chainloop, so a value outside the vocabulary is a mistake to
		// normalise away rather than a future kind to pass through.
		{name: "a kind outside the vocabulary is text", kind: "jira-ticket", want: SpecKindText},
		// Only the CLI makes skill entries, so an agent cannot write one.
		{name: "an agent-written skill is text", kind: SpecKindSkill, want: SpecKindText},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ResolveSpecKind(tc.kind))
		})
	}
}

func TestResolveSpecRole(t *testing.T) {
	testCases := []struct {
		name string
		role string
		want string
	}{
		{name: "a task is kept", role: SpecRoleTask, want: SpecRoleTask},
		{name: "a spec is kept", role: SpecRoleSpec, want: SpecRoleSpec},
		{name: "a plan is kept", role: SpecRolePlan, want: SpecRolePlan},
		{name: "a reference is kept", role: SpecRoleReference, want: SpecRoleReference},
		{name: "case and padding are normalised", role: "  Plan ", want: SpecRolePlan},
		// Unlike the kind, a role is never guessed: no role is a valid state,
		// and a consumer applies its own rule to it.
		{name: "an unset role is no role", role: "", want: ""},
		{name: "a role outside the vocabulary is no role", role: "design", want: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ResolveSpecRole(tc.role))
		})
	}
}

// TestSpecEntryMatchesSchema closes the loop between the struct tags here and
// the property names in the schema. Each is tested on its own elsewhere, so a
// field renamed on one side and not the other would otherwise only surface as a
// failed push against a real control plane.
func TestSpecEntryMatchesSchema(t *testing.T) {
	testCases := []struct {
		name  string
		entry SpecEntry
	}{
		{
			name: "every field populated",
			entry: SpecEntry{
				Kind: SpecKindTicket, Role: SpecRoleTask, Title: "PFM-7289: Capture the spec",
				Description: "The ticket that the session implements.",
				URI:         "https://linear.app/chainloop/issue/PFM-7289", Digest: testSpecDigest, CapturedAt: "2026-09-16T10:12:03Z",
			},
		},
		{
			// What a spec written in the session itself looks like: no source
			// to point at, so the field is absent rather than synthesised.
			name:  "only what the schema requires",
			entry: SpecEntry{Kind: SpecKindText, Digest: testSpecDigest, CapturedAt: "2026-09-16T10:12:03Z"},
		},
		{
			name: "a skill with its metadata",
			entry: SpecEntry{
				Kind: SpecKindSkill, Title: "asd-ste100", Digest: testSpecDigest, CapturedAt: "2026-10-07T10:14:51Z",
				Metadata: &SpecMetadata{
					Source: SkillSourceUser, PackageDigest: testSpecDigest,
					InvocationCount: 3, ByModel: 2, ByUser: 1, InSubagents: 1,
				},
			},
		},
		{
			// A package over the limit is not uploaded, and a session can
			// use a skill only from the model: zero counts stay in the output.
			name: "a skill without a package",
			entry: SpecEntry{
				Kind: SpecKindSkill, Title: "graphify", Digest: testSpecDigest, CapturedAt: "2026-10-07T10:14:51Z",
				Metadata: &SpecMetadata{Source: SkillSourceUnknown, InvocationCount: 1, ByModel: 1},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := json.Marshal(NewEvidence(Data{
				SchemaVersion: "v1",
				Agent:         Agent{Name: "claude-code"},
				Session:       Session{ID: "abc-123", StartedAt: "2026-09-16T10:00:00Z", DurationSeconds: 60},
				Spec:          []SpecEntry{tc.entry},
			}))
			require.NoError(t, err)

			require.NoError(t, validateEvidence(doc))
		})
	}
}
