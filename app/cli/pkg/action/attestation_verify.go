//
// Copyright 2025-2026 The Chainloop Authors.
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

package action

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/verifier"
	"github.com/sigstore/cosign/v3/pkg/blob"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrAttestationNotVerifiable indicates that the attestation's authenticity could not be
// established at all, either because the bundle carries no verification material or because
// the control plane has no trusted root to verify it against. It is a verification failure:
// a bundle that was never verified must never be reported as verified.
var ErrAttestationNotVerifiable = errors.New("attestation could not be verified")

type AttestationVerifyAction struct {
	cfg *ActionsOpts
}

func NewAttestationVerifyAction(cfg *ActionsOpts) *AttestationVerifyAction {
	return &AttestationVerifyAction{cfg}
}

// Run verifies the given attestation bundle and returns an error if it can not be
// verified, so callers gating on the exit code never take an unverified bundle as good.
func (action *AttestationVerifyAction) Run(ctx context.Context, fileOrURL string) error {
	content, err := blob.LoadFileOrURL(fileOrURL)
	if err != nil {
		return fmt.Errorf("loading attestation: %w", err)
	}

	return verifyBundleOrFail(ctx, content, action.cfg)
}

// verifyBundleOrFail verifies the bundle against the trusted root configured in the control
// plane. Any outcome other than a successful verification is returned as an error.
func verifyBundleOrFail(ctx context.Context, content []byte, opts *ActionsOpts) error {
	sc := pb.NewSigningServiceClient(opts.CPConnection)
	trResp, err := sc.GetTrustedRoot(ctx, &pb.GetTrustedRootRequest{})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return fmt.Errorf("%w: the control plane has no trusted root configured", ErrAttestationNotVerifiable)
		}

		return fmt.Errorf("failed getting trusted root: %w", err)
	}

	tr, err := trustedRootPbToVerifier(trResp)
	if err != nil {
		return fmt.Errorf("getting roots: %w", err)
	}

	if err := verifier.VerifyBundle(ctx, content, tr); err != nil {
		if errors.Is(err, verifier.ErrMissingVerificationMaterial) {
			return fmt.Errorf("%w: the bundle does not contain verification material", ErrAttestationNotVerifiable)
		}

		opts.Logger.Debug().Err(err).Msg("bundle verification failed")
		return errors.New("bundle verification failed")
	}

	return nil
}

// verifyBundle reports whether the bundle could be verified. Unlike verifyBundleOrFail, a
// bundle that is not verifiable is not an error, it is simply reported as not verified.
func verifyBundle(ctx context.Context, content []byte, opts *ActionsOpts) (bool, error) {
	if err := verifyBundleOrFail(ctx, content, opts); err != nil {
		if errors.Is(err, ErrAttestationNotVerifiable) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
