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

package service

import (
	"context"
	"testing"

	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	jwtm "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func TestInfoFromAuth(t *testing.T) {
	t.Run("no claims", func(t *testing.T) {
		_, err := infoFromAuth(jwtm.NewContext(context.Background(), nil))
		assert.Error(t, err)
	})

	t.Run("invalid claims", func(t *testing.T) {
		_, err := infoFromAuth(jwtm.NewContext(context.Background(), &jwt.RegisteredClaims{}))
		assert.Error(t, err)
	})

	t.Run("valid claims", func(t *testing.T) {
		want := &casJWT.Claims{Role: "test", StoredSecretID: "test"}
		got, err := infoFromAuth(jwtm.NewContext(context.Background(), want))
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})
}
