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

package e2e

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImages(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{name: "pasted image", data: pastedPNG()},
		{name: "media image", data: mediaPNG()},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := png.Decode(bytes.NewReader(tc.data))
			require.NoError(t, err, "the image must stay a valid PNG")
		})
	}

	t.Run("media image base64 holds the secret as a whole word", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString(mediaPNG())
		assert.Contains(t, encoded, "/"+mediaSecret+"/")
	})
}
