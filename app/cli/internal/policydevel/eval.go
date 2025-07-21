//
// Copyright 2025 The Chainloop Authors.
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

package policydevel

import (
	"context"
	"fmt"
	"os"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/rs/zerolog"

	v12 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
)

type EvalOptions struct {
	PolicyPath   string
	Material     []byte
	MaterialKind string
	Annotations  map[string]string
	MaterialFile string
}

type EvalResult struct {
	Passed     bool
	Violations []Violation
}

type Violation struct {
	Message string
}

func Evaluate(opts *EvalOptions) (*EvalResult, error) {
	schema := &v1.CraftingSchema{
		Policies: &v1.Policies{
			Materials: []*v1.PolicyAttachment{
				{
					Policy: &v1.PolicyAttachment_Ref{Ref: fmt.Sprintf("file://%s", opts.PolicyPath)},
					With:   nil,
				},
			},
			Attestation: nil,
		},
		PolicyGroups: nil,
	}

	// Create the material with annotations
	material := &v12.Attestation_Material{
		M: &v12.Attestation_Material_Artifact_{
			Artifact: &v12.Attestation_Material_Artifact{
				Id:      "evaluated material",
				Content: opts.Material,
			},
		},
		MaterialType: v1.CraftingSchema_Material_MaterialType(v1.CraftingSchema_Material_MaterialType_value[opts.MaterialKind]),
		Annotations:  opts.Annotations,
		InlineCas:    true,
	}

	logger := zerolog.New(os.Stderr).Level(zerolog.WarnLevel)
	v := policies.NewPolicyVerifier(schema, nil, &logger)
	evs, err := v.VerifyMaterial(context.Background(), material, opts.MaterialFile)
	if err != nil {
		return nil, err
	}

	evalResult := &EvalResult{
		Passed:     len(evs[0].Violations) == 0,
		Violations: make([]Violation, 0, len(evs[0].Violations)),
	}

	for _, e := range evs {
		for _, v := range e.Violations {
			evalResult.Violations = append(evalResult.Violations, Violation{
				Message: fmt.Sprintf("%s: %s", v.Subject, v.Message),
			})
		}
	}
	return evalResult, nil
}
