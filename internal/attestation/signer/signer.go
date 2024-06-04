package signer

import (
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/signer/chainloop"
	clsigner "github.com/chainloop-dev/chainloop/internal/attestation/signer/cosign"
	"github.com/rs/zerolog"
	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
)

// GetSigner creates a new Signer based on input parameters
func GetSigner(keyPath string, logger zerolog.Logger, client pb.SigningServiceClient) sigstoresigner.Signer {
	var signer sigstoresigner.Signer
	if keyPath != "" {
		signer = clsigner.NewCosignSigner(keyPath, logger)
	} else {
		signer = chainloop.NewChainloopSigner(keyPath, client, logger)
	}

	return sigdsee.WrapSigner(signer, "application/vnd.in-toto+json")
}
