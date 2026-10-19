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

package e2e

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixture values shared by the tests of the package.
const (
	featTranscript      = "Transcript"
	featSecretRedaction = "Secret redaction"
	colCursor           = "Cursor"
	testHarnessVersion  = "2.1.0"
	testModel           = "haiku"
)

const twoTables = `# Title

Intro text.

| Feature | What it means | Claude Code | Cursor |
|---------|---------------|-------------|--------|
| Transcript | The evidence holds the conversation. | Yes | Partial |
| Welcome message | The user sees a **message**. | Yes | No |

Why not Yes:

- **Transcript, Cursor:** reasons.

| Agent | Hook installation | Hooks |
|-------|-------------------|-------|
| Claude Code | settings | many |

| Feature | What it means | Claude Code | Cursor |
|---------|---------------|-------------|--------|
| Secret redaction | Secrets are redacted. | **Yes** | Not applicable |
| Media skipped | Media is skipped. | Unknown | Unknown |
`

func TestParseExpected(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		wantErr  string
		features []string
		cells    map[[2]string]Support
	}{
		{
			name:     "feature tables only, other tables ignored",
			input:    twoTables,
			features: []string{featTranscript, "Welcome message", featSecretRedaction, "Media skipped"},
			cells: map[[2]string]Support{
				{featTranscript, "Claude Code"}:      SupportYes,
				{featTranscript, colCursor}:          SupportPartial,
				{"Welcome message", colCursor}:       SupportNo,
				{featSecretRedaction, "Claude Code"}: SupportYes,
				{featSecretRedaction, colCursor}:     SupportNotApplicable,
				{"Media skipped", colCursor}:         SupportUnknown,
			},
		},
		{
			name: "unknown cell value",
			input: `| Feature | What it means | Claude Code |
|---|---|---|
| Transcript | x | Maybe |
`,
			wantErr: `feature "Transcript", column "Claude Code": unknown value "Maybe"`,
		},
		{
			name: "duplicate feature",
			input: `| Feature | What it means | Claude Code |
|---|---|---|
| Transcript | x | Yes |
| Transcript | y | Yes |
`,
			wantErr: `feature "Transcript" is listed twice`,
		},
		{
			name: "row with missing cells",
			input: `| Feature | What it means | Claude Code | Cursor |
|---|---|---|---|
| Transcript | x | Yes |
`,
			wantErr: `feature "Transcript" has 3 cells, want 4`,
		},
		{
			name:    "no feature table",
			input:   "# nothing here\n",
			wantErr: "no feature table found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseExpected(strings.NewReader(tc.input))
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.features, got.Features)
			for key, want := range tc.cells {
				gotSupport, ok := got.Support(key[0], key[1])
				assert.True(t, ok, "cell %v", key)
				assert.Equal(t, want, gotSupport, "cell %v", key)
			}
		})
	}
}

func TestSupportTested(t *testing.T) {
	testCases := []struct {
		support Support
		want    bool
	}{
		{SupportYes, true},
		{SupportPartial, true},
		{SupportUnknown, true},
		{SupportNo, false},
		{SupportNotApplicable, false},
	}
	for _, tc := range testCases {
		t.Run(string(tc.support), func(t *testing.T) {
			assert.Equal(t, tc.want, tc.support.Tested())
		})
	}
}

// TestTraceREADMEParses keeps the real feature table readable by the suite:
// a README edit that breaks the table shape fails here, not in the nightly run.
func TestTraceREADMEParses(t *testing.T) {
	f, err := os.Open(traceREADMEPath)
	require.NoError(t, err)
	defer f.Close()

	got, err := ParseExpected(f)
	require.NoError(t, err)
	assert.Contains(t, got.Harnesses, HarnessClaudeCode)
	assert.Contains(t, got.Features, featTranscript)
}
