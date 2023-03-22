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

package deptrack

import (
	"context"
	"fmt"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/internal/credentials"
)

type Integration struct {
	integrationUseCase *biz.IntegrationUseCase
	credsW             credentials.Writer
}

const Kind = "Dependency-Track"

func New(integrationUC *biz.IntegrationUseCase, cw credentials.Writer) *Integration {
	return &Integration{integrationUC, cw}
}

func (uc *Integration) Add(ctx context.Context, orgID, host, apiKey string, enableProjectCreation bool) (*biz.Integration, error) {
	// Validate Credentials before saving them
	creds := &credentials.APICreds{Host: host, Key: apiKey}
	if err := creds.Validate(); err != nil {
		return nil, biz.NewErrValidation(err)
	}

	// Create the secret in the external secrets manager
	secretID, err := uc.credsW.SaveCredentials(ctx, orgID, creds)
	if err != nil {
		return nil, fmt.Errorf("storing the credentials: %w", err)
	}

	c := &v1.IntegrationConfig{
		Config: &v1.IntegrationConfig_DependencyTrack_{
			DependencyTrack: &v1.IntegrationConfig_DependencyTrack{
				AllowAutoCreate: enableProjectCreation, Domain: host,
			},
		},
	}

	// Persist data
	return uc.integrationUseCase.Create(ctx, orgID, Kind, secretID, c)
}
