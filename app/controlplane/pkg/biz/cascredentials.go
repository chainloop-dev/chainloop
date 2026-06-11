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
	"fmt"
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
	// OrgID identifies the org the CAS backend belongs to. Required for
	// every CAS JWT — managed providers (e.g. AWS-S3-ACCESS-POINT) need
	// it to scope per-tenant STS sessions, and non-managed providers
	// still carry it for audit traceability.
	OrgID uuid.UUID
	// SourceInternal flags tokens minted for the control plane's own CAS
	// client so the CAS can skip audit events for internal traffic
	SourceInternal bool
}

func (uc *CASCredentialsUseCase) GenerateTemporaryCredentials(backendRef *CASCredsOpts) (string, error) {
	if backendRef.OrgID == uuid.Nil {
		return "", fmt.Errorf("org id is required")
	}

	var opts []robotaccount.GenerateOpt
	if backendRef.SourceInternal {
		opts = append(opts, robotaccount.WithSourceInternal())
	}

	return uc.jwtBuilder.GenerateJWT(backendRef.BackendType, backendRef.SecretPath, jwt.CASAudience, backendRef.Role, backendRef.MaxBytes, backendRef.OrgID.String(), opts...)
}
