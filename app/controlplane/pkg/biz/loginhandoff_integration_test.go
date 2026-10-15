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

package biz_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/internal/oauth"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const handoffTestCallback = "http://localhost:1234/auth/callback"

func TestLoginHandoffUseCase(t *testing.T) {
	suite.Run(t, new(loginHandoffTestSuite))
}

type loginHandoffTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	user      *biz.User
	verifier  string
	challenge string
}

func (s *loginHandoffTestSuite) SetupTest() {
	var err error
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())
	s.user, err = s.User.UpsertByEmail(context.Background(), "handoff@test.com", nil)
	s.Require().NoError(err)

	s.verifier, err = oauth.NewCodeVerifier()
	s.Require().NoError(err)
	s.challenge = oauth.CodeChallengeS256(s.verifier)
}

func (s *loginHandoffTestSuite) issue() string {
	code, err := s.LoginHandoff.Issue(context.Background(), s.user.ID, time.Hour, s.challenge, handoffTestCallback)
	s.Require().NoError(err)
	return code
}

func (s *loginHandoffTestSuite) requireReason(err error, reason string) {
	var exchangeErr biz.ErrLoginHandoffExchange
	s.Require().True(errors.As(err, &exchangeErr), "unexpected error %v", err)
	s.Equal(reason, exchangeErr.Reason)
}

func (s *loginHandoffTestSuite) TestIssueValidation() {
	ctx := context.Background()
	testCases := []struct {
		name      string
		userID    string
		challenge string
		callback  string
	}{
		{"invalid user", "not-a-uuid", s.challenge, handoffTestCallback},
		{"invalid challenge", s.user.ID, "plain", handoffTestCallback},
		{"missing callback", s.user.ID, s.challenge, ""},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			_, err := s.LoginHandoff.Issue(ctx, tc.userID, time.Hour, tc.challenge, tc.callback)
			s.Error(err)
		})
	}
}

func (s *loginHandoffTestSuite) TestIssueCodeIsRandomAndNotStored() {
	code := s.issue()
	s.GreaterOrEqual(len(code), 43)
	s.NotEqual(code, s.issue())

	// Only the hash is stored
	rows := s.Data.DB.LoginHandoff.Query().AllX(context.Background())
	s.Require().Len(rows, 2)
	for _, r := range rows {
		s.NotEqual(code, r.CodeHash)
		s.Len(r.CodeHash, 64)
	}
}

func (s *loginHandoffTestSuite) TestExchange() {
	ctx := context.Background()
	h, err := s.LoginHandoff.Exchange(ctx, s.issue(), s.verifier, handoffTestCallback)
	s.Require().NoError(err)
	s.Equal(s.user.ID, h.UserID.String())
	s.Equal(time.Hour, h.TokenTTL)
}

func (s *loginHandoffTestSuite) TestExchangeFailures() {
	ctx := context.Background()
	otherVerifier, err := oauth.NewCodeVerifier()
	s.Require().NoError(err)

	testCases := []struct {
		name     string
		code     func() string
		verifier string
		callback string
		// shift of the clock at exchange time
		after  time.Duration
		reason string
	}{
		{"unknown code", func() string { return "unknown" }, s.verifier, handoffTestCallback, 0, biz.LoginHandoffReasonUnknown},
		{"expired", s.issue, s.verifier, handoffTestCallback, biz.LoginHandoffTTL, biz.LoginHandoffReasonExpired},
		{"wrong verifier", s.issue, otherVerifier, handoffTestCallback, 0, biz.LoginHandoffReasonWrongVerifier},
		{"challenge as verifier", s.issue, s.challenge, handoffTestCallback, 0, biz.LoginHandoffReasonWrongVerifier},
		{"wrong callback", s.issue, s.verifier, "http://localhost:4321/auth/callback", 0, biz.LoginHandoffReasonWrongCallback},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			code := tc.code()
			s.LoginHandoff.SetNow(func() time.Time { return time.Now().Add(tc.after) })
			defer s.LoginHandoff.SetNow(time.Now)

			_, err := s.LoginHandoff.Exchange(ctx, code, tc.verifier, tc.callback)
			s.requireReason(err, tc.reason)
		})
	}
}

func (s *loginHandoffTestSuite) TestExchangeIsSingleUse() {
	ctx := context.Background()
	code := s.issue()

	_, err := s.LoginHandoff.Exchange(ctx, code, s.verifier, handoffTestCallback)
	s.Require().NoError(err)

	_, err = s.LoginHandoff.Exchange(ctx, code, s.verifier, handoffTestCallback)
	s.requireReason(err, biz.LoginHandoffReasonAlreadyUsed)
}

func (s *loginHandoffTestSuite) TestFailedExchangeBurnsTheCode() {
	ctx := context.Background()
	code := s.issue()

	_, err := s.LoginHandoff.Exchange(ctx, code, "wrong-verifier-wrong-verifier-wrong-verifier", handoffTestCallback)
	s.requireReason(err, biz.LoginHandoffReasonWrongVerifier)

	_, err = s.LoginHandoff.Exchange(ctx, code, s.verifier, handoffTestCallback)
	s.requireReason(err, biz.LoginHandoffReasonAlreadyUsed)
}

func (s *loginHandoffTestSuite) TestConcurrentExchangeOnlyOneWins() {
	ctx := context.Background()
	code := s.issue()

	const workers = 10
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		successes int
	)

	for range workers {
		wg.Go(func() {
			_, err := s.LoginHandoff.Exchange(ctx, code, s.verifier, handoffTestCallback)
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
				return
			}
			s.True(biz.IsErrLoginHandoffExchange(err), "unexpected error %v", err)
		})
	}

	wg.Wait()
	s.Equal(1, successes)
}

func (s *loginHandoffTestSuite) TestSweep() {
	ctx := context.Background()
	used := s.issue()
	_ = s.issue() // pending
	_, err := s.LoginHandoff.Exchange(ctx, used, s.verifier, handoffTestCallback)
	s.Require().NoError(err)

	// The used code goes, the pending one stays
	s.Require().NoError(s.LoginHandoff.Sweep(ctx))
	s.Equal(1, s.Data.DB.LoginHandoff.Query().CountX(ctx))

	// After expiry, the pending one goes too
	s.LoginHandoff.SetNow(func() time.Time { return time.Now().Add(biz.LoginHandoffTTL + time.Second) })
	s.Require().NoError(s.LoginHandoff.Sweep(ctx))
	s.Equal(0, s.Data.DB.LoginHandoff.Query().CountX(ctx))
}

func (s *loginHandoffTestSuite) TestDeletedUserDeletesCodes() {
	ctx := context.Background()
	code := s.issue()
	s.Require().NoError(s.User.DeleteUser(ctx, s.user.ID))

	_, err := s.LoginHandoff.Exchange(ctx, code, s.verifier, handoffTestCallback)
	require.True(s.T(), biz.IsErrLoginHandoffExchange(err))
}
