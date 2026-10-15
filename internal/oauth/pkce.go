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

package oauth

import (
	"crypto/subtle"
	"regexp"

	"golang.org/x/oauth2"
)

// RFC 7636: a verifier is 43 to 128 unreserved characters. A S256 challenge is the
// unpadded base64url encoding of a SHA-256 digest, so always 43 characters.
// Verifiers and challenges are created with oauth2.GenerateVerifier and oauth2.S256ChallengeFromVerifier
var (
	codeVerifierRegexp  = regexp.MustCompile(`^[A-Za-z0-9\-._~]{43,128}$`)
	s256ChallengeRegexp = regexp.MustCompile(`^[A-Za-z0-9\-_]{43}$`)
)

// ValidS256Challenge tells if the value has the shape of a S256 challenge
func ValidS256Challenge(challenge string) bool {
	return s256ChallengeRegexp.MatchString(challenge)
}

// VerifyS256 tells, in constant time, if the verifier matches the S256 challenge
func VerifyS256(verifier, challenge string) bool {
	if !codeVerifierRegexp.MatchString(verifier) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(oauth2.S256ChallengeFromVerifier(verifier)), []byte(challenge)) == 1
}
