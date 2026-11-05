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

package fileca

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/errorsx"
	"github.com/chainloop-dev/chainloop/pkg/errorsx/errorsxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewErrorsRecordOriginStack(t *testing.T) {
	invalidPEM := filepath.Join(t.TempDir(), "invalid.pem")
	require.NoError(t, os.WriteFile(invalidPEM, []byte("not a pem"), 0o600))

	testCases := []struct {
		name     string
		certPath string
		wantErr  string
	}{
		{name: "missing certificate", certPath: filepath.Join(t.TempDir(), "missing.pem"), wantErr: "fileca: reading certificate"},
		{name: "invalid certificate", certPath: invalidPEM, wantErr: "fileca: loading certificate"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.certPath, "key.pem", "", false)
			require.ErrorContains(t, err, tc.wantErr)
			assert.True(t, errorsx.HasStack(err))
			assert.True(t, strings.HasSuffix(errorsxtest.ErrorOriginFunction(err), "loadKeyPair"))
		})
	}
}
