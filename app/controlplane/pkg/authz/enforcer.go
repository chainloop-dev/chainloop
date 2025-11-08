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
	"fmt"
	"slices"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
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
	ManagedResources    []string
	RolesMap            map[Role][]*Policy
	RestrictOrgCreation bool
}

type Enforcer struct {
	*casbin.Enforcer

	config              *Config
	RestrictOrgCreation bool
}

func (e *Enforcer) Enforce(sub string, p *Policy) (bool, error) {
	return e.Enforcer.Enforce(sub, p.Resource, p.Action)
}

// EnforceWithPolicies checks if the required policy exists in the provided list of allowed policies.
// This is used for ACL-based authorization (e.g., API tokens) where policies are stored in the database
// rather than in Casbin. Returns true if the required policy is found in the allowed list.
func (e *Enforcer) EnforceWithPolicies(sub string, p *Policy, allowedPolicies []*Policy) (bool, error) {
	for _, allowed := range allowedPolicies {
		if allowed.Resource == p.Resource && allowed.Action == p.Action {
			return true, nil
		}
	}
	return false, nil
}

// NewInMemoryEnforcer creates a new casbin authorization enforcer with in-memory storage.
// Only static role policies from RolesMap are loaded. API token policies are checked separately
// using EnforceWithPolicies and are not stored in Casbin.
func NewInMemoryEnforcer(config *Config) (*Enforcer, error) {
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

	e := &Enforcer{enforcer, config, config.RestrictOrgCreation}

	// Initialize the enforcer with the roles map
	if err := syncRBACRoles(e, config); err != nil {
		return nil, fmt.Errorf("failed to sync roles: %w", err)
	}

	return e, nil
}

// NewFileAdapter creates a new casbin authorization enforcer
// based on a CSV file as policies storage backend
func NewFiletypeEnforcer(path string, config *Config) (*Enforcer, error) {
	// policy storage in filesystem
	a := fileadapter.NewAdapter(path)
	e, err := newEnforcer(a, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	return e, nil
}

// NewEnforcer creates a new casbin authorization enforcer for the policies stored
// in the database and the model defined in model.conf
func newEnforcer(a persist.Adapter, config *Config) (*Enforcer, error) {
	// load model defined in model.conf
	m, err := model.NewModelFromString(string(modelFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	// create enforcer for authorization
	enforcer, err := casbin.NewEnforcer(m, a)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	e := &Enforcer{enforcer, config, config.RestrictOrgCreation}

	// Initialize the enforcer with the roles map
	if err := syncRBACRoles(e, config); err != nil {
		return nil, fmt.Errorf("failed to sync roles: %w", err)
	}

	return e, nil
}

// Load the roles map into the enforcer
// This is done by adding all the policies defined in the roles map
// and removing all the policies that are not
func syncRBACRoles(e *Enforcer, config *Config) error {
	return doSync(e, config)
}

func doSync(e *Enforcer, c *Config) error {
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

		// if it's not a managed resource, skip deletion
		if !slices.Contains(conf.ManagedResources, resource) {
			continue
		}

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
