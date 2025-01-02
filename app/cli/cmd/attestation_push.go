//
// Copyright 2024-2025 The Chainloop Authors.
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
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
)

func newAttestationPushCmd() *cobra.Command {
	var (
		pkPath, bundle                         string
		annotationsFlag                        []string
		signServerCAPath                       string
		signServerAuthUser, signServerAuthPass string
	)

	cmd := &cobra.Command{
		Use:   "push",
		Short: "generate and push the attestation to the control plane",
		Example: `  chainloop attestation push --key <key path>|<env://VAR_NAME> --token [chainloop-token] --annotation key=value,key2=val2

  # sign the resulting attestation using a cosign key present in the filesystem and stdin for the passphrase
  # NOTE that the --token flag can be replaced by having the CHAINLOOP_TOKEN env variable
  chainloop attestation push --key cosign.key --token [chainloop-token]

  # or retrieve the key from an environment variable containing the private key
  chainloop attestation push --key env://[ENV_VAR]

  # The passphrase can be retrieved from a well-known environment variable
  export CHAINLOOP_SIGNING_PASSWORD="my cosign key passphrase"
  chainloop attestation push --key cosign.key
  
  # You can provide values for the annotations that have previously defined in the contract for example 
  chainloop attestation push --annotation key=value --annotation key2=value2
  # Or alternatively
  chainloop attestation push --annotation key=value,key2=value2`,
		Annotations: map[string]string{
			useAPIToken: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := executableInfo()
			if err != nil {
				return fmt.Errorf("getting executable information: %w", err)
			}
			a, err := action.NewAttestationPush(&action.AttestationPushOpts{
				ActionsOpts: actionOpts, KeyPath: pkPath, BundlePath: bundle,
				CLIVersion: info.Version, CLIDigest: info.Digest,
				SignServerCAPath: signServerCAPath, LocalStatePath: attestationLocalStatePath,
			})
			if err != nil {
				return fmt.Errorf("failed to load action: %w", err)
			}

			annotations, err := extractAnnotations(annotationsFlag)
			if err != nil {
				return err
			}

			var res *action.AttestationResult
			err = runWithBackoffRetry(
				func() error {
					res, err = a.Run(cmd.Context(), attestationID, annotations)
					return err
				},
			)

			if err != nil {
				if errors.Is(err, action.ErrAttestationNotInitialized) {
					return err
				}
				if status.Code(err) == codes.Unimplemented {
					return ErrKeylessNotSupported
				}
				return newGracefulError(err)
			}

			res.Status.Digest = res.Digest

			// In TABLE format, we render the attestation status
			if err := encodeOutput(res.Status, fullStatusTable); err != nil {
				return fmt.Errorf("failed to render output: %w", err)
			}

			// We do a final check to see if the attestation has policy violations
			// and fail the command if needed
			if res.Status.HasPolicyViolations && res.Status.MustBlockOnPolicyViolations {
				return ErrBlockedByPolicyViolation
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&pkPath, "key", "k", "", "reference (path or env variable name) to the cosign or KMS key that will be used to sign the attestation")
	cmd.Flags().StringSliceVar(&annotationsFlag, "annotation", nil, "additional annotation in the format of key=value")
	cmd.Flags().StringVar(&bundle, "bundle", "", "output a Sigstore bundle to the provided path  ")
	flagAttestationID(cmd)

	cmd.Flags().StringVar(&signServerCAPath, "signserver-ca-path", "", "custom CA to be used for SignServer communications")
	cmd.Flags().StringVar(&signServerAuthUser, "signserver-auth-user", "", "")
	cmd.Flags().StringVar(&signServerAuthPass, "signserver-auth-pass", "", "")

	return cmd
}

var ErrBlockedByPolicyViolation = errors.New("the operator requires that the attestation process must have all policies passing before continue, please fix them and try again")
