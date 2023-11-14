//
// Copyright 2023 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newReferrerDiscoverCmd() *cobra.Command {
	var digest, kind string

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "(Preview) inspect pieces of evidence or artifacts stored through Chainloop",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewReferrerDiscover(actionOpts).Run(context.Background(), digest, kind)
			if err != nil {
				return err
			}

			// NOTE: this is a preview/beta command, for now we only return JSON format
			return encodeJSON(res)
		},
	}

	cmd.Flags().StringVarP(&digest, "digest", "d", "", "hash of the attestation, piece of evidence or artifact, i.e sha256:deadbeef")
	err := cmd.MarkFlagRequired("digest")
	cobra.CheckErr(err)
	cmd.Flags().StringVarP(&kind, "kind", "k", "", "optional kind of the referrer, used to disambiguate between multiple referrers with the same digest")

	return cmd
}
