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
	"os"
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"

	"github.com/spf13/viper"
)

// resolveTraceIdentity settles the organization and project that were not fixed
// by a flag or by .chainloop.yml, prompting for them when the session is
// interactive. It updates cfg in place, marking for write-back only the values
// that differ from what the file holds.
//
// It returns an authenticated executor pinned to the resolved organization,
// which the caller reuses to create the workflow and must close. Nothing is
// written to the repository here: the caller saves only after the workflow
// exists, so an aborted or failed init leaves the repository untouched.
func resolveTraceIdentity(ctx context.Context, cfg *traceInitConfig, repoRoot string) (*action.AttestationExecutor, error) {
	// An API token is bound to its own organization server-side, so there is
	// nothing to choose and no membership list to read.
	interactive := authTokenIsUser && traceInitCanPrompt()

	if !interactive {
		if cfg.project == "" {
			return nil, errors.New("--project is required in non-interactive mode (or add projectName to .chainloop.yml)")
		}

		return openTraceExecutor(ctx, cfg.organization)
	}

	// Authenticate before asking anything: the pickers are populated from the
	// control plane, so an unauthenticated user should see the login error
	// rather than answer questions that lead to one.
	//
	// The connection starts unpinned. A .chainloop.yml pinning an organization
	// the user has since left would otherwise fail this check before they get
	// the chance to pick another one, and listing memberships is a user-scoped
	// call that ignores the organization anyway.
	executor, err := openTraceExecutor(ctx, "")
	if err != nil {
		return nil, err
	}

	// pinTo swaps the connection for one pinned to the resolved organization,
	// so the project listing and the workflow creation that follows both
	// target it. The replacement is opened before the old one is dropped, so
	// executor always names a live connection for the error path to close.
	pinTo := func(org string) (projectLister, error) {
		if org == "" {
			return executor, nil
		}

		pinned, err := openTraceExecutor(ctx, org)
		if err != nil {
			return nil, err
		}

		_ = executor.Close()
		executor = pinned

		return pinned, nil
	}

	if err := resolveIdentityInteractively(ctx, cfg, newHuhPrompter(os.LookupEnv),
		viper.GetString(confOptions.organization.viperKey), filepath.Base(repoRoot), executor, pinTo); err != nil {
		_ = executor.Close()
		return nil, err
	}

	return executor, nil
}

// resolveIdentityInteractively asks for the settings the flags left open and
// records them on cfg. pinTo is called once the organization is settled, and
// always, since the caller works on the repinned connection whether or not a
// project question follows.
//
// A value the user passed as a flag is already settled: it skips its question
// and keeps the value and the save behavior resolveTraceInitConfig gave it,
// which is what the save flags mean at this point.
func resolveIdentityInteractively(ctx context.Context, cfg *traceInitConfig, p prompter,
	currentOrg, repoDir string, orgs orgAPI, pinTo func(org string) (projectLister, error)) error {
	askOrg, askProject := !cfg.saveOrganization, !cfg.saveProject

	if askOrg {
		org, err := resolveInteractiveOrganization(ctx, orgs, p, cfg.organization, currentOrg)
		if err != nil {
			return err
		}

		if !org.prompted {
			logger.Info().Str("organization", org.value).Msg("using your only organization")
		}

		cfg.organization, cfg.saveOrganization = org.value, org.save
	}

	projects, err := pinTo(cfg.organization)
	if err != nil {
		return err
	}

	if !askProject {
		return nil
	}

	project, err := resolveInteractiveProject(ctx, projects, p, cfg.project, repoDir)
	if err != nil {
		return err
	}

	cfg.project, cfg.saveProject = project.value, project.save

	return nil
}

// openTraceExecutor dials the control plane pinned to orgName, an empty name
// keeping the default connection, and verifies the credentials work.
func openTraceExecutor(ctx context.Context, orgName string) (*action.AttestationExecutor, error) {
	executor, err := action.NewAttestationExecutor(ActionOpts, Version, action.WithForcedOrganization(orgName))
	if err != nil {
		return nil, err
	}

	if err := executor.CheckAuth(ctx); err != nil {
		_ = executor.Close()
		return nil, err
	}

	return executor, nil
}
