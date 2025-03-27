//
// Copyright 2023-2025 The Chainloop Authors.
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
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/stretchr/testify/assert"
)

func TestEncodeResource(t *testing.T) {
	const validDigestHex = "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"

	testCases := []struct {
		name      string
		fileName  string
		digest    string
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotEncoded, err := encodeResource(tc.fileName, tc.digest)
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
