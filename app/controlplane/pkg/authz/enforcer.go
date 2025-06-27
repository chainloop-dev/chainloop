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
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"

	psqlwatcher "github.com/IguteChung/casbin-psql-watcher"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	entadapter "github.com/casbin/ent-adapter"
	config "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
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
	ManagedResources []string
	RolesMap         map[Role][]*Policy
}

type Enforcer struct {
	*casbin.Enforcer

	config *Config
}

func (e *Enforcer) AddPolicies(sub *SubjectAPIToken, policies ...*Policy) error {
	if len(policies) == 0 {
		return errors.New("no policies to add")
	}

	if sub == nil {
		return errors.New("no subject provided")
	}

	for _, p := range policies {
		casbinPolicy := []string{sub.String(), p.Resource, p.Action}
		// Add policies one by one to skip existing ones.
		// This is because the bulk method AddPoliciesEx does not work well with the ent adapter
		if _, err := e.AddPolicy(casbinPolicy); err != nil {
			return fmt.Errorf("failed to add policy: %w", err)
		}
	}

	return nil
}

func (e *Enforcer) Enforce(sub string, p *Policy) (bool, error) {
	return e.Enforcer.Enforce(sub, p.Resource, p.Action)
}

// Remove all the policies for the given subject
func (e *Enforcer) ClearPolicies(sub *SubjectAPIToken) error {
	if sub == nil {
		return errors.New("no subject provided")
	}

	// Get all the policies for the subject
	policies, err := e.GetFilteredPolicy(0, sub.String())
	if err != nil {
		return fmt.Errorf("failed to get policies: %w", err)
	}

	if _, err := e.RemovePolicies(policies); err != nil {
		return fmt.Errorf("failed to remove policies: %w", err)
	}

	return nil
}

// NewDatabaseEnforcer creates a new casbin authorization enforcer
// based on a database backend as policies storage backend
func NewDatabaseEnforcer(c *config.DatabaseConfig, config *Config) (*Enforcer, error) {
	// policy storage in database
	a, err := entadapter.NewAdapter(c.Driver, c.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	e, err := newEnforcer(a, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	// watch for policy changes in database and update enforcer
	w, err := psqlwatcher.NewWatcherWithConnString(context.Background(), c.Source, psqlwatcher.Option{})
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	if err = e.SetWatcher(w); err != nil {
		return nil, fmt.Errorf("failed to set watcher: %w", err)
	}

	if err = w.SetUpdateCallback(func(string) {
		// When there is a change in the policy, we load the in-memory policy for the current enforcer
		if err := e.LoadPolicy(); err != nil {
			fmt.Printf("failed to load policy: %v", err)
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to set update callback: %w", err)
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

	e := &Enforcer{enforcer, config}

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
			// Add policies one by one to skip existing ones.
			// This is because the bulk method AddPoliciesEx does not work well with the ent adapter
			casbinPolicy := []string{string(role), p.Resource, p.Action}
			_, err := e.AddPolicy(casbinPolicy)
			if err != nil {
				return fmt.Errorf("failed to add policy: %w", err)
			}
		}
	}

	// Delete all the policies that are not in the roles map
	// 1 - load the policies from the enforcer DB
	policies, err := e.GetPolicy()
	if err != nil {
		return fmt.Errorf("failed to get policies: %w", err)
	}

	for _, gotPolicies := range policies {
		role := gotPolicies[0]
		resource := gotPolicies[1]
		action := gotPolicies[2]
		policy := &Policy{Resource: resource, Action: action}

		// if it's not a managed resource, skip deletion
		if !slices.Contains(conf.ManagedResources, resource) {
			continue
		}

		wantPolicies, ok := conf.RolesMap[Role(role)]
		// if the role does not exist in the map, we can delete the policy
		if !ok {
			_, err := e.RemovePolicy(role, policy.Resource, policy.Action)
			if err != nil {
				return fmt.Errorf("failed to remove policy: %w", err)
			}
			continue
		}

		// We have the role in the map, so we now compare the policies
		found := false
		for _, p := range wantPolicies {
			if p.Resource == policy.Resource && p.Action == policy.Action {
				found = true
				break
			}
		}

		// If the policy is not in the map, we remove it
		if !found {
			_, err := e.RemovePolicy(gotPolicies)
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

	return nil
}
