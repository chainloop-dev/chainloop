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

package buildlog

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// binlogWriter builds minimal binary logs for the record shapes the synthetic
// fixtures do not contain, following MSBuild's BuildEventArgsWriter layout.
type binlogWriter struct {
	out     bytes.Buffer
	strings map[string]int
}

func newBinlogWriter(version, minReaderVersion int32) *binlogWriter {
	w := &binlogWriter{strings: map[string]int{}}
	_ = binary.Write(&w.out, binary.LittleEndian, version)
	_ = binary.Write(&w.out, binary.LittleEndian, minReaderVersion)
	return w
}

func putVarint(buf *bytes.Buffer, v int) {
	u := uint32(int32(v))
	for u >= 0x80 {
		buf.WriteByte(byte(u) | 0x80)
		u >>= 7
	}
	buf.WriteByte(byte(u))
}

// str returns the deduplicated index of s, writing its String record first
// when it is new. Index 0 is null, as in MSBuild.
func (w *binlogWriter) str(s *string) int {
	if s == nil {
		return 0
	}
	if i, ok := w.strings[*s]; ok {
		return i
	}
	putVarint(&w.out, recordString)
	putVarint(&w.out, len(*s))
	w.out.WriteString(*s)
	i := len(w.strings) + stringStartIndex
	w.strings[*s] = i
	return i
}

type diagnosticRecord struct {
	kind         int
	message      string
	arguments    []string
	extendedType string
	code         string
	file         string
	project      string
	line, column int
}

func ptr(s string) *string { return &s }

const (
	codeVariadicClass = "C4840"
	codeUnresolvedRef = "MSB3245"
	sourceFile        = `C:\src\a.cpp`
	sourceProject     = `C:\src\a.vcxproj`
)

func (w *binlogWriter) diagnostic(d diagnosticRecord) {
	// String records go out first, as the writer flushes them before the
	// record that references them.
	msg := w.str(ptr(d.message))
	sender := w.str(ptr("Task"))
	var extType, extData int
	if d.extendedType != "" {
		extType = w.str(ptr(d.extendedType))
		extData = w.str(ptr(`{"ruleId":"` + d.code + `"}`))
	}
	args := make([]int, 0, len(d.arguments))
	for _, a := range d.arguments {
		args = append(args, w.str(ptr(a)))
	}
	code, file, project := w.str(ptr(d.code)), w.str(ptr(d.file)), w.str(ptr(d.project))

	var body bytes.Buffer
	flags := flagMessage | flagBuildEventContext | flagSenderName | flagTimestamp
	if d.extendedType != "" {
		flags |= flagExtended
	}
	if len(args) > 0 {
		flags |= flagArguments
	}
	putVarint(&body, flags)
	putVarint(&body, msg)
	for _, v := range []int{1, 2, 3, 4, 5, 6, 7} {
		putVarint(&body, v)
	}
	putVarint(&body, sender)
	_ = binary.Write(&body, binary.LittleEndian, int64(638000000000000000))
	putVarint(&body, 1)
	if d.extendedType != "" {
		putVarint(&body, extType)
		putVarint(&body, 0) // no metadata
		putVarint(&body, extData)
	}
	if len(args) > 0 {
		putVarint(&body, len(args))
		for _, a := range args {
			putVarint(&body, a)
		}
	}
	putVarint(&body, 0) // subcategory
	putVarint(&body, code)
	putVarint(&body, file)
	putVarint(&body, project)
	putVarint(&body, d.line)
	putVarint(&body, d.column)
	putVarint(&body, 0)
	putVarint(&body, 0)

	putVarint(&w.out, d.kind)
	putVarint(&w.out, body.Len())
	w.out.Write(body.Bytes())
}

func (w *binlogWriter) bytes(t *testing.T) []byte {
	t.Helper()
	putVarint(&w.out, recordEndOfFile)
	return gzipOf(t, w.out.Bytes())
}

func gzipOf(t *testing.T, raw []byte) []byte {
	t.Helper()
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, err := zw.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return gz.Bytes()
}

func TestParseBinlogRecordShapes(t *testing.T) {
	w := newBinlogWriter(25, 18)
	// MSVC reports its diagnostics as extended events carrying a SARIF result.
	w.diagnostic(diagnosticRecord{
		kind: recordWarning, message: "non-portable use of class as an argument to a variadic function",
		extendedType: "SarifBuildEvent", code: codeVariadicClass, file: sourceFile, project: sourceProject, line: 10, column: 4,
	})
	// Task warnings may keep their message as a format string plus arguments.
	w.diagnostic(diagnosticRecord{
		kind: recordWarning, message: "Reference {0} could not be resolved: {1}", arguments: []string{"Foo", "not found"},
		code: codeUnresolvedRef, file: `C:\sdk\Microsoft.Common.targets`, project: `C:\src\b.csproj`, line: 2400, column: 5,
	})
	w.diagnostic(diagnosticRecord{
		kind: recordError, message: msgUndeclared,
		code: codeUndeclared, file: sourceFile, project: sourceProject, line: 41, column: 7,
	})

	report, err := Parse(w.bytes(t))
	require.NoError(t, err)

	assert.Equal(t, Summary{
		Warnings: 2,
		Errors:   1,
		ByCode:   map[string]int{codeUndeclared: 1, codeVariadicClass: 1, codeUnresolvedRef: 1},
	}, report.Summary)
	assert.Equal(t, inLogs([]Diagnostic{
		{Severity: SeverityError, Code: codeUndeclared, Message: msgUndeclared, File: sourceFile, Line: 41, Column: 7, Project: sourceProject, Occurrences: 1},
		{Severity: SeverityWarning, Code: codeVariadicClass, Message: "non-portable use of class as an argument to a variadic function", File: sourceFile, Line: 10, Column: 4, Project: sourceProject, Occurrences: 1},
		{Severity: SeverityWarning, Code: codeUnresolvedRef, Message: "Reference Foo could not be resolved: not found", File: `C:\sdk\Microsoft.Common.targets`, Line: 2400, Column: 5, Project: `C:\src\b.csproj`, Occurrences: 1},
	}, 0), report.Diagnostics)
}

func TestParseBinlogUnsupportedVersions(t *testing.T) {
	testCases := []struct {
		name             string
		version, minRead int32
	}{
		{name: "older than length-prefixed records", version: 17, minRead: 17},
		{name: "requires a newer reader", version: 40, minRead: 40},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(newBinlogWriter(tc.version, tc.minRead).bytes(t))
			assert.ErrorIs(t, err, ErrUnsupportedBinlog)
		})
	}
}

func TestParseBinlogTruncatedFails(t *testing.T) {
	w := newBinlogWriter(25, 18)
	w.diagnostic(diagnosticRecord{kind: recordWarning, message: "m", code: codeUnusedLocal, file: "a.cs", project: csharpProject, line: 1, column: 1})
	complete := w.out.Bytes()

	// A log cut short, whether inside a record or before the end-of-file
	// record, is incomplete evidence: the parse must fail rather than report
	// only the warnings it got to.
	for name, raw := range map[string][]byte{
		"inside a record":        complete[:len(complete)-3],
		"missing end-of-file":    complete,
		"inside the file header": complete[:5],
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(gzipOf(t, raw))
			assert.Error(t, err)
			assert.NotErrorIs(t, err, ErrNoBuildLog)
		})
	}
}

func TestParseBinlogMalformedRecordFails(t *testing.T) {
	w := newBinlogWriter(25, 18)
	// A record whose declared length leaves fields unread means the layout is
	// not the one this reader knows.
	putVarint(&w.out, recordWarning)
	putVarint(&w.out, 40)
	w.out.Write(make([]byte, 40))

	_, err := Parse(w.bytes(t))
	assert.Error(t, err)
}
