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

package cmd

import (
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/s3"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/cobra"
)

func newCASBackendAddAWSS3Cmd() *cobra.Command {
	var bucketName, accessKeyID, secretAccessKey, region, endpoint string
	cmd := &cobra.Command{
		Use:   "aws-s3",
		Short: "Register a AWS S3 storage bucket",
		RunE: func(cmd *cobra.Command, args []string) error {
			isDefault, err := cmd.Flags().GetBool("default")
			cobra.CheckErr(err)

			name, err := cmd.Flags().GetString("name")
			cobra.CheckErr(err)

			description, err := cmd.Flags().GetString("description")
			cobra.CheckErr(err)

			if isDefault {
				if confirmed, err := confirmDefaultCASBackendOverride(actionOpts, ""); err != nil {
					return err
				} else if !confirmed {
					log.Info("Aborting...")
					return nil
				}
			}

			location := bucketName
			// If there is a custom endpoint we want to store it as part of the fqdn location
			if endpoint != "" {
				location = fmt.Sprintf("%s/%s", endpoint, bucketName)
			}

			opts := &action.NewCASBackendAddOpts{
				Name:        name,
				Location:    location,
				Provider:    s3.ProviderID,
				Description: description,
				Credentials: map[string]any{
					"accessKeyID":     accessKeyID,
					"secretAccessKey": secretAccessKey,
					"region":          region,
				},
				Default: isDefault,
			}

			res, err := action.NewCASBackendAdd(actionOpts).Run(opts)
			if err != nil {
				return err
			} else if res == nil {
				return nil
			}

			return encodeOutput(res, casBackendItemTableOutput)
		},
	}

	cmd.Flags().StringVar(&bucketName, "bucket", "", "S3 bucket name")
	err := cmd.MarkFlagRequired("bucket")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&accessKeyID, "access-key-id", "", "AWS Access Key ID")
	err = cmd.MarkFlagRequired("access-key-id")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&secretAccessKey, "secret-access-key", "", "AWS Secret Access Key")
	err = cmd.MarkFlagRequired("secret-access-key")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&region, "region", "", "AWS region for the bucket")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Custom Endpoint URL for other S3 compatible backends i.e MinIO")

	return cmd
}
