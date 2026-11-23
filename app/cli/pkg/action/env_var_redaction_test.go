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

package action

import (
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/envredaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestNewEnvVarRedactionItem(t *testing.T) {
	testCases := []struct {
		name    string
		mode    string
		vars    []string
		want    *pb.EnvVarRedaction
		wantErr string
	}{
		{name: "none", mode: string(envredaction.ModeNone), want: &pb.EnvVarRedaction{}},
		{name: "known identity", mode: string(envredaction.ModeKnownIdentity), want: &pb.EnvVarRedaction{Type: pb.EnvVarRedaction_TYPE_KNOWN_IDENTITY}},
		{
			name: "custom list",
			mode: string(envredaction.ModeCustom),
			vars: []string{"BUILD_REQUESTEDFOR", "BUILD_REQUESTEDFOREMAIL"},
			want: &pb.EnvVarRedaction{Type: pb.EnvVarRedaction_TYPE_CUSTOM, EnvVars: []string{"BUILD_REQUESTEDFOR", "BUILD_REQUESTEDFOREMAIL"}},
		},
		{name: "custom without variables", mode: string(envredaction.ModeCustom), wantErr: "custom env var redaction requires at least one variable name"},
		{name: "variables without custom", mode: string(envredaction.ModeKnownIdentity), vars: []string{"MY_OWNER"}, wantErr: "redacted env vars can only be set with the custom mode"},
		{name: "unknown mode", mode: "unknown-mode", wantErr: "invalid env var redaction mode \"unknown-mode\", valid values are: none, known-identity, custom"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewEnvVarRedactionPb(tc.mode, tc.vars)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.True(t, proto.Equal(tc.want, got), "got %v, want %v", got, tc.want)
		})
	}
}

func TestEnvVarRedactionConversions(t *testing.T) {
	testCases := []struct {
		name     string
		in       *pb.EnvVarRedaction
		wantItem *EnvVarRedactionItem
	}{
		{name: "not set", in: nil, wantItem: nil},
		{name: "unspecified", in: &pb.EnvVarRedaction{}, wantItem: nil},
		{
			name:     "known identity",
			in:       &pb.EnvVarRedaction{Type: pb.EnvVarRedaction_TYPE_KNOWN_IDENTITY},
			wantItem: &EnvVarRedactionItem{Mode: envredaction.ModeKnownIdentity},
		},
		{
			name:     "custom list item",
			in:       &pb.EnvVarRedaction{Type: pb.EnvVarRedaction_TYPE_CUSTOM, EnvVars: []string{"OTHER_VAR"}},
			wantItem: &EnvVarRedactionItem{Mode: envredaction.ModeCustom, EnvVars: []string{"OTHER_VAR"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantItem, pbEnvVarRedactionToAction(tc.in))
		})
	}
}
