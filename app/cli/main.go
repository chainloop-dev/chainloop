//
// Copyright 2023 The Chainloop Authors.
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

package main

import (
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/cmd"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/go-kratos/kratos/v2/errors"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/status"
)

func main() {
	// Couldn't find an easier way to disable the timestamp
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, FormatTimestamp: func(interface{}) string { return "" }})
	rootCmd := cmd.NewRootCmd(logger)

	if err := rootCmd.Execute(); err != nil {
		msg, exitCode := errorInfo(err, logger)
		logger.Error().Msg(msg)
		os.Exit(exitCode)
	}
}

// handle predefinided errors and handle types so we can tailor the experience
func errorInfo(err error, logger zerolog.Logger) (string, int) {
	var msg string
	exitCode := 1

	// Extract message from grpc message if applicable
	st, ok := status.FromError(err)
	if !ok {
		// Regular error
		msg = err.Error()
	} else {
		msg = st.Message()
	}

	// Make overrides
	switch {
	case v1.IsOciRepositoryErrorReasonRequired(err):
		msg = "you need to set up an OCI repository first. Refer to https://docs.chainloop.dev/getting-started/setup#add-oci-repository for more information."
	case v1.IsOciRepositoryErrorReasonInvalid(err):
		msg = "the OCI repository you provided is invalid. Refer to https://docs.chainloop.dev/getting-started/setup#add-oci-repository for more information."
	case v1.IsAllowListErrorNotInList(err):
		msg = "your user is not part of the private beta yet. You can request access at https://docs.chainloop.dev/getting-started/private-beta"
	case isWrappedErr(st, jwtMiddleware.ErrTokenExpired):
		msg = "your authentication token has expired, please run chainloop auth login again"
	case isWrappedErr(st, jwtMiddleware.ErrMissingJwtToken):
		msg = "authentication required, please run \"chainloop auth login\""
	case errors.As(err, &cmd.GracefulError{}):
		// Graceful recovery if the flag is set and the received error is marked as recoverable
		if cmd.GracefulExit {
			logger.Debug().Msg("GracefulErrorExit enabled (exitCode 0). If you want to disable it set --graceful-exit=false")
			exitCode = 0
		}
	}

	return msg, exitCode
}

func isWrappedErr(grpcStatus *status.Status, err *errors.Error) bool {
	target := errors.FromError(grpcStatus.Err())
	return target.Code == err.Code && err.Message == target.Message
}
