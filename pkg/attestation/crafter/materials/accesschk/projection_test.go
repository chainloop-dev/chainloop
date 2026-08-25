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

package accesschk

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	everyone     = "Everyone"
	keyAllAccess = "KEY_ALL_ACCESS"
)

// Project must be lossless: every object's descriptor is exactly reconstructable
// from the descriptors table, and identical descriptors are shared.
func TestProjectIsLossless(t *testing.T) {
	const sys = "SYSTEM"
	report := &Report{
		Tool: Tool{Name: ToolName, Version: "6.15"},
		Objects: []Object{
			{Name: "key1", Owner: sys, DACL: []ACE{{Index: 0, Principal: everyone, Rights: []string{keyAllAccess}}}, AccessEntries: []AccessEntry{}},
			{Name: "key2", Owner: sys, DACL: []ACE{{Index: 0, Principal: everyone, Rights: []string{keyAllAccess}}}, AccessEntries: []AccessEntry{}}, // same descriptor as key1
			{Name: "key3", Owner: "Admins", DACL: []ACE{{Index: 0, Principal: sys, Rights: []string{"KEY_READ"}}}, AccessEntries: []AccessEntry{}},   // distinct
		},
	}

	proj, err := report.Project()
	require.NoError(t, err)

	// dedup happened: 3 objects, 2 distinct descriptors.
	assert.Len(t, proj.Objects, 3)
	assert.Len(t, proj.Descriptors, 2)
	assert.Equal(t, proj.Objects[0].Descriptor, proj.Objects[1].Descriptor, "identical descriptors must share an index")
	assert.NotEqual(t, proj.Objects[0].Descriptor, proj.Objects[2].Descriptor)

	// every object reconstructs to its original descriptor, byte-for-byte.
	for i, po := range proj.Objects {
		orig := report.Objects[i]
		d := proj.Descriptors[po.Descriptor]
		assert.Equal(t, orig.Name, po.Name)
		assert.Equal(t, orig.Owner, d.Owner)
		assert.Equal(t, orig.DescriptorFlags, d.DescriptorFlags)
		assert.Equal(t, orig.DACL, d.DACL)
		assert.Equal(t, orig.SACL, d.SACL)
		assert.Equal(t, orig.AccessEntries, d.AccessEntries)
	}
}

// A parsed report projected must reconstruct the exact same set of (name,
// descriptor) pairs as the flat objects, over real AccessChk text.
func TestProjectRoundTripFromParse(t *testing.T) {
	const sample = `accesschk v6.15
HKLM\SOFTWARE\A
  DESCRIPTOR FLAGS:
      [SE_DACL_PRESENT]
  OWNER: NT AUTHORITY\SYSTEM
  [0] ACCESS_ALLOWED_ACE_TYPE: Everyone
	KEY_ALL_ACCESS
HKLM\SOFTWARE\B
  DESCRIPTOR FLAGS:
      [SE_DACL_PRESENT]
  OWNER: NT AUTHORITY\SYSTEM
  [0] ACCESS_ALLOWED_ACE_TYPE: Everyone
	KEY_ALL_ACCESS
`
	report, err := Parse(strings.NewReader(sample))
	require.NoError(t, err)
	require.Len(t, report.Objects, 2)

	proj, err := report.Project()
	require.NoError(t, err)
	require.Len(t, proj.Objects, 2)
	// A and B share an identical descriptor.
	assert.Len(t, proj.Descriptors, 1)
	assert.Equal(t, proj.Objects[0].Descriptor, proj.Objects[1].Descriptor)

	for i, po := range proj.Objects {
		d := proj.Descriptors[po.Descriptor]
		assert.Equal(t, report.Objects[i].Name, po.Name)
		assert.Equal(t, report.Objects[i].DACL, d.DACL)
		assert.Equal(t, report.Objects[i].Owner, d.Owner)
	}
}

// The serialized projection is the contract the policy engine consumes, so assert
// the marshaled JSON shape directly: the descriptors table, objects that carry a
// "descriptor" index (and no inline DACL), the retained/omitted raw_lines
// fallback, and the top-level raw field.
func TestProjectJSONShape(t *testing.T) {
	report := &Report{
		Tool: Tool{Name: ToolName, Version: "6.15"},
		Raw:  "verbatim text",
		Objects: []Object{
			{Name: "key1", DACL: []ACE{{Index: 0, AceType: "access_allowed_ace_type", Principal: everyone, Rights: []string{keyAllAccess}}}, AccessEntries: []AccessEntry{}, RawLines: []string{"  raw line"}},
			{Name: "key2", DACL: []ACE{{Index: 0, AceType: "access_allowed_ace_type", Principal: everyone, Rights: []string{keyAllAccess}}}, AccessEntries: []AccessEntry{}}, // same descriptor, no raw lines
		},
	}

	proj, err := report.Project()
	require.NoError(t, err)
	b, err := json.Marshal(proj)
	require.NoError(t, err)

	// Decode into a struct mirroring the wire contract policies rely on.
	var wire struct {
		Tool        map[string]any `json:"tool"`
		Descriptors []struct {
			DACL          []map[string]any `json:"dacl"`
			AccessEntries []any            `json:"access_entries"`
		} `json:"descriptors"`
		Objects []struct {
			Name       string   `json:"name"`
			Descriptor *int     `json:"descriptor"`
			RawLines   []string `json:"raw_lines"`
		} `json:"objects"`
		Raw string `json:"raw"`
	}
	require.NoError(t, json.Unmarshal(b, &wire))

	assert.Equal(t, "verbatim text", wire.Raw)
	require.Len(t, wire.Descriptors, 1) // identical descriptors shared
	require.Len(t, wire.Objects, 2)

	require.NotNil(t, wire.Objects[0].Descriptor)
	assert.Equal(t, "key1", wire.Objects[0].Name)
	assert.Equal(t, *wire.Objects[0].Descriptor, *wire.Objects[1].Descriptor)
	assert.Equal(t, everyone, wire.Descriptors[*wire.Objects[0].Descriptor].DACL[0]["principal"])
	assert.NotNil(t, wire.Descriptors[0].AccessEntries)

	// raw_lines is retained for key1 and omitted for key2.
	assert.Equal(t, []string{"  raw line"}, wire.Objects[0].RawLines)
	assert.Nil(t, wire.Objects[1].RawLines)

	// Objects reference a descriptor index and never carry the DACL inline.
	js := string(b)
	assert.Contains(t, js, `"name":"key2","descriptor":0}`)
	assert.NotContains(t, js, `"name":"key2","descriptor":0,"dacl"`)
}
