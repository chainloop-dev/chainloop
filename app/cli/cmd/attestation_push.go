//
// Copyright 2024-2026 The Chainloop Authors.
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
	"bytes"
	"errors"
	"fmt"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
)

func newAttestationPushCmd() *cobra.Command {
	var (
		pkPath, bundle   string
		annotationsFlag  []string
		signServerCAPath string
		// Client certificate and passphrase for SignServer auth
		signServerAuthCertPath string
		signServerAuthCertPass string
		bypassPolicyCheck      bool
		deactivateCIReport     bool
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
				ActionsOpts: ActionOpts, KeyPath: pkPath, BundlePath: bundle,
				CLIVersion: info.Version, CLIDigest: info.Digest,
				LocalStatePath: attestationLocalStatePath,
				SignServerOpts: &action.SignServerOpts{
					CAPath:             signServerCAPath,
					AuthClientCertPath: signServerAuthCertPath,
					AuthClientCertPass: signServerAuthCertPass,
				},
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
					res, err = a.Run(cmd.Context(), attestationID, annotations, bypassPolicyCheck)
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

			// Render the attestation status to a string
			buf := &bytes.Buffer{}
			if err := fullStatusTableWithWriter(res.Status, buf); err != nil {
				return fmt.Errorf("failed to render output: %w", err)
			}

			res.Status.TerminalOutput = buf.Bytes()

			// Report to CI/CD platform if supported and not disabled
			if !deactivateCIReport && !res.Status.DryRun && res.Status.RunnerContext != nil {
				// Clean up possible ANSI characters from the output
				sanitizedOutput := removeAnsiCharactersFromBytes(res.Status.TerminalOutput)
				if err := res.Status.RunnerContext.RawRunner.Report(sanitizedOutput, res.Status.AttestationViewURL); err != nil {
					logger.Warn().Err(err).Msg("failed to write CI/CD platform report")
				} else {
					// Log success message based on runner type
					switch res.Status.RunnerContext.RawRunner.ID() {
					case schemaapi.CraftingSchema_Runner_GITHUB_ACTION:
						logger.Info().Msg("attestation report written to GitHub step summary")
					case schemaapi.CraftingSchema_Runner_GITLAB_PIPELINE:
						logger.Info().Msg("attestation report written to chainloop-attestation-report.txt artifact")
					}
				}
			}

			// In TABLE format, we render the attestation status
			if err := output.EncodeOutput(flagOutputFormat, res.Status, fullStatusTable); err != nil {
				return fmt.Errorf("failed to render output: %w", err)
			}

			if err := validatePolicyEnforcement(res.Status, bypassPolicyCheck); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&pkPath, "key", "k", "", "reference (path or env variable name) to the cosign or KMS key that will be used to sign the attestation")
	cmd.Flags().StringSliceVar(&annotationsFlag, "annotation", nil, "additional annotation in the format of key=value")
	cmd.Flags().StringVar(&bundle, "bundle", "", "output a Sigstore bundle to the provided path  ")
	flagAttestationID(cmd)

	cmd.Flags().StringVar(&signServerCAPath, "signserver-ca-path", "", "custom CA to be used for SignServer TLS connection")
	cmd.Flags().StringVar(&signServerAuthCertPath, "signserver-client-cert", "", "path to client certificate in PEM format for authenticated SignServer TLS connection")
	cmd.Flags().StringVar(&signServerAuthCertPass, "signserver-client-pass", "", "certificate passphrase for authenticated SignServer TLS connection")
	cmd.Flags().BoolVar(&bypassPolicyCheck, exceptionFlagName, false, "do not fail this command on policy violations enforcement")
	cmd.Flags().BoolVar(&deactivateCIReport, "deactivate-ci-report", false, "deactivate automatic attestation report to CI/CD platform")

	return cmd
}

const exceptionFlagName = "exception-bypass-policy-check"

var (
	ErrBlockedByPolicyViolation = fmt.Errorf("the operator requires all policies to pass before continuing, please fix them and try again or temporarily bypass the policy check using --%s", exceptionFlagName)
	exceptionBypassPolicyCheck  = fmt.Sprintf("Attention: You have opted to bypass the policy enforcement check and an operator has been notified of this exception.\nPlease make sure you are back on track with the policy evaluations and remove the --%s as soon as possible.", exceptionFlagName)
)

type GateError struct {
	PolicyName string
}

func NewGateError(policyName string) error {
	return &GateError{PolicyName: policyName}
}

func (e *GateError) Error() string {
	return fmt.Sprintf("the policy %q is configured as a gate and has violations", e.PolicyName)
}

func validatePolicyEnforcement(status *action.AttestationStatusResult, bypassPolicyCheck bool) error {
	// Block if any of the policies has been configured as a gate.
	for _, evaluations := range status.PolicyEvaluations {
		for _, eval := range evaluations {
			if len(eval.Violations) > 0 && eval.Gate {
				if bypassPolicyCheck {
					logger.Warn().Msg(exceptionBypassPolicyCheck)
					continue
				}

				return NewGateError(eval.Name)
			}
		}
	}

	// Do a final check in case the operator has configured the attestation
	// to be blocked on any policy violation.
	if status.MustBlockOnPolicyViolations {
		if bypassPolicyCheck {
			logger.Warn().Msg(exceptionBypassPolicyCheck)
			return nil
		}

		if status.HasPolicyViolations {
			return ErrBlockedByPolicyViolation
		}
	}

	return nil
}
