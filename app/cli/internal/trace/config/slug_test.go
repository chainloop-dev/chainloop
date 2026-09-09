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

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Fixtures the table cases share.
const (
	validName = "my-project"
	// errLowercase is the fragment the DNS-1123 message uses for a name whose
	// characters or edges are wrong, as opposed to its length.
	errLowercase = "lowercase"
)

func TestSlugifyDNS1123(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty input", input: "", want: ""},
		{name: "an already valid name is unchanged", input: validName, want: validName},
		{name: "capitals are lowercased", input: "MyProject", want: "myproject"},
		{name: "spaces become dashes", input: "My Cool Project", want: "my-cool-project"},
		{name: "runs of separators collapse into one dash", input: "  --Foo_Bar!! ", want: "foo-bar"},
		{name: "punctuation is dropped", input: "acme.io/payments (v2)", want: "acme-io-payments-v2"},
		{name: "leading and trailing dashes are trimmed", input: "---edge---", want: "edge"},
		{name: "digits are kept, including as the first character", input: "3D Renderer", want: "3d-renderer"},
		{name: "underscores are separators", input: "snake_case_name", want: "snake-case-name"},
		{name: "non-ascii characters are dropped", input: "café münchen", want: "caf-m-nchen"},
		{name: "a name of only punctuation normalizes to empty", input: "!!!", want: ""},
		{
			name:  "an over-long name is cut to 63 characters",
			input: strings.Repeat("a", 70),
			want:  strings.Repeat("a", 63),
		},
		{
			// Cutting at 63 must not leave a trailing dash, which would be invalid.
			name:  "cutting does not leave a trailing dash",
			input: strings.Repeat("a", 63) + " tail",
			want:  strings.Repeat("a", 63),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := SlugifyDNS1123(tc.input)
			assert.Equal(t, tc.want, got)

			// Whatever comes out must either be empty or be a name the server accepts.
			if got != "" {
				assert.Empty(t, validation.IsDNS1123Label(got), "slug %q is not a valid DNS-1123 label", got)
			}
		})
	}
}

func TestSlugifyDNS1123IsIdempotent(t *testing.T) {
	for _, input := range []string{"My Cool Project", "  --Foo_Bar!! ", strings.Repeat("a", 70), "already-valid"} {
		once := SlugifyDNS1123(input)
		assert.Equal(t, once, SlugifyDNS1123(once), "slugifying %q twice changed the result", input)
	}
}

func TestValidateDNS1123Label(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "a valid name passes", input: validName},
		{name: "a single character is valid", input: "a"},
		{name: "63 characters is valid", input: strings.Repeat("a", 63)},
		{name: "empty is rejected", input: "", wantErr: "cannot be empty"},
		{name: "capitals are rejected", input: "MyProject", wantErr: errLowercase},
		{name: "spaces are rejected", input: "my project", wantErr: errLowercase},
		{name: "a leading dash is rejected", input: "-my-project", wantErr: errLowercase},
		{name: "a trailing dash is rejected", input: "my-project-", wantErr: errLowercase},
		{name: "64 characters is rejected", input: strings.Repeat("a", 64), wantErr: "63"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDNS1123Label(ProjectSubject, tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			// The rule is the same for both; only the message differs.
			assert.ErrorContains(t, ValidateDNS1123Label(OrganizationSubject, tc.input), "organization")
		})
	}
}
