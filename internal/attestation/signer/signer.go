package signer

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/rs/zerolog"
	"github.com/sigstore/cosign/v2/pkg/signature"
	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"golang.org/x/term"
)

type ChainloopSigner struct {
	sigstoresigner.Signer

	keyPath              string
	signingServiceClient pb.SigningServiceClient
	logger               zerolog.Logger
	mu                   sync.Mutex
}

var _ sigstoresigner.Signer = (*ChainloopSigner)(nil)

func NewChainloopSigner(keyPath string, sc pb.SigningServiceClient, logger zerolog.Logger) *ChainloopSigner {
	return &ChainloopSigner{
		keyPath:              keyPath,
		signingServiceClient: sc,
		logger:               logger,
	}
}

func (cs *ChainloopSigner) SignMessage(message io.Reader, opts ...sigstoresigner.SignOption) ([]byte, error) {

	var err error

	err = cs.ensureInitiated(context.Background())
	if err != nil {
		return nil, err
	}

	return cs.Signer.SignMessage(message, opts...)
}

func (cs *ChainloopSigner) ensureInitiated(ctx context.Context) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.Signer != nil {
		return nil
	}

	var (
		signer sigstoresigner.Signer
		err    error
	)

	if cs.keyPath != "" {
		cs.logger.Debug().Str("path", cs.keyPath).Msg("loading key")
		signer, err = signature.SignerFromKeyRef(context.Background(), cs.keyPath, getPass)
		if err != nil {
			return err
		}
	} else {
		// key is not provided, let's create one
		cs.logger.Info().Msg("key not provided, running in key-less mode")
		signer, err = cs.keyLessSigner(ctx)
		if err != nil {
			return err
		}
	}

	cs.Signer = sigdsee.WrapSigner(signer, "application/vnd.in-toto+json")

	return nil
}

func getPass(confirm bool) ([]byte, error) {
	read := readPasswordFn(confirm)
	return read()
}

// based on cosign code
// https://pkg.go.dev/github.com/sigstore/cosign/cmd/cosign/cli/generate
func readPasswordFn(confirm bool) func() ([]byte, error) {
	pw, ok := os.LookupEnv("CHAINLOOP_SIGNING_PASSWORD")
	switch {
	case ok:
		return func() ([]byte, error) {
			return []byte(pw), nil
		}
	case isTerminal():
		return func() ([]byte, error) {
			return getPassFromTerm(confirm)
		}
	// Handle piped in passwords.
	default:
		return func() ([]byte, error) {
			return io.ReadAll(os.Stdin)
		}
	}
}

func isTerminal() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func getPassFromTerm(confirm bool) ([]byte, error) {
	fmt.Fprint(os.Stderr, "Enter password for private key: ")
	// Unnecessary convert of syscall.Stdin on *nix, but Windows is a uintptr
	// nolint:unconvert
	pw1, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr)
	if !confirm {
		return pw1, nil
	}
	fmt.Fprint(os.Stderr, "Enter password for private key again: ")
	// Unnecessary convert of syscall.Stdin on *nix, but Windows is a uintptr
	// nolint:unconvert
	confirmpw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}

	if string(pw1) != string(confirmpw) {
		return nil, errors.New("passwords do not match")
	}
	return pw1, nil
}

type certificateRequest struct {
	PrivateKey *ecdsa.PrivateKey
	// CertificateRequestPEM contains the signed public key and the CSR metadata
	CertificateRequestPEM []byte
}

func (cs *ChainloopSigner) keyLessSigner(ctx context.Context) (sigstoresigner.Signer, error) {
	request, err := cs.createCertificateRequest()
	if err != nil {
		return nil, err
	}
	_, err = cs.certFromChainloop(ctx, request)
	if err != nil {
		return nil, err
	}
	sv, err := sigstoresigner.LoadECDSASignerVerifier(request.PrivateKey, crypto.SHA256)
	if err != nil {
		return nil, err
	}

	return sv, nil
}

func (cs *ChainloopSigner) createCertificateRequest() (*certificateRequest, error) {
	cs.logger.Debug().Msg("generating new certificate request")

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating cert: %w", err)
	}
	csrTmpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: "ephemeral certificate"}}
	derCSR, err := x509.CreateCertificateRequest(rand.Reader, csrTmpl, priv)
	if err != nil {
		return nil, fmt.Errorf("generating certificate request: %w", err)
	}

	// Encode CSR to PEM
	pemCSR := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: derCSR,
	})

	return &certificateRequest{
		CertificateRequestPEM: pemCSR,
		PrivateKey:            priv,
	}, nil
}

func (cs *ChainloopSigner) certFromChainloop(ctx context.Context, req *certificateRequest) ([]string, error) {
	cr := pb.GenerateSigningCertRequest{
		CertificateSigningRequest: req.CertificateRequestPEM,
	}

	// call chainloop
	resp, err := cs.signingServiceClient.GenerateSigningCert(ctx, &cr)
	if err != nil {
		return nil, err
	}

	// get full chain
	return resp.GetChain().GetCertificates(), nil
}
