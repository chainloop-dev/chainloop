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
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"regexp"
)

// RFC 7636: a verifier is 43 to 128 unreserved characters. A S256 challenge is the
// unpadded base64url encoding of a SHA-256 digest, so always 43 characters
var (
	codeVerifierRegexp   = regexp.MustCompile(`^[A-Za-z0-9\-._~]{43,128}$`)
	s256ChallengeRegexp  = regexp.MustCompile(`^[A-Za-z0-9\-_]{43}$`)
	codeVerifierRandSize = 32
)

// NewCodeVerifier returns a random PKCE verifier with 256 bits of entropy
func NewCodeVerifier() (string, error) {
	b := make([]byte, codeVerifierRandSize)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CodeChallengeS256 returns the S256 challenge of the given verifier
func CodeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// ValidS256Challenge tells if the value has the shape of a S256 challenge
func ValidS256Challenge(challenge string) bool {
	return s256ChallengeRegexp.MatchString(challenge)
}

// VerifyS256 tells, in constant time, if the verifier matches the S256 challenge
func VerifyS256(verifier, challenge string) bool {
	if !codeVerifierRegexp.MatchString(verifier) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(CodeChallengeS256(verifier)), []byte(challenge)) == 1
}
