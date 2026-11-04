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

package v1

import (
	"testing"

	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/stretchr/testify/assert"
)

func TestAttestationServiceInitRequestRedact(t *testing.T) {
	const identity = "eyJhbGciOiJFUzI1NiJ9.identity-claims.signature"

	req := &AttestationServiceInitRequest{WorkflowName: "pr-validation", ProjectName: "my-project", SandboxIdentity: identity}

	// The logging middleware of the control plane uses Redact
	var _ logging.Redacter = req

	got := req.Redact()
	assert.NotContains(t, got, identity)
	assert.Contains(t, got, redactedValue)
	assert.Contains(t, got, "pr-validation")
	assert.Contains(t, got, "my-project")
	// The request itself keeps the token
	assert.Equal(t, identity, req.GetSandboxIdentity())

	withoutIdentity := &AttestationServiceInitRequest{WorkflowName: "pr-validation"}
	assert.Equal(t, withoutIdentity.String(), withoutIdentity.Redact())
	assert.NotContains(t, withoutIdentity.Redact(), redactedValue)
}
