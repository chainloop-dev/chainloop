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

func TestMalformedWorkflowRunExpirationWindow(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(
		"attestations:\n  workflow_run_expiration_window: definitely-not-a-duration\n",
	), 0o600))

	c := config.New(config.WithSource(file.NewSource(configPath)))
	t.Cleanup(func() {
		require.NoError(t, c.Close())
	})
	require.NoError(t, c.Load())

	var bootstrap conf.Bootstrap
	require.Error(t, c.Scan(&bootstrap))
}

func TestMalformedWorkflowRunExpirationCheckInterval(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(
		"attestations:\n  workflow_run_expiration_check_interval: definitely-not-a-duration\n",
	), 0o600))

	c := config.New(config.WithSource(file.NewSource(configPath)))
	t.Cleanup(func() {
		require.NoError(t, c.Close())
	})
	require.NoError(t, c.Load())

	var bootstrap conf.Bootstrap
	require.Error(t, c.Scan(&bootstrap))
}
