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
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
)

// An MSBuild binary log (-bl) is a gzip stream: a header of two little-endian
// int32 (the file format version and the minimum reader version), then a
// sequence of records, each a 7-bit encoded kind followed by a 7-bit encoded
// length and the record body. Strings are written once as String records and
// referenced by index afterwards.
//
// The layout is MSBuild's BuildEventArgsWriter/BuildEventArgsReader. Only the
// records that carry diagnostics are decoded; every other record is skipped by
// its length, which is why logs older than the length-prefixed format are not
// supported.
const (
	recordEndOfFile      = 0
	recordBuildFinished  = 2
	recordProjectStarted = 3
	recordError          = 9
	recordWarning        = 10
	recordString         = 24

	// minBinlogVersion is the first format version whose records are all
	// length-prefixed (MSBuild's ForwardCompatibilityMinimalVersion).
	minBinlogVersion = 18
	// knownBinlogVersion is the newest format version whose diagnostic record
	// layout this reader was written against. Newer versions may append fields
	// to a record, which are skipped; older ones must match it exactly.
	knownBinlogVersion = 25

	// stringStartIndex is the index MSBuild assigns to the first String record;
	// 0 is null, 1 is the empty string and the rest are reserved.
	stringStartIndex = 10
)

// BuildEventArgsFieldFlags: which optional fields a record carries.
const (
	flagBuildEventContext = 1 << 0
	flagHelpKeyword       = 1 << 1
	flagMessage           = 1 << 2
	flagSenderName        = 1 << 3
	flagThreadID          = 1 << 4
	flagTimestamp         = 1 << 5
	flagSubcategory       = 1 << 6
	flagCode              = 1 << 7
	flagFile              = 1 << 8
	flagProjectFile       = 1 << 9
	flagLineNumber        = 1 << 10
	flagColumnNumber      = 1 << 11
	flagEndLineNumber     = 1 << 12
	flagEndColumnNumber   = 1 << 13
	flagArguments         = 1 << 14
	flagImportance        = 1 << 15
	flagExtended          = 1 << 16
)

// ErrUnsupportedBinlog is returned for a binary log whose format version this
// reader cannot decode.
var ErrUnsupportedBinlog = errors.New("unsupported MSBuild binary log version")

// maxBinlogRecord bounds a single record body. Diagnostic records are a few
// hundred bytes; the bound only stops a corrupt length from allocating
// gigabytes.
const maxBinlogRecord = 64 << 20

// formatArgRe matches a .NET composite-format placeholder such as {0} or {1,-8:N2}.
var formatArgRe = regexp.MustCompile(`\{(\d+)(?:,[^}:]*)?(?::[^}]*)?\}`)

// isBinlog reports whether peeked bytes are the start of an MSBuild binary log:
// a gzip stream whose first decompressed int32 is a plausible format version.
// A tar.gz, the other gzip content a material may hold, starts with an entry
// name instead.
func isBinlog(peek []byte) bool {
	if !bytes.HasPrefix(peek, []byte{0x1f, 0x8b}) {
		return false
	}
	zr, err := gzip.NewReader(bytes.NewReader(peek))
	if err != nil {
		return false
	}
	var version int32
	if err := binary.Read(zr, binary.LittleEndian, &version); err != nil {
		return false
	}
	return version > 0 && version < 1<<16
}

type binlogReader struct {
	r       *bufio.Reader
	version int32
	strings []string
}

// parseBinlog reads the diagnostics of an MSBuild binary log.
func parseBinlog(r io.Reader) (*parsedLog, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("opening binary log: %w", err)
	}
	defer zr.Close()

	br := &binlogReader{r: bufio.NewReader(zr)}
	var header [2]int32
	if err := binary.Read(br.r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("reading binary log header: %w", err)
	}
	br.version = header[0]
	if br.version < minBinlogVersion {
		return nil, fmt.Errorf("%w %d: produce the log with a newer MSBuild (format %d or later)", ErrUnsupportedBinlog, br.version, minBinlogVersion)
	}
	if header[1] > knownBinlogVersion {
		return nil, fmt.Errorf("%w %d: it requires a reader for format %d or later", ErrUnsupportedBinlog, br.version, header[1])
	}

	log := &parsedLog{format: FormatMSBuildBinlog, tool: Tool{Name: ToolMSBuild}}
	for n := 0; ; n++ {
		kind, err := br.varint(br.r)
		if err != nil {
			return nil, fmt.Errorf("binary log is truncated: reading record %d: %w", n, err)
		}
		if kind == recordEndOfFile {
			return log, nil
		}
		length, err := br.varint(br.r)
		if err != nil {
			return nil, fmt.Errorf("binary log is truncated: reading record %d: %w", n, err)
		}
		if length < 0 {
			return nil, fmt.Errorf("binary log record %d has an invalid length %d", n, length)
		}
		// Only records read into memory are bounded; the rest are skipped
		// whatever their size (an embedded project-imports archive can be large).
		if kind != recordString && kind != recordWarning && kind != recordError &&
			kind != recordProjectStarted && kind != recordBuildFinished {
			if _, err := br.r.Discard(length); err != nil {
				return nil, fmt.Errorf("binary log is truncated: skipping record %d: %w", n, err)
			}
			continue
		}
		if length > maxBinlogRecord {
			return nil, fmt.Errorf("binary log record %d has an invalid length %d", n, length)
		}

		switch kind {
		case recordBuildFinished:
			body, err := br.body(length)
			if err != nil {
				return nil, fmt.Errorf("binary log is truncated: reading record %d: %w", n, err)
			}
			if log.result, err = br.buildResult(body); err != nil {
				return nil, fmt.Errorf("decoding binary log record %d: %w", n, err)
			}
		case recordProjectStarted:
			body, err := br.body(length)
			if err != nil {
				return nil, fmt.Errorf("binary log is truncated: reading record %d: %w", n, err)
			}
			if log.solution == "" {
				// The first project started is the one the build was run for.
				// Only a label, so a record this reader cannot decode is no error.
				log.solution = br.projectFile(body)
			}
		case recordString:
			// A String record's length is the string's own length prefix.
			body, err := br.body(length)
			if err != nil {
				return nil, fmt.Errorf("binary log is truncated: reading record %d: %w", n, err)
			}
			br.strings = append(br.strings, string(body))
		case recordWarning, recordError:
			body, err := br.body(length)
			if err != nil {
				return nil, fmt.Errorf("binary log is truncated: reading record %d: %w", n, err)
			}
			d, err := br.diagnostic(body)
			if err != nil {
				return nil, fmt.Errorf("decoding binary log record %d: %w", n, err)
			}
			if kind == recordError {
				d.Severity = SeverityError
			} else {
				d.Severity = SeverityWarning
			}
			log.diagnostics = append(log.diagnostics, d)
		}
	}
}

func (br *binlogReader) body(length int) ([]byte, error) {
	body := make([]byte, length)
	if _, err := io.ReadFull(br.r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// diagnostic decodes a Warning or Error record body: the shared event fields
// (driven by the flags), then the diagnostic fields that are always present.
func (br *binlogReader) diagnostic(body []byte) (Diagnostic, error) {
	r := bytes.NewReader(body)
	message, arguments, err := br.eventFields(r)
	if err != nil {
		return Diagnostic{}, err
	}
	d, err := br.diagnosticFields(r)
	if err != nil {
		return Diagnostic{}, err
	}

	// Unread bytes mean the record does not have the layout this reader
	// expects. That is fatal for the versions it knows, where it would mean a
	// misread; newer versions may append fields, which are safe to skip.
	if r.Len() != 0 && br.version <= knownBinlogVersion {
		return Diagnostic{}, fmt.Errorf("record has %d unread bytes", r.Len())
	}

	d.Message = formatMessage(message, arguments)
	return d, nil
}

// eventFields reads the optional fields every build event may carry, in the
// order of BuildEventArgsReader.ReadBuildEventArgsFields, and returns the
// message with its format arguments.
func (br *binlogReader) eventFields(r *bytes.Reader) (string, []string, error) {
	flags, err := br.varint(r)
	if err != nil {
		return "", nil, err
	}

	var message string
	var arguments []string
	skipVarint := func() error { return br.skipVarints(r, 1) }
	skipString := func() error { _, err := br.str(r); return err }

	fields := []struct {
		flag int
		read func() error
	}{
		{flagMessage, func() (err error) { message, err = br.str(r); return err }},
		// nodeId, projectContextId, targetId, taskId, submissionId,
		// projectInstanceId, evaluationId
		{flagBuildEventContext, func() error { return br.skipVarints(r, 7) }},
		{flagThreadID, skipVarint},
		{flagHelpKeyword, skipString},
		{flagSenderName, skipString},
		// ticks, then the DateTimeKind
		{flagTimestamp, func() error {
			if _, err := r.Seek(8, io.SeekCurrent); err != nil {
				return err
			}
			return skipVarint()
		}},
		// type, metadata (an index into a name-value list), data
		{flagExtended, func() error {
			if err := skipString(); err != nil {
				return err
			}
			if err := skipVarint(); err != nil {
				return err
			}
			return skipString()
		}},
		// Diagnostic events never set these, but the reader honors them.
		{flagSubcategory, skipString},
		{flagCode, skipString},
		{flagFile, skipString},
		{flagProjectFile, skipString},
		{flagLineNumber, skipVarint},
		{flagColumnNumber, skipVarint},
		{flagEndLineNumber, skipVarint},
		{flagEndColumnNumber, skipVarint},
		{flagArguments, func() (err error) { arguments, err = br.arguments(r); return err }},
		{flagImportance, skipVarint},
	}
	for _, f := range fields {
		if flags&f.flag == 0 {
			continue
		}
		if err := f.read(); err != nil {
			return "", nil, err
		}
	}
	return message, arguments, nil
}

func (br *binlogReader) arguments(r *bytes.Reader) ([]string, error) {
	count, err := br.varint(r)
	if err != nil {
		return nil, err
	}
	if count < 0 || count > r.Len() {
		return nil, fmt.Errorf("invalid argument count %d", count)
	}
	arguments := make([]string, 0, count)
	for range count {
		a, err := br.str(r)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, a)
	}
	return arguments, nil
}

// diagnosticFields reads what BuildEventArgsReader.ReadDiagnosticFields does:
// subcategory, code, file, project file, line, column, end line, end column.
func (br *binlogReader) diagnosticFields(r *bytes.Reader) (Diagnostic, error) {
	var d Diagnostic
	var err error
	if _, err = br.str(r); err != nil {
		return d, err
	}
	if d.Code, err = br.str(r); err != nil {
		return d, err
	}
	if d.File, err = br.str(r); err != nil {
		return d, err
	}
	if d.Project, err = br.str(r); err != nil {
		return d, err
	}
	if d.Line, err = br.varint(r); err != nil {
		return d, err
	}
	if d.Column, err = br.varint(r); err != nil {
		return d, err
	}
	return d, br.skipVarints(r, 2)
}

// buildResult reads a BuildFinished record body: the event fields, then
// whether the build succeeded.
func (br *binlogReader) buildResult(body []byte) (string, error) {
	r := bytes.NewReader(body)
	if _, _, err := br.eventFields(r); err != nil {
		return "", err
	}
	succeeded, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	if succeeded != 0 {
		return ResultSucceeded, nil
	}
	return ResultFailed, nil
}

// projectFile returns the project file of a ProjectStarted record body: the
// event fields, an optional parent build context, then the project file. It
// returns the empty string when the body cannot be decoded.
func (br *binlogReader) projectFile(body []byte) string {
	r := bytes.NewReader(body)
	if _, _, err := br.eventFields(r); err != nil {
		return ""
	}
	hasParent, err := r.ReadByte()
	if err != nil {
		return ""
	}
	if hasParent != 0 {
		if err := br.skipVarints(r, 7); err != nil {
			return ""
		}
	}
	project, err := br.str(r)
	if err != nil {
		return ""
	}
	return project
}

// formatMessage expands a .NET composite format string with its arguments,
// which is how MSBuild stores lazily formatted task messages.
func formatMessage(format string, arguments []string) string {
	if len(arguments) == 0 {
		return format
	}
	return formatArgRe.ReplaceAllStringFunc(format, func(m string) string {
		i, err := strconv.Atoi(formatArgRe.FindStringSubmatch(m)[1])
		if err != nil || i >= len(arguments) {
			return m
		}
		return arguments[i]
	})
}

// str reads a deduplicated string reference.
func (br *binlogReader) str(r io.ByteReader) (string, error) {
	i, err := br.varint(r)
	if err != nil {
		return "", err
	}
	switch {
	case i == 0, i == 1:
		return "", nil
	case i < stringStartIndex || i-stringStartIndex >= len(br.strings):
		return "", fmt.Errorf("invalid string reference %d", i)
	}
	return br.strings[i-stringStartIndex], nil
}

func (br *binlogReader) skipVarints(r io.ByteReader, n int) error {
	for range n {
		if _, err := br.varint(r); err != nil {
			return err
		}
	}
	return nil
}

// varint reads a .NET 7-bit encoded int32.
func (br *binlogReader) varint(r io.ByteReader) (int, error) {
	var v uint32
	for shift := 0; shift < 35; shift += 7 {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && shift > 0 {
				err = io.ErrUnexpectedEOF
			}
			return 0, err
		}
		v |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return int(int32(v)), nil
		}
	}
	return 0, errors.New("invalid 7-bit encoded integer")
}
