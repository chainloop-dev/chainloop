//
// Copyright 2024-2025 The Chainloop Authors.
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
	"regexp"
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
	NewPrometheusUseCase,
	NewProjectVersionUseCase,
	NewProjectsUseCase,
	NewAuditorUseCase,
	NewUserAccessSyncerUseCase,
	wire.Bind(new(PromObservable), new(*PrometheusUseCase)),
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

		return NewErrValidationStr(errMsg)
	}

	return nil
}

// ValidateVersion validates that the provided version string is in a valid format.
// The version string must match the following regular expression: ^[a-zA-Z0-9.\-]+$
// This ensures the version only contains alphanumeric characters, dots, and hyphens.
func ValidateVersion(version string) error {
	// Basic regex check (example: allow alphanumeric, dots, hyphens, underscores, plus signs, and build metadata)
	regex := regexp.MustCompile(`^[a-zA-Z0-9.\-_+]+(?:\+[a-zA-Z0-9.\-_]+)?$`)
	if !regex.MatchString(version) {
		return NewErrValidationStr(fmt.Sprintf("invalid version format: %s. Valid examples: '1.0.0', 'v2.1-alpha', '3.0.0+build.123', '2024.3.12', 'v1.0_beta'", version))
	}

	return nil
}

// EntityRef is a reference to an entity
type EntityRef struct {
	// ID is the unique identifier of the entity
	ID string
	// Name is the name of the entity
	Name string
}

func ToPtr[T any](v T) *T {
	return &v
}
