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

package radamsa_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/radamsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// each of these logs holds two parseable -M records.
var (
	metaA = []byte("seed: 1\nmuta-num: 1, generator: file\n")
	metaB = []byte("seed: 2\nmuta-num: 2, generator: jump\n")
	bad   = []byte("this is not a radamsa metadata log")
)

func TestParseReportBytes(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		wantRecords int
		wantErr     bool
		wantInvalid bool // err should be classified as ErrInvalidReport
	}{
		{name: "single valid log", data: metaA, wantRecords: 2},
		{name: "single malformed log is fatal", data: bad, wantErr: true, wantInvalid: true},
		{name: "single empty log is a valid zero-iteration report", data: []byte("\n \n"), wantRecords: 0},
		{name: "plain gzip single log", data: gzipBytes(t, metaA), wantRecords: 2},
		{name: "plain gzip malformed log is fatal", data: gzipBytes(t, bad), wantErr: true, wantInvalid: true},
		{name: "tar.gz merges records", data: tarGzBytes(t, map[string][]byte{"m_1.log": metaA, "m_2.log": metaB}), wantRecords: 4},
		{name: "zip merges records", data: zipBytes(t, map[string][]byte{"m_1.log": metaA, "m_2.log": metaB}), wantRecords: 4},
		{name: "malformed archive entry is fatal", data: tarGzBytes(t, map[string][]byte{"m_1.log": metaA, "bad.log": bad}), wantErr: true, wantInvalid: true},
		{name: "archive with only empty entries yields empty array", data: zipBytes(t, map[string][]byte{"empty.log": []byte(" \n")}), wantRecords: 0},
		{name: "empty archive yields empty array", data: tarGzBytes(t, nil), wantRecords: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs, err := radamsa.ParseReportBytes(tc.data)
			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantInvalid, errors.Is(err, radamsa.ErrInvalidReport))
				return
			}
			require.NoError(t, err)
			assert.Len(t, recs, tc.wantRecords)
			assert.NotNil(t, recs, "records must be a non-nil slice so it marshals to [] not null")
		})
	}
}

// TestParseReportBytesDeterministicOrder guards that merged records are ordered by
// archive entry name, independent of the order the archive stores them, so
// input.elements is reproducible.
func TestParseReportBytesDeterministicOrder(t *testing.T) {
	// a.log's record carries seed 1, b.log's carries seed 2.
	data := zipBytes(t, map[string][]byte{"b.log": []byte("seed: 2\n"), "a.log": []byte("seed: 1\n")})
	recs, err := radamsa.ParseReportBytes(data)
	require.NoError(t, err)
	require.Len(t, recs, 2)
	assert.EqualValues(t, 1, recs[0]["seed"], "a.log must sort before b.log")
	assert.EqualValues(t, 2, recs[1]["seed"])
}

func TestInspectReport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "meta.txt", metaA)
	writeFile(t, dir, "empty.txt", []byte(" \n"))
	writeFile(t, dir, "invalid.txt", bad)
	writeFile(t, dir, "single.log.gz", gzipBytes(t, metaA))
	writeFile(t, dir, "r.tar.gz", tarGzBytes(t, map[string][]byte{"m_1.log": metaA, "m_2.log": metaB}))
	writeFile(t, dir, "onebad.zip", zipBytes(t, map[string][]byte{"m_1.log": metaA, "bad.log": bad}))

	tests := []struct {
		name        string
		file        string
		wantRecords int
		wantErr     bool
		wantInvalid bool
	}{
		{name: "single valid log", file: "meta.txt", wantRecords: 2},
		{name: "single empty log is valid", file: "empty.txt", wantRecords: 0},
		{name: "single malformed log is fatal", file: "invalid.txt", wantErr: true, wantInvalid: true},
		{name: "plain gzip single log", file: "single.log.gz", wantRecords: 2},
		{name: "tar.gz merges records", file: "r.tar.gz", wantRecords: 4},
		{name: "malformed archive entry is fatal", file: "onebad.zip", wantErr: true, wantInvalid: true},
		{name: "missing file is a file error not an invalid report", file: "nope.txt", wantErr: true, wantInvalid: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs, err := radamsa.InspectReport(filepath.Join(dir, tc.file))
			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantInvalid, errors.Is(err, radamsa.ErrInvalidReport))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRecords, recs)
		})
	}
}

// TestReportArchiveAboveGenericEntryCap guards that the report path accepts an
// archive with more entries than archiveio's generic 10,000-entry default, which
// the "one -M file per run, >10,000 runs" use case exceeds.
func TestReportArchiveAboveGenericEntryCap(t *testing.T) {
	files := make(map[string][]byte, 10001)
	for i := 0; i < 10001; i++ {
		files["m_"+strconv.Itoa(i)+".log"] = []byte("seed: 1\n")
	}
	recs, err := radamsa.ParseReportBytes(zipBytes(t, files))
	require.NoError(t, err)
	assert.Len(t, recs, 10001)
}

func writeFile(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o600))
}

func gzipBytes(t *testing.T, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(content)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func zipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func tarGzBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}
