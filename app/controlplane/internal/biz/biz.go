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
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/wire"
	"github.com/moby/moby/pkg/namesgenerator"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewWorkflowUsecase,
	NewUserUseCase,
	NewRootAccountUseCase,
	NewWorkflowRunUseCase,
	NewOrganizationUseCase,
	NewWorkflowContractUseCase,
	NewCASCredentialsUseCase,
	NewCASBackendUseCase,
	NewOrgMetricsUseCase,
	NewIntegrationUseCase,
	NewMembershipUseCase,
	NewCASClientUseCase,
	NewOrgInvitationUseCase,
	NewAttestationUseCase,
	NewWorkflowRunExpirerUseCase,
	NewCASMappingUseCase,
	NewReferrerUseCase,
	NewAPITokenUseCase,
	NewAPITokenSyncerUseCase,
	NewAttestationStateUseCase,
	wire.Struct(new(NewIntegrationUseCaseOpts), "*"),
	wire.Struct(new(NewUserUseCaseParams), "*"),
)

// generate a DNS1123-valid random name using moby's namesgenerator
// plus an additional random number
func generateRandomName() (string, error) {
	// Create a random name
	name := namesgenerator.GetRandomName(0)
	// and append a random number to it
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(1000))
	if err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}

	// Replace underscores with dashes to make it compatible with DNS1123
	name = strings.ReplaceAll(fmt.Sprintf("%s-%d", name, randomNumber), "_", "-")
	return name, nil
}
