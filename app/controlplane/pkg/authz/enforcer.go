//
// Copyright 2025 The Chainloop Authors.
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

package authz

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

type SubjectAPIToken struct {
	ID string
}

func (t *SubjectAPIToken) String() string {
	return fmt.Sprintf("api-token:%s", t.ID)
}

//go:embed model.conf
var modelFile []byte

type Config struct {
	RolesMap map[Role][]*Policy
}

type Enforcer struct {
	*casbin.Enforcer

	config *Config
}

func (e *Enforcer) Enforce(sub string, p *Policy) (bool, error) {
	// This enforcer does not support API token subjects
	// this is due to the fact that API tokens are not stored in casbin yet
	// To use them, make sure you use the AuthzUseCase.Enforce method instead
	if strings.HasPrefix(sub, "api-token:") {
		return false, errors.New("API token subjects not supported")
	}

	return e.Enforcer.Enforce(sub, p.Resource, p.Action)
}

// NewEnforcer creates a new casbin authorization enforcer with in-memory storage.
// Only static role policies from RolesMap are loaded
func NewEnforcer(config *Config) (*Enforcer, error) {
	// load model defined in model.conf
	m, err := model.NewModelFromString(string(modelFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	// Create enforcer without a persistent adapter - policies will be stored in memory only
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	e := &Enforcer{enforcer, config}

	// Initialize the enforcer with the roles map
	if err := syncRBACRoles(e, config); err != nil {
		return nil, fmt.Errorf("failed to sync roles: %w", err)
	}

	return e, nil
}

func syncRBACRoles(e *Enforcer, c *Config) error {
	// allow to override config during sync
	conf := c
	if conf == nil {
		conf = e.config
	}

	// Add all the defined policies if they don't exist
	for role, policies := range conf.RolesMap {
		for _, p := range policies {
			// Add policies one by one to skip existing ones
			casbinPolicy := []string{string(role), p.Resource, p.Action}
			_, err := e.AddPolicy(casbinPolicy)
			if err != nil {
				return fmt.Errorf("failed to add policy: %w", err)
			}
		}
	}

	// Delete all the policies that are not in the roles map
	// 1 - load the policies from the enforcer
	policies, err := e.GetPolicy()
	if err != nil {
		return fmt.Errorf("failed to get policies: %w", err)
	}

	// clone policies, as delete operations in CasBin alters the "policies" slice
	clonedPolicies := slices.Clone(policies)

	for _, p := range clonedPolicies {
		role := p[0]
		resource := p[1]
		action := p[2]

		wantPolicies, ok := conf.RolesMap[Role(role)]
		// if the role does not exist in the map, we can delete the policy
		if !ok {
			_, err := e.RemovePolicy(role, resource, action)
			if err != nil {
				return fmt.Errorf("failed to remove policy: %w", err)
			}
			continue
		}

		// We have the role in the map, so we now compare the policies
		found := false
		for _, p := range wantPolicies {
			if p.Resource == resource && p.Action == action {
				found = true
				break
			}
		}

		// If the policy is not in the map, we remove it
		if !found {
			_, err := e.RemovePolicy(p)
			if err != nil {
				return fmt.Errorf("failed to remove policy: %w", err)
			}
		}
	}

	// To finish we make sure that the admin role inherit all the policies from the viewer role
	_, err = e.AddGroupingPolicy(string(RoleAdmin), string(RoleViewer))
	if err != nil {
		return fmt.Errorf("failed to add grouping policy: %w", err)
	}

	// same for the owner
	_, err = e.AddGroupingPolicy(string(RoleOwner), string(RoleAdmin))
	if err != nil {
		return fmt.Errorf("failed to add grouping policy: %w", err)
	}

	// Members are contributors as well
	_, err = e.AddGroupingPolicy(string(RoleOrgMember), string(RoleOrgContributor))
	if err != nil {
		return fmt.Errorf("failed to add grouping policy: %w", err)
	}

	// ProjectAdmins are ProjectViewers as well
	_, err = e.AddGroupingPolicy(string(RoleProjectAdmin), string(RoleProjectViewer))
	if err != nil {
		return fmt.Errorf("failed to add grouping policy: %w", err)
	}

	return nil
}
