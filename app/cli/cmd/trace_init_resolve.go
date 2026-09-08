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
	"golang.org/x/term"
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
	interactive := authTokenIsUser && isInteractive(os.LookupEnv,
		term.IsTerminal(int(os.Stdin.Fd())), term.IsTerminal(int(os.Stderr.Fd())))

	if !interactive {
		if cfg.project == "" {
			return nil, errors.New("--project is required in non-interactive mode (or add projectName to .chainloop.yml)")
		}

		return openTraceExecutor(ctx, cfg.organization)
	}

	// Authenticate before asking anything: the pickers are populated from the
	// control plane, so an unauthenticated user should see the login error
	// rather than answer questions that lead to one.
	executor, err := openTraceExecutor(ctx, cfg.organization)
	if err != nil {
		return nil, err
	}

	prompt := newHuhPrompter(os.LookupEnv)

	// The organization step runs on this connection whatever it is pinned to:
	// listing memberships is a user-scoped call that ignores the organization.
	org, err := resolveInteractiveOrganization(ctx, executor, prompt,
		cfg.organization, viper.GetString(confOptions.organization.viperKey))
	if err != nil {
		_ = executor.Close()
		return nil, err
	}

	if !org.prompted {
		logger.Info().Str("organization", org.value).Msg("using your only organization")
	}

	// Reopen pinned to the chosen organization so the project listing and the
	// workflow creation that follows both target it.
	if org.value != cfg.organization {
		_ = executor.Close()
		if executor, err = openTraceExecutor(ctx, org.value); err != nil {
			return nil, err
		}
	}

	cfg.organization, cfg.saveOrganization = org.value, org.save

	project, err := resolveInteractiveProject(ctx, executor, prompt,
		cfg.project, filepath.Base(repoRoot))
	if err != nil {
		_ = executor.Close()
		return nil, err
	}

	cfg.project, cfg.saveProject = project.value, project.save

	return executor, nil
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
