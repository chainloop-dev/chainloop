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

package builtins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProjectContext(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() context.Context
		wantName    string
		wantVersion string
		wantOK      bool
	}{
		{
			name:   "no project context attached",
			setup:  context.Background,
			wantOK: false,
		},
		{
			name: "context with project + version",
			setup: func() context.Context {
				return WithProjectContext(context.Background(), ProjectContext{Name: "my-app", Version: "v1.2.3"})
			},
			wantName:    "my-app",
			wantVersion: "v1.2.3",
			wantOK:      true,
		},
		{
			name: "context with only project name",
			setup: func() context.Context {
				return WithProjectContext(context.Background(), ProjectContext{Name: "my-app"})
			},
			wantName: "my-app",
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc, ok := ProjectContextFromContext(tt.setup())
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantName, pc.Name)
			assert.Equal(t, tt.wantVersion, pc.Version)
		})
	}
}
