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

package biz

import (
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt"
	robotaccount "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
)

type CASCredentialsUseCase struct {
	jwtBuilder *robotaccount.Builder
}

func NewCASCredentialsUseCase(c *conf.Auth) (*CASCredentialsUseCase, error) {
	const defaultExpirationTime = 10 * time.Second

	builder, err := robotaccount.NewBuilder(
		robotaccount.WithPrivateKey(c.CasRobotAccountPrivateKeyPath),
		robotaccount.WithIssuer(jwt.DefaultIssuer),
		robotaccount.WithExpiration(defaultExpirationTime),
	)

	if err != nil {
		return nil, err
	}

	return &CASCredentialsUseCase{builder}, nil
}

func (uc *CASCredentialsUseCase) GenerateTemporaryCredentials(secretID string, role robotaccount.Role) (string, error) {
	return uc.jwtBuilder.GenerateJWT(secretID, jwt.CASAudience, role)
}
