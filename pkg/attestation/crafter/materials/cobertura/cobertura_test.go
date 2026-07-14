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

package cobertura_test

import (
	"encoding/json"
	"encoding/xml"
	"math"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/cobertura"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr bool
		assert  func(t *testing.T, c *cobertura.Coverage)
	}{
		{
			name: "valid cobertura report",
			input: `<?xml version="1.0"?>
<coverage line-rate="0.75" branch-rate="0.5" lines-covered="3" lines-valid="4" branches-covered="1" branches-valid="2" complexity="2" version="1.9" timestamp="1700000000">
  <sources>
    <source>src</source>
  </sources>
  <packages>
    <package name="com.example" line-rate="0.75" branch-rate="0.5" complexity="2">
      <classes>
        <class name="Main" filename="src/Main.java" line-rate="0.75" branch-rate="0.5" complexity="2">
          <lines>
            <line number="1" hits="1" branch="false"/>
            <line number="2" hits="0" branch="true" condition-coverage="50% (1/2)"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`,
			assert: func(t *testing.T, c *cobertura.Coverage) {
				assert.Equal(t, "coverage", c.XMLName.Local)
				assert.InDelta(t, 0.75, float64(c.LineRate), 0.001)
				assert.InDelta(t, 0.5, float64(c.BranchRate), 0.001)
				assert.Equal(t, 3, c.LinesCovered)
				assert.Equal(t, 4, c.LinesValid)
				assert.Equal(t, []string{"src"}, c.Sources)
				require.Len(t, c.Packages, 1)
				assert.Equal(t, "com.example", c.Packages[0].Name)
				require.Len(t, c.Packages[0].Classes, 1)
				assert.Equal(t, "src/Main.java", c.Packages[0].Classes[0].Filename)
				require.Len(t, c.Packages[0].Classes[0].Lines, 2)
				assert.True(t, c.Packages[0].Classes[0].Lines[1].Branch)
				assert.Equal(t, "50% (1/2)", c.Packages[0].Classes[0].Lines[1].ConditionCoverage)
			},
		},
		{
			// A named XMLName tag makes xml.Unmarshal reject a mismatched root
			// element (e.g. a JaCoCo <report>), which is what we rely on to
			// distinguish Cobertura from other XML coverage formats.
			name:    "wrong root element is rejected",
			input:   `<report name="debug"><package name="x"/></report>`,
			wantErr: true,
		},
		{
			name:    "malformed xml returns error",
			input:   `<coverage line-rate="0.5"`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var c cobertura.Coverage
			err := xml.Unmarshal([]byte(tc.input), &c)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			tc.assert(t, &c)
		})
	}
}

// TestRateMarshalJSON verifies that non-finite coverage ratios (NaN/Inf), which
// coverage tools emit for an empty report (line-rate="NaN", i.e. 0/0),
// serialise to JSON null instead of failing json.Marshal. Finite values keep
// their numeric form.
func TestRateMarshalJSON(t *testing.T) {
	const null = "null"
	testCases := []struct {
		name string
		in   cobertura.Rate
		want string
	}{
		{name: "finite ratio", in: cobertura.Rate(0.75), want: "0.75"},
		{name: "zero", in: cobertura.Rate(0), want: "0"},
		{name: "NaN becomes null", in: cobertura.Rate(math.NaN()), want: null},
		{name: "positive infinity becomes null", in: cobertura.Rate(math.Inf(1)), want: null},
		{name: "negative infinity becomes null", in: cobertura.Rate(math.Inf(-1)), want: null},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}

// TestEmptyReportMarshalsToValidJSON is the regression guard for the core
// requirement: a legitimate empty report (no measurable lines) must project to
// valid JSON so the policy engine can evaluate it and treat it as valid rather
// than erroring. line-rate is null and lines-valid is 0, which lets a policy
// guard on lines-valid > 0 before interpreting coverage.
func TestEmptyReportMarshalsToValidJSON(t *testing.T) {
	const emptyReport = `<?xml version="1.0" encoding="UTF-8"?>
<coverage line-rate="NaN" branch-rate="0" lines-covered="0" lines-valid="0" branches-covered="0" branches-valid="0" complexity="0">
  <sources></sources>
  <packages></packages>
</coverage>`

	var c cobertura.Coverage
	require.NoError(t, xml.Unmarshal([]byte(emptyReport), &c))

	out, err := json.Marshal(&c)
	require.NoError(t, err, "empty report must marshal without a NaN error")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out, &decoded))
	assert.Nil(t, decoded["line-rate"], "NaN line-rate must project as null")
	assert.EqualValues(t, 0, decoded["lines-valid"], "lines-valid stays 0 so policies can detect an empty report")
}
