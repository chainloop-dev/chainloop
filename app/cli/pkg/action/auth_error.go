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

package action

import (
	"errors"
	"fmt"

	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// authMessages maps the control plane's authentication failures, keyed by the
// message Kratos sends for each, to what the user should do about them.
//
// NOTE: kratos errors.Is() is deliberately not used here because it compares
// only Code and Reason, which are 401 and "UNAUTHORIZED" for all of these, so
// every 401 would match the first entry. The message is the only part that
// tells them apart, which is why it is the key.
var authMessages = map[string]string{
	jwtMiddleware.ErrTokenExpired.Message:    `your authentication token has expired, please run "chainloop auth login" again`,
	jwtMiddleware.ErrTokenInvalid.Message:    `your authentication token is invalid, please run "chainloop auth login" again`,
	jwtMiddleware.ErrTokenParseFail.Message:  `failed to parse authentication token, please run "chainloop auth login" again`,
	jwtMiddleware.ErrMissingJwtToken.Message: `authentication required, please run "chainloop auth login"`,
}

// AuthErrorMessage renders the message the CLI shows for an authentication
// failure, and reports whether err was one. Every place that surfaces a
// credential problem goes through it — the top-level error handler and the
// commands that only log the failure, like `chainloop trace run` — so the CLI
// says the same thing wherever it comes up.
//
// It digs the gRPC status out of a wrapped error, so callers are free to add
// context: grpc's status.FromError replaces a wrapped error's message with the
// whole error chain, which nothing in the table above would match.
func AuthErrorMessage(err error) (string, bool) {
	var grpcErr interface{ GRPCStatus() *status.Status }
	if !errors.As(err, &grpcErr) {
		return "", false
	}

	st := grpcErr.GRPCStatus()
	if st.Code() != codes.Unauthenticated {
		return "", false
	}

	if msg, known := authMessages[st.Message()]; known {
		return msg, true
	}

	// Any other 401: the control plane's own message says more than a guess.
	return fmt.Sprintf("authentication error: %s", st.Message()), true
}
