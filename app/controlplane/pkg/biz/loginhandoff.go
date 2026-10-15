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

package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/internal/oauth"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var loginHandoffTracer = otelx.Tracer("chainloop-controlplane", "biz/loginhandoff")

const (
	// LoginHandoffTTL is how long a handoff code can be exchanged after it is issued
	LoginHandoffTTL         = 60 * time.Second
	loginHandoffSweepPeriod = 10 * time.Minute
)

// Reasons why a handoff exchange fails. They are meant for the server log only,
// the client always gets the same error
const (
	LoginHandoffReasonUnknown       = "unknown code"
	LoginHandoffReasonExpired       = "expired"
	LoginHandoffReasonAlreadyUsed   = "already used"
	LoginHandoffReasonWrongVerifier = "wrong verifier"
	LoginHandoffReasonWrongCallback = "wrong callback"
)

// LoginHandoff is a single-use code a login delivers to the client callback instead of the session token
type LoginHandoff struct {
	UserID uuid.UUID
	// Lifetime of the session token created at exchange time
	TokenTTL      time.Duration
	CodeChallenge string
	Callback      string
	ExpiresAt     time.Time
}

type LoginHandoffRepo interface {
	Create(ctx context.Context, codeHash string, h *LoginHandoff) error
	// Consume marks the code used in one atomic step. It returns the handoff and whether this
	// call is the one that consumed it. It returns ErrNotFound if there is no such code
	Consume(ctx context.Context, codeHash string, at time.Time) (*LoginHandoff, bool, error)
	// DeleteExpired removes the codes that expired before the given time. A used code is
	// inert, it goes when it expires
	DeleteExpired(ctx context.Context, before time.Time) (int, error)
}

// ErrLoginHandoffExchange is returned by a failed exchange. Reason is for the server log only
type ErrLoginHandoffExchange struct {
	Reason string
}

func (e ErrLoginHandoffExchange) Error() string {
	return "login handoff exchange failed: " + e.Reason
}

type LoginHandoffUseCase struct {
	repo   LoginHandoffRepo
	logger *log.Helper
	// overridable in tests
	now func() time.Time
}

func NewLoginHandoffUseCase(repo LoginHandoffRepo, logger log.Logger) *LoginHandoffUseCase {
	return &LoginHandoffUseCase{
		repo:   repo,
		logger: log.NewHelper(log.With(logger, "component", "biz/LoginHandoff")),
		now:    time.Now,
	}
}

// Issue stores a new handoff code for the given user and returns it. Only its hash is stored
func (uc *LoginHandoffUseCase) Issue(ctx context.Context, userID string, tokenTTL time.Duration, codeChallenge, callback string) (string, error) {
	ctx, span := otelx.Start(ctx, loginHandoffTracer, "LoginHandoffUseCase.Issue")
	defer span.End()

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return "", NewErrInvalidUUID(err)
	}

	if !oauth.ValidS256Challenge(codeChallenge) {
		return "", NewErrValidationStr("invalid code challenge")
	}

	if callback == "" {
		return "", NewErrValidationStr("callback is required")
	}

	// 256 bits from crypto/rand, base64url encoded
	code := oauth2.GenerateVerifier()
	if err := uc.repo.Create(ctx, hashLoginHandoffCode(code), &LoginHandoff{
		UserID:        userUUID,
		TokenTTL:      tokenTTL,
		CodeChallenge: codeChallenge,
		Callback:      callback,
		ExpiresAt:     uc.now().Add(LoginHandoffTTL),
	}); err != nil {
		return "", fmt.Errorf("storing handoff code: %w", err)
	}

	return code, nil
}

// Exchange consumes the handoff code and returns it when the verifier matches the challenge and
// the callback matches the one the login was started with. Any failure is an ErrLoginHandoffExchange
func (uc *LoginHandoffUseCase) Exchange(ctx context.Context, code, codeVerifier, callback string) (*LoginHandoff, error) {
	ctx, span := otelx.Start(ctx, loginHandoffTracer, "LoginHandoffUseCase.Exchange")
	defer span.End()

	now := uc.now()
	// The code is marked used before any other check, so a wrong guess of the verifier burns it
	h, consumed, err := uc.repo.Consume(ctx, hashLoginHandoffCode(code), now)
	if err != nil {
		if IsNotFound(err) {
			return nil, ErrLoginHandoffExchange{Reason: LoginHandoffReasonUnknown}
		}
		return nil, fmt.Errorf("consuming handoff code: %w", err)
	}

	switch {
	case !consumed:
		return nil, ErrLoginHandoffExchange{Reason: LoginHandoffReasonAlreadyUsed}
	case !now.Before(h.ExpiresAt):
		return nil, ErrLoginHandoffExchange{Reason: LoginHandoffReasonExpired}
	case !oauth.VerifyS256(codeVerifier, h.CodeChallenge):
		return nil, ErrLoginHandoffExchange{Reason: LoginHandoffReasonWrongVerifier}
	case callback != h.Callback:
		return nil, ErrLoginHandoffExchange{Reason: LoginHandoffReasonWrongCallback}
	}

	return h, nil
}

// Sweep deletes the expired handoff codes
func (uc *LoginHandoffUseCase) Sweep(ctx context.Context) error {
	ctx, span := otelx.Start(ctx, loginHandoffTracer, "LoginHandoffUseCase.Sweep")
	defer span.End()

	n, err := uc.repo.DeleteExpired(ctx, uc.now())
	if err != nil {
		return fmt.Errorf("deleting expired login handoff codes: %w", err)
	}

	if n > 0 {
		uc.logger.Debugw("msg", "deleted expired login handoff codes", "count", n)
	}

	return nil
}

// StartSweeper runs Sweep periodically until the context is done
func (uc *LoginHandoffUseCase) StartSweeper(ctx context.Context) {
	ticker := time.NewTicker(loginHandoffSweepPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := uc.Sweep(ctx); err != nil {
				uc.logger.Errorf("login handoff sweep failed: %v", err)
			}
		}
	}
}

func hashLoginHandoffCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
