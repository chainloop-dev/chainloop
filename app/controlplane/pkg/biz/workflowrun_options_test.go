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

package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveWorkflowRunExpirerOpts(t *testing.T) {
	tests := []struct {
		name                 string
		opts                 *WorkflowRunExpirerOpts
		wantExpirationWindow time.Duration
		wantCheckInterval    time.Duration
	}{
		{
			name:                 "nil options use defaults",
			wantCheckInterval:    time.Minute,
			wantExpirationWindow: time.Hour,
		},
		{
			name:                 "empty options use defaults",
			opts:                 &WorkflowRunExpirerOpts{},
			wantExpirationWindow: time.Hour,
			wantCheckInterval:    time.Minute,
		},
		{
			name: "non-positive options use defaults",
			opts: &WorkflowRunExpirerOpts{
				ExpirationWindow: -time.Hour,
				CheckInterval:    -time.Minute,
			},
			wantExpirationWindow: time.Hour,
			wantCheckInterval:    time.Minute,
		},
		{
			name: "positive expiration preserves override with zero check interval",
			opts: &WorkflowRunExpirerOpts{
				ExpirationWindow: 2 * time.Hour,
				CheckInterval:    0,
			},
			wantExpirationWindow: 2 * time.Hour,
			wantCheckInterval:    time.Minute,
		},
		{
			name: "positive check interval preserves override with negative expiration",
			opts: &WorkflowRunExpirerOpts{
				ExpirationWindow: -time.Hour,
				CheckInterval:    5 * time.Minute,
			},
			wantExpirationWindow: time.Hour,
			wantCheckInterval:    5 * time.Minute,
		},
		{
			name: "positive options are preserved",
			opts: &WorkflowRunExpirerOpts{
				ExpirationWindow: 2 * time.Hour,
				CheckInterval:    5 * time.Minute,
			},
			wantExpirationWindow: 2 * time.Hour,
			wantCheckInterval:    5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveWorkflowRunExpirerOpts(tt.opts)
			assert.Equal(t, tt.wantExpirationWindow, got.ExpirationWindow)
			assert.Equal(t, tt.wantCheckInterval, got.CheckInterval)
		})
	}
}
