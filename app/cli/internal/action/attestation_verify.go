//
// Copyright 2025 The Chainloop Authors.
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
	"github.com/sigstore/cosign/v2/pkg/blob"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AttestationVerifyAction struct {
	cfg *ActionsOpts
}

func NewAttestationVerifyAction(cfg *ActionsOpts) *AttestationVerifyAction {
	return &AttestationVerifyAction{cfg}
}

func (action *AttestationVerifyAction) Run(ctx context.Context, fileOrURL string) (bool, error) {
	content, err := blob.LoadFileOrURL(fileOrURL)
	if err != nil {
		return false, fmt.Errorf("loading attestation: %w", err)
	}

	return verifyBundle(ctx, content, action.cfg)
}

func verifyBundle(ctx context.Context, content []byte, opts *ActionsOpts) (bool, error) {
	sc := pb.NewSigningServiceClient(opts.CPConnection)
	trResp, err := sc.GetTrustedRoot(ctx, &pb.GetTrustedRootRequest{})
	if err != nil {
		// if trusted root is not implemented, skip verification
		if status.Code(err) != codes.Unimplemented {
			return false, fmt.Errorf("failed getting trusted root: %w", err)
		}
	}

	if trResp != nil {
		tr, err := trustedRootPbToVerifier(trResp)
		if err != nil {
			return false, fmt.Errorf("getting roots: %w", err)
		}
		if err = verifier.VerifyBundle(ctx, content, tr); err != nil {
			if !errors.Is(err, verifier.ErrMissingVerificationMaterial) {
				opts.Logger.Debug().Err(err).Msg("bundle verification failed")
				return false, errors.New("bundle verification failed")
			}
		} else {
			return true, nil
		}
	}

	return false, nil
}
