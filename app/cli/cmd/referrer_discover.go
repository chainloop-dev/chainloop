//
// Copyright 2023-2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/cli/cmd/options"
	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newReferrerDiscoverCmd() *cobra.Command {
	var digest, kind string
	paginationOpts := &options.PaginationOpts{DefaultLimit: 20}

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "(Preview) inspect pieces of evidence or artifacts stored through Chainloop",
		Long: `(Preview) inspect pieces of evidence or artifacts stored through Chainloop.

When logged in as a user, the discovery covers every organization you are a member of.
Set the organization explicitly with --org (or its environment variable) to search only that organization.
An API token always searches only its own organization.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pagination := &action.PaginationOpts{
				Limit:      paginationOpts.Limit,
				NextCursor: paginationOpts.NextCursor,
			}

			res, err := action.NewReferrerDiscoverPrivate(ActionOpts,
				// An explicitly selected organization scopes the discovery to it
				action.WithCurrentOrgOnly(organizationExplicitlySelected(cmd)),
			).Run(context.Background(), digest, kind, pagination)
			if err != nil {
				return err
			}

			// NOTE: this is a preview/beta command, for now we only return JSON format
			return output.EncodeJSON(res)
		},
	}

	cmd.Flags().StringVarP(&digest, "digest", "d", "", "hash of the attestation, piece of evidence or artifact, i.e sha256:deadbeef")
	err := cmd.MarkFlagRequired("digest")
	cobra.CheckErr(err)
	cmd.Flags().StringVarP(&kind, "kind", "k", "", "optional kind of the referrer, used to disambiguate between multiple referrers with the same digest")
	cobra.CheckErr(err)
	paginationOpts.AddFlags(cmd)

	return cmd
}
