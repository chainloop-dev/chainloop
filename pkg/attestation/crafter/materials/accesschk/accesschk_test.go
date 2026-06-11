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

package accesschk_test

import (
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/accesschk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Default(t *testing.T) {
	data, err := os.ReadFile("./testdata/default.txt")
	require.NoError(t, err)

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, "AccessChk", report.Tool.Name)
	assert.Equal(t, "6.15", report.Tool.Version)
	assert.Equal(t, string(data), report.Raw)
	assert.True(t, report.LooksLikeAccessChk())

	require.Len(t, report.Objects, 1)
	obj := report.Objects[0]
	assert.Equal(t, `c:\windows\system32\notepad.exe`, obj.Name)

	require.Len(t, obj.AccessEntries, 3)
	assert.Equal(t, "RW", obj.AccessEntries[0].Access)
	assert.Equal(t, `NT AUTHORITY\SYSTEM`, obj.AccessEntries[0].Principal)
	assert.Empty(t, obj.AccessEntries[0].Rights)
	assert.Equal(t, "RW", obj.AccessEntries[1].Access)
	assert.Equal(t, `BUILTIN\Administrators`, obj.AccessEntries[1].Principal)
	assert.Equal(t, "R", obj.AccessEntries[2].Access)
	assert.Equal(t, `BUILTIN\Users`, obj.AccessEntries[2].Principal)

	assert.Equal(t, []string{
		`  RW NT AUTHORITY\SYSTEM`,
		`  RW BUILTIN\Administrators`,
		`  R  BUILTIN\Users`,
	}, obj.RawLines)
}

func TestParse_Verbose(t *testing.T) {
	data, err := os.ReadFile("./testdata/verbose.txt")
	require.NoError(t, err)

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	require.Len(t, report.Objects, 1)
	entries := report.Objects[0].AccessEntries
	require.Len(t, entries, 2)

	assert.Equal(t, "RW", entries[0].Access)
	assert.Equal(t, `NT AUTHORITY\SYSTEM`, entries[0].Principal)
	assert.Equal(t, []string{"FILE_ALL_ACCESS"}, entries[0].Rights)

	assert.Equal(t, "R", entries[1].Access)
	assert.Equal(t, `BUILTIN\Users`, entries[1].Principal)
	assert.Equal(t, []string{"FILE_EXECUTE", "FILE_READ_ATTRIBUTES", "FILE_READ_DATA"}, entries[1].Rights)
}

func TestParse_Service(t *testing.T) {
	data, err := os.ReadFile("./testdata/service.txt")
	require.NoError(t, err)

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	require.Len(t, report.Objects, 1)
	assert.Equal(t, "spooler", report.Objects[0].Name)
	require.Len(t, report.Objects[0].AccessEntries, 2)
	assert.Equal(t, []string{"SERVICE_ALL_ACCESS"}, report.Objects[0].AccessEntries[0].Rights)
	assert.True(t, report.LooksLikeAccessChk())
}

func TestParse_SDDL(t *testing.T) {
	data, err := os.ReadFile("./testdata/sddl.txt")
	require.NoError(t, err)

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	require.Len(t, report.Objects, 1)
	obj := report.Objects[0]
	assert.Equal(t, `c:\windows\system32\notepad.exe`, obj.Name)
	// SDDL/descriptor output is not parsed into structured access entries,
	// but it is preserved verbatim in raw_lines for policy string matching.
	assert.Empty(t, obj.AccessEntries)
	assert.Contains(t, obj.RawLines, "  DESCRIPTOR FLAGS:")
	assert.Contains(t, obj.RawLines, "  OWNER: NT SERVICE\\TrustedInstaller")
	assert.True(t, report.LooksLikeAccessChk())
}

func TestParse_NoBanner(t *testing.T) {
	data, err := os.ReadFile("./testdata/nobanner.txt")
	require.NoError(t, err)

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, "AccessChk", report.Tool.Name)
	assert.Empty(t, report.Tool.Version)
	require.Len(t, report.Objects, 1)
	assert.Equal(t, `c:\windows\system32\notepad.exe`, report.Objects[0].Name)
	require.Len(t, report.Objects[0].AccessEntries, 2)
	assert.True(t, report.LooksLikeAccessChk())
}

func TestParse_DescriptorFormat(t *testing.T) {
	data, err := os.ReadFile("./testdata/descriptor.txt")
	require.NoError(t, err)

	report, err := accesschk.Parse(data)
	require.NoError(t, err)
	assert.True(t, report.LooksLikeAccessChk())

	require.Len(t, report.Objects, 1)
	obj := report.Objects[0]
	assert.Equal(t, "ExampleService", obj.Name)
	assert.Equal(t, `NT AUTHORITY\SYSTEM`, obj.Owner)
	assert.Equal(t, []string{"SE_DACL_PRESENT", "SE_SELF_RELATIVE"}, obj.DescriptorFlags)
	// The -l grammar does not use the compact RW form, so AccessEntries stays empty.
	assert.Empty(t, obj.AccessEntries)

	require.Len(t, obj.DACL, 3)

	assert.Equal(t, 0, obj.DACL[0].Index)
	assert.Equal(t, "ACCESS_ALLOWED_ACE_TYPE", obj.DACL[0].AceType)
	assert.Equal(t, `NT AUTHORITY\SYSTEM`, obj.DACL[0].Principal)
	assert.Empty(t, obj.DACL[0].AceFlags)
	assert.Equal(t, []string{"SERVICE_QUERY_STATUS", "SERVICE_START", "READ_CONTROL"}, obj.DACL[0].Rights)

	assert.Equal(t, `BUILTIN\Administrators`, obj.DACL[1].Principal)
	assert.Equal(t, []string{"SERVICE_ALL_ACCESS"}, obj.DACL[1].Rights)

	assert.Equal(t, 2, obj.DACL[2].Index)
	assert.Equal(t, "ACCESS_DENIED_ACE_TYPE", obj.DACL[2].AceType)
	assert.Equal(t, `NT AUTHORITY\NETWORK`, obj.DACL[2].Principal)
	assert.Equal(t, []string{"INHERITED_ACE"}, obj.DACL[2].AceFlags)
	assert.Equal(t, []string{"SERVICE_STOP"}, obj.DACL[2].Rights)

	require.Len(t, obj.SACL, 1)
	assert.Equal(t, 0, obj.SACL[0].Index)
	assert.Empty(t, obj.SACL[0].AceType)
	assert.Equal(t, "Everyone", obj.SACL[0].Principal)
	assert.Equal(t, []string{"FAILED_ACCESS_ACE_FLAG"}, obj.SACL[0].AceFlags)
	assert.Equal(t, []string{"SERVICE_ALL_ACCESS"}, obj.SACL[0].Rights)
}

func TestParse_Garbage(t *testing.T) {
	data, err := os.ReadFile("./testdata/garbage.txt")
	require.NoError(t, err)

	report, err := accesschk.Parse(data)
	require.NoError(t, err)
	assert.False(t, report.LooksLikeAccessChk())
}

func TestParse_InvalidUTF8(t *testing.T) {
	_, err := accesschk.Parse([]byte{0xff, 0xfe, 0x00, 0x01})
	assert.Error(t, err)
}

func TestParse_Empty(t *testing.T) {
	report, err := accesschk.Parse([]byte{})
	require.NoError(t, err)
	assert.False(t, report.LooksLikeAccessChk())
	assert.Empty(t, report.Objects)
}
