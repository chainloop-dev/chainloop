//
// Copyright 2024-2026 The Chainloop Authors.
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

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt"
	robotaccount "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/google/uuid"
)

type CASCredentialsUseCase struct {
	jwtBuilder *robotaccount.Builder
}

func NewCASCredentialsUseCase(c *conf.Auth) (*CASCredentialsUseCase, error) {
	const defaultExpirationTime = 30 * time.Second

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

type CASCredsOpts struct {
	BackendType string // i.e OCI, S3
	SecretPath  string // path to for example the OCI secret in the vault
	Role        robotaccount.Role
	MaxBytes    int64
	// OrgID is the org the CAS backend belongs to. Required for managed
	// backends (e.g. AWS-S3-ACCESS-POINT) that need to scope per-tenant
	// STS sessions; uuid.Nil is treated as "absent" for the others
	// (OCI, S3, AzureBlob).
	OrgID uuid.UUID
}

func (uc *CASCredentialsUseCase) GenerateTemporaryCredentials(backendRef *CASCredsOpts) (string, error) {
	var orgID string
	if backendRef.OrgID != uuid.Nil {
		orgID = backendRef.OrgID.String()
	}
	return uc.jwtBuilder.GenerateJWT(backendRef.BackendType, backendRef.SecretPath, jwt.CASAudience, backendRef.Role, backendRef.MaxBytes, orgID)
}
