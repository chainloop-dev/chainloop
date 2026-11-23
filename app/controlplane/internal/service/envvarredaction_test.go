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

package service

import (
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/pkg/attestation/envredaction"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

const fooVar = "FOO"

func TestEnvVarRedactionConversion(t *testing.T) {
	testCases := []struct {
		name string
		pb   *pb.EnvVarRedaction
		biz  *biz.EnvVarRedaction
		// wantPb is what the biz value converts back to, when it differs from pb
		wantPb *pb.EnvVarRedaction
	}{
		{name: "not set", pb: nil, biz: nil},
		{
			name:   "unspecified means none",
			pb:     &pb.EnvVarRedaction{},
			biz:    &biz.EnvVarRedaction{Mode: envredaction.ModeNone},
			wantPb: &pb.EnvVarRedaction{Type: pb.EnvVarRedaction_TYPE_UNSPECIFIED},
		},
		{
			name: "known identity",
			pb:   &pb.EnvVarRedaction{Type: pb.EnvVarRedaction_TYPE_KNOWN_IDENTITY},
			biz:  &biz.EnvVarRedaction{Mode: envredaction.ModeKnownIdentity},
		},
		{
			name: "custom",
			pb:   &pb.EnvVarRedaction{Type: pb.EnvVarRedaction_TYPE_CUSTOM, EnvVars: []string{fooVar, "BAR"}},
			biz:  &biz.EnvVarRedaction{Mode: envredaction.ModeCustom, EnvVars: []string{fooVar, "BAR"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.biz, pbEnvVarRedactionToBiz(tc.pb))

			want := tc.pb
			if tc.wantPb != nil {
				want = tc.wantPb
			}
			assert.True(t, proto.Equal(want, bizEnvVarRedactionToPb(tc.biz)), "got %v, want %v", bizEnvVarRedactionToPb(tc.biz), want)
		})
	}
}
