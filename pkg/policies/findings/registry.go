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

package findings

import (
	"encoding/json"
	"fmt"

	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	FindingTypeVulnerability    = "VULNERABILITY"
	FindingTypeSAST             = "SAST"
	FindingTypeLicenseViolation = "LICENSE_VIOLATION"
)

// findingTypes maps declared finding type strings to proto message constructors.
var findingTypes = map[string]func() proto.Message{
	FindingTypeVulnerability:    func() proto.Message { return &v1.PolicyVulnerabilityFinding{} },
	FindingTypeSAST:             func() proto.Message { return &v1.PolicySASTFinding{} },
	FindingTypeLicenseViolation: func() proto.Message { return &v1.PolicyLicenseViolationFinding{} },
}

// IsValidFindingType checks whether a finding type string is recognized.
func IsValidFindingType(findingType string) bool {
	_, ok := findingTypes[findingType]
	return ok
}

// ValidateFinding validates a raw violation object against the proto schema
// for the given finding type. It marshals the raw map to JSON, unmarshals into
// the corresponding proto message, and runs buf.validate constraints.
// Returns the validated proto message on success.
func ValidateFinding(findingType string, raw map[string]any) (proto.Message, error) {
	factory, ok := findingTypes[findingType]
	if !ok {
		return nil, fmt.Errorf("unknown finding type %q", findingType)
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshaling raw finding to JSON: %w", err)
	}

	msg := factory()
	// DiscardUnknown allows policies to include fields added in newer proto
	// versions without breaking older CLIs that haven't been updated yet.
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("finding does not match %s schema: %w", findingType, err)
	}

	if err := protovalidate.Validate(msg); err != nil {
		return nil, fmt.Errorf("finding validation failed for %s: %w", findingType, err)
	}

	return msg, nil
}

// SetViolationFinding populates the oneof finding field on a Violation proto
// based on the finding type and the validated proto message.
func SetViolationFinding(violation *v1.PolicyEvaluation_Violation, findingType string, finding proto.Message) error {
	switch findingType {
	case FindingTypeVulnerability:
		f, ok := finding.(*v1.PolicyVulnerabilityFinding)
		if !ok {
			return fmt.Errorf("finding is not a PolicyVulnerabilityFinding")
		}
		violation.Finding = &v1.PolicyEvaluation_Violation_Vulnerability{Vulnerability: f}
	case FindingTypeSAST:
		f, ok := finding.(*v1.PolicySASTFinding)
		if !ok {
			return fmt.Errorf("finding is not a PolicySASTFinding")
		}
		violation.Finding = &v1.PolicyEvaluation_Violation_Sast{Sast: f}
	case FindingTypeLicenseViolation:
		f, ok := finding.(*v1.PolicyLicenseViolationFinding)
		if !ok {
			return fmt.Errorf("finding is not a PolicyLicenseViolationFinding")
		}
		violation.Finding = &v1.PolicyEvaluation_Violation_LicenseViolation{LicenseViolation: f}
	default:
		return fmt.Errorf("unknown finding type %q", findingType)
	}

	return nil
}
