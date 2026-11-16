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

package commitverification

import "fmt"

// VerificationStatus represents the status of a commit signature verification
type VerificationStatus int

const (
	// VerificationStatusUnspecified indicates an unspecified status
	VerificationStatusUnspecified VerificationStatus = iota
	// VerificationStatusVerified indicates the signature was successfully verified
	VerificationStatusVerified
	// VerificationStatusUnverified indicates the signature check failed or is invalid
	VerificationStatusUnverified
	// VerificationStatusUnavailable indicates verification could not be performed
	VerificationStatusUnavailable
	// VerificationStatusNotApplicable indicates no signature present or platform doesn't support it
	VerificationStatusNotApplicable
)

// verificationStatusNames are the text forms of the statuses. They are the same
// as the status names in the attestation.
var verificationStatusNames = map[VerificationStatus]string{
	VerificationStatusUnspecified:   "unspecified",
	VerificationStatusVerified:      "verified",
	VerificationStatusUnverified:    "unverified",
	VerificationStatusUnavailable:   "unavailable",
	VerificationStatusNotApplicable: "not_applicable",
}

// String returns the text form of the status.
func (s VerificationStatus) String() string {
	if name, ok := verificationStatusNames[s]; ok {
		return name
	}

	return fmt.Sprintf("VerificationStatus(%d)", int(s))
}

// MarshalText encodes the status as its text form, so that JSON carries a name
// and not a number that depends on the order of the constants.
func (s VerificationStatus) MarshalText() ([]byte, error) {
	name, ok := verificationStatusNames[s]
	if !ok {
		return nil, fmt.Errorf("unknown verification status %d", int(s))
	}

	return []byte(name), nil
}

// UnmarshalText decodes the text form of a status.
func (s *VerificationStatus) UnmarshalText(text []byte) error {
	for status, name := range verificationStatusNames {
		if name == string(text) {
			*s = status
			return nil
		}
	}

	return fmt.Errorf("unknown verification status %q", string(text))
}

// CommitVerification represents the result of a commit signature verification
type CommitVerification struct {
	// Whether verification was attempted
	Attempted bool `json:"attempted"`
	// Verification status
	Status VerificationStatus `json:"status"`
	// Human-readable reason for the status
	Reason string `json:"reason,omitempty"`
	// Platform that performed the verification (e.g., "github", "gitlab")
	Platform string `json:"platform,omitempty"`
	// Optional: The signing key ID if verified
	KeyID string `json:"key_id,omitempty"`
	// Optional: The signature algorithm used
	SignatureAlgorithm string `json:"signature_algorithm,omitempty"`
}
