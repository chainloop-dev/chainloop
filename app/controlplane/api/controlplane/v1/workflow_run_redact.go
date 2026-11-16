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

import "google.golang.org/protobuf/proto"

// redactedValue replaces a sensitive field in a logged request.
const redactedValue = "[REDACTED]"

// Redact returns the request for the request log without the sandbox identity token. The
// logging middleware prints the request arguments, and the token must not go to the logs.
func (m *AttestationServiceInitRequest) Redact() string {
	if m.GetSandboxIdentity() == "" {
		return m.String()
	}

	redacted, ok := proto.Clone(m).(*AttestationServiceInitRequest)
	if !ok {
		return redactedValue
	}

	redacted.SandboxIdentity = redactedValue

	return redacted.String()
}
