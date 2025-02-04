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

package vault_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/chainloop-dev/chainloop/pkg/credentials/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const defaultToken = "notasecret"

func (s *testSuite) TestNewManager() {
	assert := assert.New(s.T())

	testCases := []struct {
		name          string
		connection    string
		token         string
		path          string
		prefix        string
		expectedError bool
		role          credentials.Role
	}{
		{name: "missing token", connection: s.connectionString, expectedError: true},
		{name: "missing address", token: defaultToken, expectedError: true},
		{name: "invalid address reader", token: defaultToken, connection: "http://non-existing:5000", expectedError: true, role: credentials.RoleReader},
		{name: "invalid address writer", token: defaultToken, connection: "http://non-existing:5000", expectedError: true},
		{name: "invalid mount path", token: defaultToken, connection: s.connectionString, path: "non-existing", expectedError: true},
		{name: "valid connection reader", connection: s.connectionString, token: defaultToken, role: credentials.RoleReader},
		{name: "valid connection reader with prefix", connection: s.connectionString, token: defaultToken, role: credentials.RoleReader, prefix: "prefix"},
		{name: "valid connection", connection: s.connectionString, token: defaultToken},
		{name: "valid connection with prefix", connection: s.connectionString, token: defaultToken, prefix: "prefix"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			opts := &vault.NewManagerOpts{AuthToken: tc.token, Address: tc.connection, MountPath: tc.path, Role: tc.role, SecretPrefix: tc.prefix}
			_, err := vault.NewManager(opts)
			if tc.expectedError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

const orgID = "test-org"

type storedSecret struct {
	Foo, Bar string
}

func (s *testSuite) TestReadWriteCredentials() {
	assert := assert.New(s.T())
	validOCICreds := &credentials.OCIKeypair{Repo: "test-repo", Username: "username", Password: "password"}

	testCases := []struct {
		name               string
		want               any
		path               string
		expectedWriteError bool
	}{
		{"valid creds", validOCICreds, "", false},
		{"valid creds custom path", validOCICreds, "fooo", false},
		{"random struct is compatible", &storedSecret{"bar", "baz"}, "", false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			opts := &vault.NewManagerOpts{AuthToken: defaultToken, Address: s.connectionString, SecretPrefix: tc.path}
			m, err := vault.NewManager(opts)
			require.NoError(s.T(), err)

			secretName, err := m.SaveCredentials(context.Background(), orgID, tc.want)
			if tc.expectedWriteError {
				assert.Error(err)
				return
			}

			assert.NoError(err)
			// Read the keypair choosing the returning struct
			var got any
			switch reflect.TypeOf(tc.want).String() {
			case "*credentials.OCIKeypair":
				got = &credentials.OCIKeypair{}
			default:
				got = &storedSecret{}
			}

			err = m.ReadCredentials(context.Background(), secretName, got)
			assert.NoError(err)

			// Compare the keypair
			assert.Equal(tc.want, got)
		})
	}

	// Check error if the key doesn't exist
	opts := &vault.NewManagerOpts{AuthToken: defaultToken, Address: s.connectionString}
	m, err := vault.NewManager(opts)
	require.NoError(s.T(), err)
	err = m.ReadCredentials(context.Background(), "bogus", nil)
	assert.ErrorIs(err, credentials.ErrNotFound)
}

// Create a new secret, delete it and check it does not exist antymore
func (s *testSuite) TestDeleteCreds() {
	assert := assert.New(s.T())
	require := require.New(s.T())
	validCreds := &credentials.OCIKeypair{Repo: "test-repo", Username: "username", Password: "password"}

	opts := &vault.NewManagerOpts{AuthToken: defaultToken, Address: s.connectionString}
	m, err := vault.NewManager(opts)
	require.NoError(err)

	secretName, err := m.SaveCredentials(context.Background(), orgID, validCreds)
	require.NoError(err)

	// Read the keypair
	got := &credentials.OCIKeypair{}
	err = m.ReadCredentials(context.Background(), secretName, got)
	assert.NoError(err)
	// Compare the keypair
	assert.Equal(validCreds, got)

	// Delete and check it does not exist
	err = m.DeleteCredentials(context.Background(), secretName)
	assert.NoError(err)

	// It does not exist
	got = &credentials.OCIKeypair{}
	err = m.ReadCredentials(context.Background(), secretName, got)
	assert.Error(err)
}

type testSuite struct {
	suite.Suite
	vault            *vaultInstance
	connectionString string
}

// Create a vault instance for the test suite that gets created and tear down for each test
func newVaultInstance(t *testing.T) *vaultInstance {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        "vault:1.12.3",
		ExposedPorts: []string{"8200/tcp"},
		AutoRemove:   true,
		Env: map[string]string{
			"VAULT_DEV_ROOT_TOKEN_ID": defaultToken,
		},
		WaitingFor: wait.ForHTTP("/v1/sys/health").WithPort("8200/tcp"),
	}

	instance, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	require.NoError(t, err)
	return &vaultInstance{instance}
}

func (db *vaultInstance) ConnectionString(t *testing.T) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	p, err := db.instance.MappedPort(ctx, "8200")
	assert.NoError(t, err)

	return fmt.Sprintf("http://0.0.0.0:%d", p.Int())
}

type vaultInstance struct {
	instance testcontainers.Container
}

func (s *testSuite) SetupSuite() {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		s.T().Skip()
	}
}

// Run before each test
func (s *testSuite) SetupTest() {
	s.vault = newVaultInstance(s.T())
	s.connectionString = s.vault.ConnectionString(s.T())
}

func (s *testSuite) TearDownTest() {
	testcontainers.CleanupContainer(s.T(), s.vault.instance, testcontainers.StopTimeout(time.Minute))
}

// Run the tests
func TestVaultIntegration(t *testing.T) {
	suite.Run(t, new(testSuite))
}
