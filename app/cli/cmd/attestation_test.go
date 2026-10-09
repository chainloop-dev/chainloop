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

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	controlplanev1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestOrgFromLocalState(t *testing.T) {
	t.Run("returns org from valid state file", func(t *testing.T) {
		state := &crafter.VersionedCraftingState{
			CraftingState: &v1.CraftingState{
				Attestation: &v1.Attestation{
					Workflow: &v1.WorkflowMetadata{
						Organization: "my-org",
					},
				},
			},
		}

		raw, err := protojson.Marshal(state)
		require.NoError(t, err)

		statePath := filepath.Join(t.TempDir(), "state.json")
		require.NoError(t, os.WriteFile(statePath, raw, 0o600))

		assert.Equal(t, "my-org", orgFromLocalState(statePath))
	})

	t.Run("returns empty for missing file", func(t *testing.T) {
		assert.Empty(t, orgFromLocalState(filepath.Join(t.TempDir(), "nonexistent.json")))
	})

	t.Run("returns empty for invalid json", func(t *testing.T) {
		statePath := filepath.Join(t.TempDir(), "bad.json")
		require.NoError(t, os.WriteFile(statePath, []byte("not json"), 0o600))
		assert.Empty(t, orgFromLocalState(statePath))
	})

	t.Run("returns empty when org not set in state", func(t *testing.T) {
		state := &crafter.VersionedCraftingState{
			CraftingState: &v1.CraftingState{
				Attestation: &v1.Attestation{
					Workflow: &v1.WorkflowMetadata{},
				},
			},
		}

		raw, err := protojson.Marshal(state)
		require.NoError(t, err)

		statePath := filepath.Join(t.TempDir(), "state.json")
		require.NoError(t, os.WriteFile(statePath, raw, 0o600))

		assert.Empty(t, orgFromLocalState(statePath))
	})
}

func TestAttestationInitValidatesFlagsRegardlessOfRepositoryConfig(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{name: "missing"},
		{name: "unreadable", setup: func(t *testing.T, dir string) {
			require.NoError(t, os.Mkdir(filepath.Join(dir, ".chainloop.yml"), 0o700))
		}},
		{name: "present", setup: func(t *testing.T, dir string) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("{}\n"), 0o600))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.setup != nil {
				tc.setup(t, dir)
			}
			t.Chdir(dir)

			cmd := newAttestationInitCmd()
			require.NoError(t, cmd.Flags().Set("workflow", "build"))
			require.NoError(t, cmd.Flags().Set("release", "true"))

			assert.EqualError(t, cmd.PreRunE(cmd, nil), "project version is required when using --release")
		})
	}
}

func TestAttestationRepositoryOrganizationCommands(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".chainloop.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("organization: repository-org\n"), 0o600))
	configPath, err := filepath.EvalSymlinks(configPath)
	require.NoError(t, err)
	t.Chdir(dir)

	for _, name := range []string{"init", "add", "push", "status", "reset"} {
		t.Run(name, func(t *testing.T) {
			_, cmd := newAttestationOrganizationTestCommand(t, name, "saved-org")
			require.NoError(t, resolveAttestationOrganization(cmd))

			organization, path := attestationOrganization(cmd)
			assert.Equal(t, "repository-org", organization)
			assert.Equal(t, configPath, path)
			assert.Equal(t, "repository-org", effectiveOrganization(cmd))
			assert.Equal(t, "saved-org", viper.GetString(confOptions.organization.viperKey))
		})
	}
}

func TestAttestationOrganizationPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name        string
		flag        string
		environment string
		repository  string
		local       string
		want        string
	}{
		{name: "flag", flag: "flag-org", environment: "environment-org", repository: "repository-org", local: "local-org", want: "flag-org"},
		{name: "environment", environment: "environment-org", repository: "repository-org", local: "local-org", want: "environment-org"},
		{name: "repository", repository: "repository-org", local: "local-org", want: "repository-org"},
		{name: "local state", local: "local-org", want: "local-org"},
		{name: "saved default", want: "saved-org"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.repository != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("organization: "+tc.repository+"\n"), 0o600))
			}
			t.Chdir(dir)

			root, cmd := newAttestationOrganizationTestCommand(t, "status", "saved-org")
			t.Setenv(CalculateEnvVarName(confOptions.organization.viperKey), tc.environment)
			if tc.flag != "" {
				require.NoError(t, root.PersistentFlags().Set(confOptions.organization.flagName, tc.flag))
			}
			if tc.local != "" {
				attestationLocalStatePath = writeAttestationOrganizationState(t, tc.local)
			}

			require.NoError(t, resolveAttestationOrganization(cmd))
			assert.Equal(t, tc.want, effectiveOrganization(cmd))
		})
	}
}

func TestAttestationVerifyDoesNotReadRepositoryOrganization(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("organization: repository-org\n"), 0o600))
	t.Chdir(dir)

	_, cmd := newAttestationOrganizationTestCommand(t, "verify", "saved-org")
	require.NoError(t, resolveAttestationOrganization(cmd))

	organization, path := attestationOrganization(cmd)
	assert.Empty(t, organization)
	assert.Empty(t, path)
	assert.Equal(t, "saved-org", effectiveOrganization(cmd))
}

func TestAttestationRepositoryOrganizationTokenMismatchStopsBeforeRootPreRun(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".chainloop.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("organization: repository-org\n"), 0o600))
	configPath, err := filepath.EvalSymlinks(configPath)
	require.NoError(t, err)
	t.Chdir(dir)

	root, cmd := newAttestationOrganizationTestCommand(t, "status", "saved-org")
	rootPreRunCalled := false
	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		rootPreRunCalled = true
		return nil
	}

	claims := jwt.MapClaims{"aud": "api-token-auth.chainloop", "jti": "token-id", "org_name": "token-org"}
	rawToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	require.NoError(t, err)
	apiToken = rawToken

	err = cmd.Parent().PersistentPreRunE(cmd, nil)
	require.Error(t, err)
	assert.False(t, rootPreRunCalled)
	assert.ErrorContains(t, err, "token-org")
	assert.ErrorContains(t, err, "repository-org")
	assert.ErrorContains(t, err, "use an API token for")
	assert.ErrorContains(t, err, "remove organization")
	assert.ErrorContains(t, err, configPath)
}

func TestAttestationRepositoryOrganizationAllowsInstanceAdminToken(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("organization: repository-org\n"), 0o600))
	t.Chdir(dir)

	root, cmd := newAttestationOrganizationTestCommand(t, "status", "saved-org")
	rootPreRunCalled := false
	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		rootPreRunCalled = true
		return nil
	}

	claims := jwt.MapClaims{"aud": "api-token-auth.chainloop", "jti": "instance-token-id", "scope": "INSTANCE_ADMIN"}
	rawToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	require.NoError(t, err)
	apiToken = rawToken

	require.NoError(t, cmd.Parent().PersistentPreRunE(cmd, nil))
	assert.True(t, rootPreRunCalled)
	assert.Equal(t, "repository-org", effectiveOrganization(cmd))
}

func TestAttestationRepositoryOrganizationMembershipErrorPreservesSavedDefault(t *testing.T) {
	_, cmd := newAttestationOrganizationTestCommand(t, "status", "saved-org")
	path := filepath.Join(t.TempDir(), ".chainloop.yml")
	setAttestationOrganization(cmd, "repository-org", path)

	err := handleOrganizationExecutionError(cmd, controlplanev1.ErrorUserNotMemberOfOrgErrorNotInOrg("not a member"))
	require.Error(t, err)
	assert.ErrorContains(t, err, path)
	assert.Equal(t, "saved-org", viper.GetString(confOptions.organization.viperKey))
}

func TestExplicitOrganizationMembershipErrorPreservesSavedDefault(t *testing.T) {
	_, cmd := newAttestationOrganizationTestCommand(t, "status", "saved-org")
	t.Setenv(CalculateEnvVarName(confOptions.organization.viperKey), "temporary-org")

	err := handleOrganizationExecutionError(cmd, controlplanev1.ErrorUserNotMemberOfOrgErrorNotInOrg("not a member"))
	require.Error(t, err)

	t.Setenv(CalculateEnvVarName(confOptions.organization.viperKey), "")
	assert.Equal(t, "saved-org", viper.GetString(confOptions.organization.viperKey))
}

func TestAttestationRepositoryOrganizationNotice(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".chainloop.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("organization: repository-org\n"), 0o600))
	configPath, err := filepath.EvalSymlinks(configPath)
	require.NoError(t, err)
	t.Chdir(dir)

	_, cmd := newAttestationOrganizationTestCommand(t, "status", "saved-org")
	var output bytes.Buffer
	previousLogger := logger
	logger = zerolog.New(&output)
	t.Cleanup(func() { logger = previousLogger })
	require.NoError(t, resolveAttestationOrganization(cmd))

	assert.Equal(t, 1, strings.Count(output.String(), "using organization from repository config"))
	assert.Contains(t, output.String(), "repository-org")
	assert.Contains(t, output.String(), configPath)
	assert.Equal(t, "saved-org", viper.GetString(confOptions.organization.viperKey))
}

func TestAttestationRepositoryConfigLookupLogging(t *testing.T) {
	for _, tc := range []struct {
		name     string
		config   string
		debug    bool
		wantWarn bool
	}{
		{name: "missing config"},
		{name: "missing config with debug", debug: true},
		{name: "invalid config", config: "organization: [\n", wantWarn: true},
		{name: "invalid config with debug", config: "organization: [\n", debug: true, wantWarn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.config != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte(tc.config), 0o600))
			}
			t.Chdir(dir)

			_, cmd := newAttestationOrganizationTestCommand(t, "init", "saved-org")
			previousDebug := flagDebug
			flagDebug = tc.debug
			t.Cleanup(func() { flagDebug = previousDebug })

			// Same as the logger seeded in main.go: no level set until the root
			// PersistentPreRunE runs.
			var output bytes.Buffer
			logger = zerolog.New(&output)

			require.NoError(t, cmd.Parent().PersistentPreRunE(cmd, nil))
			// init reads .chainloop.yml again for the project metadata. --release
			// without a version makes PreRunE stop right after that lookup.
			require.NoError(t, cmd.Flags().Set("workflow", "build"))
			require.NoError(t, cmd.Flags().Set("release", "true"))
			require.EqualError(t, cmd.PreRunE(cmd, nil), "project version is required when using --release")

			if tc.wantWarn {
				assert.Contains(t, output.String(), `"level":"warn"`)
			} else {
				assert.Empty(t, output.String())
			}
			if !tc.debug {
				assert.NotContains(t, output.String(), `"level":"debug"`)
			}

			wantLevel := zerolog.InfoLevel
			if tc.debug {
				wantLevel = zerolog.DebugLevel
			}
			assert.Equal(t, wantLevel, logger.GetLevel())
		})
	}
}

func newAttestationOrganizationTestCommand(t *testing.T, command, savedOrganization string) (*cobra.Command, *cobra.Command) {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv(tokenEnvVarName, "")
	t.Setenv(CalculateEnvVarName(confOptions.organization.viperKey), "")

	previousAPIToken := apiToken
	previousLocalStatePath := attestationLocalStatePath
	previousRemoteState := useAttestationRemoteState
	previousLogger := logger
	apiToken = ""
	missingLocalStatePath := filepath.Join(t.TempDir(), "missing-state.json")
	useAttestationRemoteState = false
	logger = zerolog.Nop()
	t.Cleanup(func() {
		apiToken = previousAPIToken
		attestationLocalStatePath = previousLocalStatePath
		useAttestationRemoteState = previousRemoteState
		logger = previousLogger
	})

	viper.SetConfigType("toml")
	require.NoError(t, viper.ReadConfig(strings.NewReader("organization = \""+savedOrganization+"\"\n")))

	root := &cobra.Command{Use: appName, PersistentPreRunE: func(*cobra.Command, []string) error { return nil }}
	root.PersistentFlags().String(confOptions.organization.flagName, "", "")
	require.NoError(t, viper.BindPFlag(confOptions.organization.viperKey, root.PersistentFlags().Lookup(confOptions.organization.flagName)))
	require.NoError(t, viper.BindEnv(confOptions.organization.viperKey, CalculateEnvVarName(confOptions.organization.viperKey)))

	attestationCmd := newAttestationCmd()
	attestationLocalStatePath = missingLocalStatePath
	root.AddCommand(attestationCmd)
	for _, candidate := range attestationCmd.Commands() {
		if candidate.Name() == command {
			return root, candidate
		}
	}
	require.FailNowf(t, "attestation command not found", "command: %q", command)
	return nil, nil
}

func writeAttestationOrganizationState(t *testing.T, organization string) string {
	t.Helper()
	state := &crafter.VersionedCraftingState{CraftingState: &v1.CraftingState{Attestation: &v1.Attestation{Workflow: &v1.WorkflowMetadata{Organization: organization}}}}
	raw, err := protojson.Marshal(state)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	return path
}

func TestExtractAnnotations(t *testing.T) {
	testCases := []struct {
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{
			input: []string{
				"foo=bar",
				"baz=qux",
			},
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			wantErr: false,
		},
		{
			input: []string{
				"foo=bar",
				"baz",
			},
			wantErr: true,
		},
		{
			input: []string{
				"foo=bar",
				"baz=qux",
				"foo=bar",
			},
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			wantErr: false,
		},
		{
			input: []string{
				"foo=bar",
				"baz=qux=qux",
			},
			want: map[string]string{
				"foo": "bar",
				"baz": "qux=qux",
			},
			wantErr: false,
		},
		{
			input: []string{
				"url=https://example.com/path?id=123&foo=bar",
			},
			want: map[string]string{
				"url": "https://example.com/path?id=123&foo=bar",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		got, err := extractAnnotations(tc.input)
		if tc.wantErr {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
}
