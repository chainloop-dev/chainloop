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
	"errors"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/envredaction"
)

// EnvVarRedactionItem is the org-level redaction of env var values in attestations.
type EnvVarRedactionItem struct {
	Mode    envredaction.Mode `json:"mode"`
	EnvVars []string          `json:"envVars,omitempty"`
}

// NewEnvVarRedactionPb builds the API setting from the CLI flag values.
func NewEnvVarRedactionPb(mode string, envVars []string) (*pb.EnvVarRedaction, error) {
	m := envredaction.Mode(mode)
	if len(envVars) > 0 && m != envredaction.ModeCustom {
		return nil, errors.New("redacted env vars can only be set with the custom mode")
	}

	// Names rejects an unknown mode and a custom mode with no names, as the control plane does.
	if _, err := envredaction.Names(m, envVars); err != nil {
		return nil, err
	}

	out := &pb.EnvVarRedaction{Type: pb.EnvVarRedaction_TYPE_UNSPECIFIED, EnvVars: envVars}
	switch m {
	case envredaction.ModeKnownIdentity:
		out.Type = pb.EnvVarRedaction_TYPE_KNOWN_IDENTITY
	case envredaction.ModeCustom:
		out.Type = pb.EnvVarRedaction_TYPE_CUSTOM
	}

	return out, nil
}

// pbEnvVarRedactionToAction converts the API setting for output. No redaction returns nil.
func pbEnvVarRedactionToAction(in *pb.EnvVarRedaction) *EnvVarRedactionItem {
	switch in.GetType() {
	case pb.EnvVarRedaction_TYPE_KNOWN_IDENTITY:
		return &EnvVarRedactionItem{Mode: envredaction.ModeKnownIdentity, EnvVars: in.GetEnvVars()}
	case pb.EnvVarRedaction_TYPE_CUSTOM:
		return &EnvVarRedactionItem{Mode: envredaction.ModeCustom, EnvVars: in.GetEnvVars()}
	default:
		return nil
	}
}
