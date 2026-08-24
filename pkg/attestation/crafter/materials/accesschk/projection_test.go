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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Project must be lossless: every object's descriptor is exactly reconstructable
// from the descriptors table, and identical descriptors are shared.
func TestProjectIsLossless(t *testing.T) {
	const sys = "SYSTEM"
	report := &Report{
		Tool: Tool{Name: ToolName, Version: "6.15"},
		Objects: []Object{
			{Name: "key1", Owner: sys, DACL: []ACE{{Index: 0, Principal: "Everyone", Rights: []string{"KEY_ALL_ACCESS"}}}, AccessEntries: []AccessEntry{}},
			{Name: "key2", Owner: sys, DACL: []ACE{{Index: 0, Principal: "Everyone", Rights: []string{"KEY_ALL_ACCESS"}}}, AccessEntries: []AccessEntry{}}, // same descriptor as key1
			{Name: "key3", Owner: "Admins", DACL: []ACE{{Index: 0, Principal: sys, Rights: []string{"KEY_READ"}}}, AccessEntries: []AccessEntry{}},         // distinct
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
