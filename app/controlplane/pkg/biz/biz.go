//
// Copyright 2024 The Chainloop Authors.
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
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/wire"
	"k8s.io/apimachinery/pkg/util/validation"
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
	NewChainloopSigningUseCase,
	wire.Struct(new(NewIntegrationUseCaseOpts), "*"),
	wire.Struct(new(NewUserUseCaseParams), "*"),
)

// generate a DNS1123-valid random name using moby's namesgenerator
// plus an additional random number
func generateValidDNS1123WithSuffix(prefix string) (string, error) {
	// Append a random number to it
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}

	// Replace underscores with dashes to make it compatible with DNS1123
	name := strings.ReplaceAll(fmt.Sprintf("%s-%d", prefix, randomNumber), "_", "-")

	if err := ValidateIsDNS1123(name); err != nil {
		return "", fmt.Errorf("generated name is not DNS1123-valid: %w", err)
	}

	return name, nil
}

func ValidateIsDNS1123(name string) error {
	// The same validation done by Kubernetes for their namespace name
	// https://github.com/kubernetes/apimachinery/blob/fa98d6eaedb4caccd69fc07d90bbb6a1e551f00f/pkg/api/validation/generic.go#L63
	err := validation.IsDNS1123Label(name)
	if len(err) > 0 {
		errMsg := ""
		for _, e := range err {
			errMsg += fmt.Sprintf("%q: %s\n", name, e)
		}

		return errors.New(errMsg)
	}

	return nil
}
