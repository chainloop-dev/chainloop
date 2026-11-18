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

package data_test

import (
	"context"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const contractNullSpecMigration = "ent/migrate/migrations/20261117162118.sql"

// Since protoyaml v0.8.0, a YAML contract with a null top-level spec fails validation on read.
// The migration rewrites the stored value to `spec: {}`, which is what the text meant before.
func TestContractNullSpecMigration(t *testing.T) {
	if !testhelpers.IntegrationTestsEnabled() {
		t.Skip()
	}

	tu := testhelpers.NewTestingUseCases(t)
	defer tu.DB.Close(t)
	ctx := context.Background()

	const header = "apiVersion: chainloop.dev/v1\nkind: Contract\nmetadata:\n  name: c\n"
	testCases := []struct {
		name   string
		format unmarshal.RawFormat
		body   string
		// want is the body after the migration. Empty means no change.
		want string
	}{
		{
			name: "comment line and no newline at the end",
			body: "# This is the default empty contract\n" + header + "spec:",
			want: "# This is the default empty contract\n" + header + "spec: {}",
		},
		{
			name: "empty contract of the docs example",
			body: header + "spec:\n",
			want: header + "spec: {}\n",
		},
		{name: "explicit null", body: header + "spec: null\n", want: header + "spec: {}\n"},
		{name: "tilde with a comment", body: header + "spec: ~ # nothing\n", want: header + "spec: {} # nothing\n"},
		{name: "comment only", body: header + "spec: # nothing\n", want: header + "spec: {} # nothing\n"},
		{name: "trailing spaces", body: header + "spec:   \n", want: header + "spec: {}\n"},
		{
			name: "CRLF line endings",
			body: "apiVersion: chainloop.dev/v1\r\nkind: Contract\r\nmetadata:\r\n  name: c\r\nspec:\r\n",
			want: "apiVersion: chainloop.dev/v1\r\nkind: Contract\r\nmetadata:\r\n  name: c\r\nspec: {}\r\n",
		},
		{name: "top-level comment after it", body: header + "spec:\n\n# end\n", want: header + "spec: {}\n\n# end\n"},
		{
			name: "indented comment and nothing else after it",
			body: header + "spec:\n  # no materials yet\n",
			want: header + "spec: {}\n  # no materials yet\n",
		},
		{
			name: "before another top-level key",
			body: "apiVersion: chainloop.dev/v1\nkind: Contract\nspec:\nmetadata:\n  name: c\n",
			want: "apiVersion: chainloop.dev/v1\nkind: Contract\nspec: {}\nmetadata:\n  name: c\n",
		},
		{name: "spec with fields", body: header + "spec:\n  materials:\n    - name: a\n      type: ARTIFACT\n"},
		{name: "spec with a comment before its fields", body: header + "spec:\n  # materials\n  materials: []\n"},
		{name: "empty spec", body: header + "spec: {}\n"},
		{name: "nested spec key", body: "apiVersion: chainloop.dev/v1\nkind: Contract\nmetadata:\n  name: c\n  spec:\nspec: {}\n"},
		{name: "v1 contract", body: "schemaVersion: v1\nrunner:\n  type: GITHUB_ACTION\n"},
		{
			name:   "JSON is not changed",
			format: unmarshal.RawFormatJSON,
			body:   `{"apiVersion":"chainloop.dev/v1","kind":"Contract","metadata":{"name":"c"},"spec":null}`,
		},
		{name: "not valid UTF-8 is skipped", body: header + "spec:\n# \xff\n"},
	}

	ids := make([]uuid.UUID, len(testCases))
	for i, tc := range testCases {
		format := tc.format
		if format == "" {
			format = unmarshal.RawFormatYAML
		}
		row, err := tu.Data.DB.WorkflowContractVersion.Create().SetRawBody([]byte(tc.body)).SetRawBodyFormat(format).Save(ctx)
		require.NoError(t, err)
		ids[i] = row.ID
	}

	migration, err := os.ReadFile(contractNullSpecMigration)
	require.NoError(t, err)
	// The first run uses the legacy string syntax, where a backslash in '...' is an escape.
	// The second run uses the default syntax, and it must change nothing.
	conn, err := tu.Data.SQLDB.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	for _, conforming := range []string{"off", "on"} {
		_, err = conn.ExecContext(ctx, "SET standard_conforming_strings = "+conforming)
		require.NoError(t, err)
		_, err = conn.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			row, err := tu.Data.DB.WorkflowContractVersion.Get(ctx, ids[i])
			require.NoError(t, err)

			if tc.want == "" {
				assert.Equal(t, tc.body, string(row.RawBody))
				return
			}

			assert.Equal(t, tc.want, string(row.RawBody))
			_, err = biz.UnmarshalAndValidateRawContract([]byte(tc.body), unmarshal.RawFormatYAML)
			require.Error(t, err, "the stored text must fail before the migration")
			_, err = biz.UnmarshalAndValidateRawContract(row.RawBody, unmarshal.RawFormatYAML)
			require.NoError(t, err, "the migrated text must be a valid contract")
		})
	}
}
