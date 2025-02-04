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

package manager_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/credentials"
	v1 "github.com/chainloop-dev/chainloop/pkg/credentials/api/credentials/v1"
	"github.com/chainloop-dev/chainloop/pkg/credentials/manager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var validAWSConfig = &v1.Credentials{
	Backend: &v1.Credentials_AwsSecretManager{
		AwsSecretManager: &v1.Credentials_AWSSecretManager{
			Region: "us-east-1",
			Creds:  &v1.Credentials_AWSSecretManager_Creds{AccessKey: "ak", SecretKey: "sk"},
		},
	},
}

var validGCPConfig = &v1.Credentials{
	Backend: &v1.Credentials_GcpSecretManager{
		GcpSecretManager: &v1.Credentials_GCPSecretManager{
			ProjectId:         "project",
			ServiceAccountKey: "../gcp/testdata/test_gcp_key.json",
		},
	},
}

func validVaultConfig(s *testSuite) *v1.Credentials {
	return &v1.Credentials{
		Backend: &v1.Credentials_Vault_{
			Vault: &v1.Credentials_Vault{
				Token:   "notasecret",
				Address: s.connectionString,
			},
		},
	}
}

func (s *testSuite) TestNewAzureManagerFromConfig() {
	testCases := []struct {
		name          string
		tenantID      string
		clientID      string
		clientSecret  string
		vaultURI      string
		Role          credentials.Role
		expectedError bool
	}{
		{name: "missing tenantID", tenantID: "", clientID: "clientID", clientSecret: "clientSecret", vaultURI: "vaultURI", Role: credentials.RoleReader, expectedError: true},
		{name: "missing clientID", tenantID: "tenantID", clientID: "", clientSecret: "clientSecret", vaultURI: "vaultURI", Role: credentials.RoleReader, expectedError: true},
		{name: "missing clientSecret", tenantID: "tenantID", clientID: "clientID", clientSecret: "", vaultURI: "vaultURI", Role: credentials.RoleReader, expectedError: true},
		{name: "missing vaultURI", tenantID: "tenantID", clientID: "clientID", clientSecret: "clientSecret", vaultURI: "", Role: credentials.RoleReader, expectedError: true},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			conf := &v1.Credentials{
				Backend: &v1.Credentials_AzureKeyVault_{
					AzureKeyVault: &v1.Credentials_AzureKeyVault{
						TenantId: tc.tenantID, ClientId: tc.clientID, ClientSecret: tc.clientSecret, VaultUri: tc.vaultURI,
					},
				},
			}

			_, err := manager.NewFromConfig(conf, tc.Role, nil)
			if tc.expectedError {
				assert.Error(s.T(), err)
			} else {
				assert.NoError(s.T(), err)
			}
		})
	}
}

func (s *testSuite) TestNewFromConfig() {
	testCases := []struct {
		name    string
		conf    *v1.Credentials
		wantErr bool
	}{
		{
			name:    "no credentials manager configuration provided",
			conf:    nil,
			wantErr: true,
		},
		{
			name:    "empty credentials manager configuration",
			conf:    &v1.Credentials{},
			wantErr: true,
		},
		{
			name:    "[AWS] valid configuration",
			conf:    validAWSConfig,
			wantErr: false,
		},
		{
			name: "[AWS] missing region",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_AwsSecretManager{
					AwsSecretManager: &v1.Credentials_AWSSecretManager{
						Creds: &v1.Credentials_AWSSecretManager_Creds{AccessKey: "ak", SecretKey: "sk"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "[AWS] missing credentials",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_AwsSecretManager{
					AwsSecretManager: &v1.Credentials_AWSSecretManager{
						Region: "us-east-1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "[AWS] missing AWS access key",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_AwsSecretManager{
					AwsSecretManager: &v1.Credentials_AWSSecretManager{
						Region: "us-east-1",
						Creds:  &v1.Credentials_AWSSecretManager_Creds{SecretKey: "sk"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "[AWS] missing AWS secret key",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_AwsSecretManager{
					AwsSecretManager: &v1.Credentials_AWSSecretManager{
						Region: "us-east-1",
						Creds:  &v1.Credentials_AWSSecretManager_Creds{AccessKey: "ak"},
					},
				},
			},
			wantErr: true,
		},
		{
			name:    "[GCP] Valid configuration",
			conf:    validGCPConfig,
			wantErr: false,
		},
		{
			name: "[GCP] missing project ID",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_GcpSecretManager{
					GcpSecretManager: &v1.Credentials_GCPSecretManager{
						ServiceAccountKey: "../gcp/testdata/test_gcp_key.json",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "[GCP] missing key path",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_GcpSecretManager{
					GcpSecretManager: &v1.Credentials_GCPSecretManager{
						ProjectId: "project",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "[GCP] invalid key path",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_GcpSecretManager{
					GcpSecretManager: &v1.Credentials_GCPSecretManager{
						ProjectId:         "project",
						ServiceAccountKey: "does-exist.json",
					},
				},
			},
			wantErr: true,
		},
		{
			name:    "[Vault] valid configuration",
			conf:    validVaultConfig(s),
			wantErr: false,
		},
		{
			name: "[Vault] missing token",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_Vault_{
					Vault: &v1.Credentials_Vault{
						Address: s.connectionString,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "[Vault] missing address",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_Vault_{
					Vault: &v1.Credentials_Vault{
						Token: "notasecret",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "[Vault] invalid address",
			conf: &v1.Credentials{
				Backend: &v1.Credentials_Vault_{
					Vault: &v1.Credentials_Vault{
						Token:   "notasecret",
						Address: "http://non-existing:5000",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			_, err := manager.NewFromConfig(tc.conf, credentials.RoleReader, nil)
			if tc.wantErr {
				assert.Error(s.T(), err)
			} else {
				assert.NoError(s.T(), err)
			}
		})
	}
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
			"VAULT_DEV_ROOT_TOKEN_ID": "notasecret",
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
