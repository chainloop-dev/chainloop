//
// Copyright 2026 The Chainloop Authors.
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

package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// identityStepsFixture wires resolveIdentityInteractively to fakes and records
// which organization the connection was repinned to.
type identityStepsFixture struct {
	orgs   *fakeOrgLister
	prompt *fakePrompter

	// defaultProjects is what a pinned connection lists, unless projectsByOrg
	// names that organization. Giving each organization its own projects is what
	// lets a test tell which connection the project step actually ran on.
	defaultProjects []string
	projectsByOrg   map[string][]string

	pinnedTo string
	pinCalls int
	pinErr   error
}

func (f *identityStepsFixture) pinTo(org string) (projectLister, error) {
	f.pinCalls++
	f.pinnedTo = org
	if f.pinErr != nil {
		return nil, f.pinErr
	}

	projects, ok := f.projectsByOrg[org]
	if !ok {
		projects = f.defaultProjects
	}

	// A fresh lister per pin, so a stale one cannot answer for the new
	// organization without the assertions noticing.
	return &fakeProjectLister{projects: projects}, nil
}

func newIdentityStepsFixture(orgs []*action.MembershipItem, projects []string, p *fakePrompter) *identityStepsFixture {
	return &identityStepsFixture{
		orgs:            &fakeOrgLister{orgs: orgs},
		defaultProjects: projects,
		prompt:          p,
	}
}

func (f *identityStepsFixture) run(cfg *traceInitConfig) error {
	return resolveIdentityInteractively(context.Background(), cfg, f.prompt, orgAcme, repoDirMyRepo, f.orgs, f.pinTo)
}

// TestResolveIdentityInteractivelyFlags pins down that a value the user passed
// explicitly is never second-guessed: no question is asked for it, its value
// survives, and so does the save flag resolveTraceInitConfig set.
func TestResolveIdentityInteractivelyFlags(t *testing.T) {
	logger = zerolog.Nop()

	orgs := []*action.MembershipItem{membership(orgAcme, true), membership(orgGlobex, false)}
	projects := []string{projectAPI, "payments"}

	t.Run("--org skips the organization question and stays saved", func(t *testing.T) {
		// saveOrganization is what resolveTraceInitConfig sets for an explicit --org.
		cfg := &traceInitConfig{organization: "flag-org", saveOrganization: true, project: "yml-project"}
		f := newIdentityStepsFixture(orgs, projects, &fakePrompter{selectAnswer: projectAPI})

		require.NoError(t, f.run(cfg))
		assert.Equal(t, "flag-org", cfg.organization)
		assert.True(t, cfg.saveOrganization, "an explicit --org must still be written to .chainloop.yml")
		assert.Equal(t, "flag-org", f.pinnedTo, "the connection must be pinned to the flag's organization")
		// Only the project question ran.
		require.Len(t, f.prompt.selects, 1)
		assert.Contains(t, f.prompt.selects[0].title, "project")
	})

	t.Run("--project skips the project question and stays saved", func(t *testing.T) {
		cfg := &traceInitConfig{project: "flag-project", saveProject: true}
		f := newIdentityStepsFixture(orgs, projects, &fakePrompter{selectAnswer: orgGlobex})

		require.NoError(t, f.run(cfg))
		assert.Equal(t, "flag-project", cfg.project)
		assert.True(t, cfg.saveProject, "an explicit --project must still be written to .chainloop.yml")
		// Only the organization question ran.
		require.Len(t, f.prompt.selects, 1)
		assert.Contains(t, f.prompt.selects[0].title, "organization")
		assert.Empty(t, f.prompt.inputs)
	})

	t.Run("both flags ask nothing at all", func(t *testing.T) {
		cfg := &traceInitConfig{
			organization: "flag-org", saveOrganization: true,
			project: "flag-project", saveProject: true,
		}
		f := newIdentityStepsFixture(orgs, projects, &fakePrompter{})

		require.NoError(t, f.run(cfg))
		assert.Empty(t, f.prompt.selects)
		assert.Empty(t, f.prompt.inputs)
		assert.Equal(t, "flag-org", cfg.organization)
		assert.Equal(t, "flag-project", cfg.project)
		assert.True(t, cfg.saveOrganization)
		assert.True(t, cfg.saveProject)
	})

	t.Run("with no flags both questions are asked", func(t *testing.T) {
		cfg := &traceInitConfig{}
		f := newIdentityStepsFixture(orgs, projects, &fakePrompter{selectAnswer: orgGlobex})

		require.NoError(t, f.run(cfg))
		// The organization picker, then the project picker.
		require.Len(t, f.prompt.selects, 2)
		assert.Contains(t, f.prompt.selects[0].title, "organization")
		assert.Contains(t, f.prompt.selects[1].title, "project")
		assert.Equal(t, orgGlobex, cfg.organization)
		assert.True(t, cfg.saveOrganization)
	})
}

func TestResolveIdentityInteractivelyPinning(t *testing.T) {
	logger = zerolog.Nop()

	orgs := []*action.MembershipItem{membership(orgAcme, true), membership(orgGlobex, false)}

	t.Run("the project listing runs on the chosen organization", func(t *testing.T) {
		cfg := &traceInitConfig{}
		f := newIdentityStepsFixture(orgs, nil, &fakePrompter{selectAnswer: orgGlobex})
		// Each organization owns a distinct project, so the options the picker
		// was given say which connection listed them. Without this the assertion
		// would hold even if the project step used a stale connection.
		f.projectsByOrg = map[string][]string{
			orgAcme:   {"acme-only"},
			orgGlobex: {"globex-only"},
		}

		require.NoError(t, f.run(cfg))
		assert.Equal(t, orgGlobex, f.pinnedTo, "pinning must happen after the organization is picked")
		assert.Equal(t, 1, f.pinCalls)

		require.Len(t, f.prompt.selects, 2)
		assert.Contains(t, f.prompt.selects[1].options, "globex-only", "the project step must list the chosen organization")
		assert.NotContains(t, f.prompt.selects[1].options, "acme-only", "it must not list the organization left behind")
	})

	t.Run("a pinning failure stops before the project question", func(t *testing.T) {
		cfg := &traceInitConfig{}
		f := newIdentityStepsFixture(orgs, []string{projectAPI}, &fakePrompter{selectAnswer: orgGlobex})
		f.pinErr = errors.New("dial failed")

		err := f.run(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dial failed")
		// The organization question ran, the project one did not.
		assert.Len(t, f.prompt.selects, 1)
	})

	t.Run("an aborted question stops before pinning", func(t *testing.T) {
		cfg := &traceInitConfig{}
		f := newIdentityStepsFixture(orgs, nil, &fakePrompter{selectErr: errAborted})

		require.ErrorIs(t, f.run(cfg), errAborted)
		assert.Zero(t, f.pinCalls)
	})
}
