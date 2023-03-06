//
// Copyright 2023 The Chainloop Authors.
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

package oci

import (
	"context"
	"testing"

	"github.com/chainloop-dev/bedrock/internal/credentials"
	"github.com/chainloop-dev/bedrock/internal/credentials/mocks"
	"github.com/chainloop-dev/bedrock/internal/ociauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFromCredentials(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)
	r := mocks.NewReader(t)
	const repo, password, username = "repo", "password", "username"

	r.On("ReadOCICreds", ctx, "secretName", mock.AnythingOfType("*credentials.OCIKeypair")).Return(nil).Run(
		func(args mock.Arguments) {
			credentials := args.Get(2).(*credentials.OCIKeypair)
			credentials.Repo = repo
			credentials.Password = password
			credentials.Username = username
		})

	b, err := NewBackendProvider(r).FromCredentials(ctx, "secretName")
	assert.NoError(err)
	creds, err := ociauth.NewCredentials(repo, username, password)
	assert.NoError(err)

	assert.Equal(&Backend{
		repo: repo, prefix: "chainloop",
		keychain: creds,
	}, b)
}
