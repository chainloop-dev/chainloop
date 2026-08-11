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
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/accesschk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildLargeInput returns a valid AccessChk default-mode document whose size
// exceeds targetSize bytes, made of repeated per-object blocks.
func buildLargeInput(targetSize int) []byte {
	var b strings.Builder
	b.WriteString("Accesschk v6.15 - Reports effective permissions for securable objects\n")
	b.WriteString("Copyright (C) 2006-2023 Mark Russinovich\n")
	b.WriteString("Sysinternals - www.sysinternals.com\n\n")
	i := 0
	for b.Len() <= targetSize {
		b.WriteString("c:\\windows\\system32\\object")
		b.WriteString(strings.Repeat("x", 8))
		b.WriteString("\n")
		b.WriteString("  RW NT AUTHORITY\\SYSTEM\n")
		b.WriteString("  RW BUILTIN\\Administrators\n")
		b.WriteString("  R  BUILTIN\\Users\n")
		i++
	}
	return []byte(b.String())
}

// TestParse_LargeInputOmitsRawFields verifies that above RawRetentionLimit the
// verbatim fallback fields (Raw / RawLines) are omitted while the structured
// projection is still produced. This is the memory guard: those fields are not
// attested and unused by policies, so trimming them for oversized inputs keeps
// the OPA input document from ballooning.
func TestParse_LargeInputOmitsRawFields(t *testing.T) {
	data := buildLargeInput(accesschk.RawRetentionLimit + 1)
	require.Greater(t, len(data), accesschk.RawRetentionLimit)

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	assert.Empty(t, report.Raw, "Raw must be omitted above the retention limit")
	require.NotEmpty(t, report.Objects)
	for _, obj := range report.Objects {
		assert.Empty(t, obj.RawLines, "RawLines must be omitted above the retention limit")
	}

	// Structured parsing is unaffected: access entries are still populated.
	assert.Equal(t, "6.15", report.Tool.Version)
	assert.Len(t, report.Objects[0].AccessEntries, 3)
	assert.True(t, report.LooksLikeAccessChk())
}

// TestParse_SmallInputRetainsRawFields verifies that below the limit the
// verbatim fields are retained, preserving the string-matching fallback for
// normal-sized evidence.
func TestParse_SmallInputRetainsRawFields(t *testing.T) {
	data := []byte("c:\\file\n  RW BUILTIN\\Administrators\n")

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, string(data), report.Raw)
	require.Len(t, report.Objects, 1)
	assert.Equal(t, []string{"  RW BUILTIN\\Administrators"}, report.Objects[0].RawLines)
}

// TestParse_LargeSDDLStillDetected verifies that an oversized descriptor-only
// document (no compact access entries) is still recognized as AccessChk output
// even though Raw is omitted, i.e. detection does not depend on Raw.
func TestParse_LargeSDDLStillDetected(t *testing.T) {
	var b strings.Builder
	b.WriteString("Accesschk v6.15\n\n")
	for b.Len() <= accesschk.RawRetentionLimit {
		b.WriteString("c:\\windows\\system32\\object\n")
		b.WriteString("  DESCRIPTOR FLAGS:\n")
		b.WriteString("  [SE_DACL_PRESENT]\n")
		b.WriteString("  OWNER: NT SERVICE\\TrustedInstaller\n")
	}
	data := []byte(b.String())
	require.Greater(t, len(data), accesschk.RawRetentionLimit)

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	assert.Empty(t, report.Raw)
	assert.True(t, report.LooksLikeAccessChk(), "descriptor marker detection must not depend on Raw")
}

// TestParse_CRLFLineEndings verifies the streaming line reader strips a
// trailing CR from CRLF-terminated lines, matching the LF behavior.
func TestParse_CRLFLineEndings(t *testing.T) {
	data := []byte("c:\\file\r\n  RW Everyone\r\n")

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	require.Len(t, report.Objects, 1)
	assert.Equal(t, "c:\\file", report.Objects[0].Name)
	require.Len(t, report.Objects[0].AccessEntries, 1)
	assert.Equal(t, "Everyone", report.Objects[0].AccessEntries[0].Principal)
	// The trailing CR from the CRLF terminator is stripped, matching LF handling.
	assert.Equal(t, []string{"  RW Everyone"}, report.Objects[0].RawLines)
}

// TestParse_VeryLongLine verifies the streaming reader handles a single line
// far larger than a typical scanner token buffer without erroring.
func TestParse_VeryLongLine(t *testing.T) {
	principal := strings.Repeat("A", 128*1024)
	data := []byte("c:\\file\n  RW " + principal + "\n")

	report, err := accesschk.Parse(data)
	require.NoError(t, err)

	require.Len(t, report.Objects, 1)
	require.Len(t, report.Objects[0].AccessEntries, 1)
	assert.Equal(t, principal, report.Objects[0].AccessEntries[0].Principal)
}
