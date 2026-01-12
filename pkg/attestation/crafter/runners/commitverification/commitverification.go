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

// CommitVerification represents the result of a commit signature verification
type CommitVerification struct {
	// Whether verification was attempted
	Attempted bool
	// Verification status
	Status VerificationStatus
	// Human-readable reason for the status
	Reason string
	// Platform that performed the verification (e.g., "github", "gitlab")
	Platform string
	// Optional: The signing key ID if verified
	KeyID string
	// Optional: The signature algorithm used
	SignatureAlgorithm string
}
