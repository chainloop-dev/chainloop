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

// Authorization package
package authz

import (
	"errors"
	"fmt"

	_ "embed"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	entadapter "github.com/casbin/ent-adapter"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
)

const (
	// Actions
	ActionRead   = "read"
	ActionList   = "list"
	ActionUpdate = "update"
	ActionDelete = "delete"

	// Resources
	ResourceWorkflowContract = "workflow_contract"
	ResourceCASArtifact      = "cas_artifact"
	ResourceReferrer         = "referrer"
)

// resource, action tuple
type Policy struct {
	Resource string
	Action   string
}

var (
	PolicyWorkflowContractList   = &Policy{ResourceWorkflowContract, ActionList}
	PolicyWorkflowContractRead   = &Policy{ResourceWorkflowContract, ActionRead}
	PolicyWorkflowContractUpdate = &Policy{ResourceWorkflowContract, ActionUpdate}
	PolicyArtifactDownload       = &Policy{ResourceCASArtifact, ActionRead}
	PolicyReferrerRead           = &Policy{ResourceReferrer, ActionRead}
)

type SubjectAPIToken struct {
	ID string
}

func (t *SubjectAPIToken) String() string {
	return fmt.Sprintf("api-token:%s", t.ID)
}

//go:embed model.conf
var modelFile []byte

type Enforcer struct {
	*casbin.Enforcer
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

// Remove all the policies for the given subject
func (e *Enforcer) ClearPolicies(sub *SubjectAPIToken) error {
	if sub == nil {
		return errors.New("no subject provided")
	}

	// Get all the policies for the subject
	policies := e.GetFilteredPolicy(0, sub.String())

	if _, err := e.Enforcer.RemovePolicies(policies); err != nil {
		return fmt.Errorf("failed to remove policies: %w", err)
	}

	return nil
}

// NewDatabaseEnforcer creates a new casbin authorization enforcer
// based on a database backend as policies storage backend
func NewDatabaseEnforcer(c *conf.Data_Database) (*Enforcer, error) {
	// policy storage in database
	a, err := entadapter.NewAdapter(c.Driver, c.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	e, err := newEnforcer(a)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	return e, nil
}

// NewFileAdapter creates a new casbin authorization enforcer
// based on a CSV file as policies storage backend
func NewFiletypeEnforcer(path string) (*Enforcer, error) {
	// policy storage in filesystem
	a := fileadapter.NewAdapter(path)
	e, err := newEnforcer(a)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	return e, nil
}

// NewEnforcer creates a new casbin authorization enforcer for the policies stored
// in the database and the model defined in model.conf
func newEnforcer(a persist.Adapter) (*Enforcer, error) {
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

	return &Enforcer{enforcer}, nil
}
