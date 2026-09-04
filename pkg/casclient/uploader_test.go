//
// Copyright 2023-2026 The Chainloop Authors.
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

package casclient

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeResource(t *testing.T) {
	const validDigestHex = "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"

	testCases := []struct {
		name      string
		fileName  string
		digest    string
		size      int64
		want      *v1.CASResource
		wantError bool
	}{
		{
			name:      "empty filename",
			digest:    "deadbeef",
			wantError: true,
		},
		{
			name:      "empty digest",
			fileName:  "foo.txt",
			wantError: true,
		},
		{
			name:      "uncompleted digest",
			digest:    "deadbeef",
			fileName:  "foo.txt",
			wantError: true,
		},
		{
			name:      "invalid digest",
			digest:    "sha256:deadbeef",
			fileName:  "foo.txt",
			wantError: true,
		},
		{
			name:     "valid",
			digest:   fmt.Sprintf("sha256:%s", validDigestHex),
			fileName: "foo.txt",
			want:     &v1.CASResource{FileName: "foo.txt", Digest: validDigestHex},
		},
		{
			name:     "valid with size",
			digest:   fmt.Sprintf("sha256:%s", validDigestHex),
			fileName: "foo.txt",
			size:     1234,
			want:     &v1.CASResource{FileName: "foo.txt", Digest: validDigestHex, Size: 1234},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotEncoded, err := encodeResource(tc.fileName, tc.digest, tc.size)
			if tc.wantError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			// Decode the returned value to make sure it's a cas resource

			raw, err := base64.StdEncoding.DecodeString(gotEncoded)
			assert.NoError(t, err)

			got := &v1.CASResource{}
			err = gob.NewDecoder(bytes.NewReader(raw)).Decode(got)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestReaderLen(t *testing.T) {
	const content = "the quick brown fox"

	t.Run("bytes.Reader", func(t *testing.T) {
		n, ok := readerLen(bytes.NewReader([]byte(content)))
		assert.True(t, ok)
		assert.Equal(t, int64(len(content)), n)
	})

	t.Run("bytes.Buffer", func(t *testing.T) {
		n, ok := readerLen(bytes.NewBufferString(content))
		assert.True(t, ok)
		assert.Equal(t, int64(len(content)), n)
	})

	t.Run("strings.Reader", func(t *testing.T) {
		n, ok := readerLen(strings.NewReader(content))
		assert.True(t, ok)
		assert.Equal(t, int64(len(content)), n)
	})

	t.Run("os.File reports remaining bytes", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "readerlen")
		require.NoError(t, err)
		_, err = f.WriteString(content)
		require.NoError(t, err)
		_, err = f.Seek(0, io.SeekStart)
		require.NoError(t, err)

		n, ok := readerLen(f)
		assert.True(t, ok)
		assert.Equal(t, int64(len(content)), n)

		// After consuming some bytes, the remaining size is reported.
		buf := make([]byte, 4)
		_, err = io.ReadFull(f, buf)
		require.NoError(t, err)
		n, ok = readerLen(f)
		assert.True(t, ok)
		assert.Equal(t, int64(len(content)-4), n)
	})

	t.Run("unknown reader is undetectable", func(t *testing.T) {
		_, ok := readerLen(io.NopCloser(strings.NewReader(content)))
		assert.False(t, ok)
	})
}
