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

package main

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/cmd"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestErrorInfoPreservesRepositoryOrganizationSource(t *testing.T) {
	err := &cmd.RepositoryOrganizationError{
		Organization: "repository-org",
		Path:         "/repo/.chainloop.yml",
		Err:          v1.ErrorUserNotMemberOfOrgErrorNotInOrg("not a member"),
	}

	message, exitCode := errorInfo(err, zerolog.Nop())

	assert.Equal(t, "organization \"repository-org\" from /repo/.chainloop.yml does not exist or you are not part of it; update the file or run \"chainloop auth login\"", message)
	assert.Equal(t, 1, exitCode)
}
