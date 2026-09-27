// Copyright 2024-2026 The Chainloop Authors.
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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"sigs.k8s.io/yaml"
)

func TestWorkflowRunExpirerOptsMapping(t *testing.T) {
	tests := []struct {
		name              string
		config            *conf.Bootstrap
		want              time.Duration
		wantCheckInterval time.Duration
	}{
		{
			name:   "missing attestations configuration",
			config: &conf.Bootstrap{},
			want:   0,
		},
		{
			name: "unset expiration window",
			config: &conf.Bootstrap{
				Attestations: &conf.Attestations{},
			},
			want: 0,
		},
		{
			name: "configured expiration window",
			config: &conf.Bootstrap{
				Attestations: &conf.Attestations{
					WorkflowRunExpirationWindow:        durationpb.New(2 * time.Hour),
					WorkflowRunExpirationCheckInterval: durationpb.New(30 * time.Second),
				},
			},
			wantCheckInterval: 30 * time.Second,
			want:              2 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := workflowRunExpirerOpts(tt.config)
			assert.Equal(t, tt.want, opts.ExpirationWindow)
			assert.Equal(t, tt.wantCheckInterval, opts.CheckInterval)
		})
	}
}

func TestWorkflowRunExpirationDurationValidation(t *testing.T) {
	validator, err := newProtoValidator()
	require.NoError(t, err)

	for _, duration := range []time.Duration{0, -time.Second} {
		t.Run(duration.String(), func(t *testing.T) {
			bootstrap := &conf.Bootstrap{
				Attestations: &conf.Attestations{
					WorkflowRunExpirationWindow: durationpb.New(duration),
				},
			}
			require.Error(t, validator.Validate(bootstrap))

			bootstrap = &conf.Bootstrap{
				Attestations: &conf.Attestations{
					WorkflowRunExpirationCheckInterval: durationpb.New(duration),
				},
			}
			require.Error(t, validator.Validate(bootstrap))
		})
	}
}

// Duration fields are decoded with protojson, which only accepts seconds with
// an "s" suffix. Go-style units such as "1m" or "1h" are rejected.
func TestWorkflowRunExpirationDurationFormat(t *testing.T) {
	testCases := []struct {
		name         string
		config       string
		wantErr      bool
		wantWindow   time.Duration
		wantInterval time.Duration
	}{
		{
			name:         "seconds",
			config:       "attestations:\n  workflow_run_expiration_window: 3600s\n  workflow_run_expiration_check_interval: 60s\n",
			wantWindow:   time.Hour,
			wantInterval: time.Minute,
		},
		{
			name:         "quoted seconds",
			config:       "attestations:\n  workflow_run_expiration_window: \"3600s\"\n  workflow_run_expiration_check_interval: \"60s\"\n",
			wantWindow:   time.Hour,
			wantInterval: time.Minute,
		},
		{
			name:         "fractional seconds",
			config:       "attestations:\n  workflow_run_expiration_check_interval: 1.5s\n",
			wantInterval: 1500 * time.Millisecond,
		},
		{
			name:    "window in hours",
			config:  "attestations:\n  workflow_run_expiration_window: 1h\n",
			wantErr: true,
		},
		{
			name:    "check interval in minutes",
			config:  "attestations:\n  workflow_run_expiration_check_interval: 1m\n",
			wantErr: true,
		},
		{
			name:    "malformed window",
			config:  "attestations:\n  workflow_run_expiration_window: definitely-not-a-duration\n",
			wantErr: true,
		},
		{
			name:    "malformed check interval",
			config:  "attestations:\n  workflow_run_expiration_check_interval: definitely-not-a-duration\n",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(tc.config), 0o600))

			bootstrap, err := loadBootstrap(t, configPath)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantWindow, bootstrap.GetAttestations().GetWorkflowRunExpirationWindow().AsDuration())
			assert.Equal(t, tc.wantInterval, bootstrap.GetAttestations().GetWorkflowRunExpirationCheckInterval().AsDuration())
		})
	}
}

// The development configuration is loaded by `make run`, so it must parse.
func TestDevelConfigLoads(t *testing.T) {
	bootstrap, err := loadBootstrap(t, filepath.Join("..", "configs", "config.devel.yaml"))
	require.NoError(t, err)
	assert.Equal(t, time.Hour, bootstrap.GetAttestations().GetWorkflowRunExpirationWindow().AsDuration())
	assert.Equal(t, time.Minute, bootstrap.GetAttestations().GetWorkflowRunExpirationCheckInterval().AsDuration())
}

// The Helm chart copies these values into the control plane configuration, so
// the chart defaults must use a format that the control plane can parse.
func TestHelmChartWorkflowRunExpirationDefaults(t *testing.T) {
	var values struct {
		Controlplane struct {
			Attestations struct {
				WorkflowRunExpirationWindow        string `json:"workflowRunExpirationWindow"`
				WorkflowRunExpirationCheckInterval string `json:"workflowRunExpirationCheckInterval"`
			} `json:"attestations"`
		} `json:"controlplane"`
	}

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "deployment", "chainloop", "values.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(raw, &values))

	attestations := values.Controlplane.Attestations
	// Same shape as templates/controlplane/configmap.yaml
	rendered := fmt.Sprintf("attestations:\n  workflow_run_expiration_window: %q\n  workflow_run_expiration_check_interval: %q\n",
		attestations.WorkflowRunExpirationWindow, attestations.WorkflowRunExpirationCheckInterval)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(rendered), 0o600))

	bootstrap, err := loadBootstrap(t, configPath)
	require.NoError(t, err)
	assert.Equal(t, time.Hour, bootstrap.GetAttestations().GetWorkflowRunExpirationWindow().AsDuration())
	assert.Equal(t, time.Minute, bootstrap.GetAttestations().GetWorkflowRunExpirationCheckInterval().AsDuration())
}

// loadBootstrap loads a configuration file the same way as main.
func loadBootstrap(t *testing.T, path string) (*conf.Bootstrap, error) {
	t.Helper()

	c := config.New(config.WithSource(file.NewSource(path)))
	t.Cleanup(func() {
		require.NoError(t, c.Close())
	})
	require.NoError(t, c.Load())

	var bootstrap conf.Bootstrap
	if err := c.Scan(&bootstrap); err != nil {
		return nil, err
	}

	return &bootstrap, nil
}
