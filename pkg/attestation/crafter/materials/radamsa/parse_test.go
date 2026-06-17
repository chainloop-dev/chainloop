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
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/radamsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	const sample = `seed: 705693910129640559698481
muta-num: 1, generator: file, checksum: "CF5DA754A292766FAA5465FD", nth: 1, path: "/tmp/m/sample_1.eds", output: file-writer, length: 16892, pattern: many-dec
byte-dec: 1, generator: jump, head: "/tmp/t/sample.eds", checksum: "F2F767F4D2E28596BD5BD982", nth: 2, output: file-writer, length: 17199, pattern: many-dec
`
	records, err := radamsa.Parse(strings.NewReader(sample))
	require.NoError(t, err)
	require.Len(t, records, 3)

	// lone seed line: too large for int64, kept as string
	assert.Equal(t, "705693910129640559698481", records[0]["seed"])

	// quoted value unquoted; bare int typed as int64; identifier kept as string
	assert.Equal(t, "CF5DA754A292766FAA5465FD", records[1]["checksum"])
	assert.EqualValues(t, 1, records[1]["nth"])
	assert.Equal(t, "file", records[1]["generator"])
	assert.Equal(t, "/tmp/m/sample_1.eds", records[1]["path"])

	// heterogeneous keys
	assert.Equal(t, "/tmp/t/sample.eds", records[2]["head"])
	_, hasSource := records[2]["source"]
	assert.False(t, hasSource)
}

func TestParse_QuotedCommaNotSplit(t *testing.T) {
	records, err := radamsa.Parse(strings.NewReader(`nth: 1, note: "a, b, c", length: 5`))
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "a, b, c", records[0]["note"])
	assert.EqualValues(t, 5, records[0]["length"])
}

func TestParse_EscapedQuoteInValue(t *testing.T) {
	// radamsa writes string values with escaped embedded quotes (\"); the comma
	// split must not treat the escaped quote as the end of the quoted span.
	records, err := radamsa.Parse(strings.NewReader(`path: "a\"b, c", nth: 1`))
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, `a"b, c`, records[0]["path"])
	assert.EqualValues(t, 1, records[0]["nth"])
}

func TestParse_Empty(t *testing.T) {
	_, err := radamsa.Parse(strings.NewReader("   \n\n"))
	assert.Error(t, err)
}

func TestParse_Garbage(t *testing.T) {
	_, err := radamsa.Parse(strings.NewReader("this is not a meta log"))
	assert.Error(t, err)
}
