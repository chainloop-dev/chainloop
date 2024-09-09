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
	_ "github.com/sigstore/sigstore/pkg/signature/kms/aws"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/azure"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/gcp"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/hashivault"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	// Couldn't find an easier way to disable the timestamp
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, FormatTimestamp: func(interface{}) string { return "" }})
	rootCmd := cmd.NewRootCmd(logger)

	// Run the command
	if err := rootCmd.Execute(); err != nil {
		msg, exitCode := errorInfo(err, logger)
		logger.Error().Msg(msg)
		os.Exit(exitCode)
	}
}

// handle predefined errors and handle types so we can tailor the experience
func errorInfo(err error, logger zerolog.Logger) (string, int) {
	var msg string
	exitCode := 1

	// Extract message from grpc message if applicable
	st, ok := status.FromError(err)
	// It's a regular error
	if !ok {
		msg = err.Error()
	} else {
		msg = st.Message()
		// Sanitize error message for validation and other known errors
		// by default status.fromError(err).Message() returns the whole error chain
		// i.e "creating API token: creating API token: rpc error: code = AlreadyExists desc = duplicated: name already taken"
		// We do not want to show that in some specific error codes, we just want to show the part of the gRPC response
		// i.e "duplicated: name already taken"
		// To do what we perform an additional parsing of the error similar to
		// https://github.com/grpc/grpc-go/blob/ced812e3287e15a009eab5b271c25750050a2f82/status/status.go#L123
		type grpcstatus interface{ GRPCStatus() *status.Status }
		var gs grpcstatus
		if errors.As(err, &gs) {
			knownCodes := []codes.Code{
				codes.AlreadyExists, codes.InvalidArgument, codes.NotFound, codes.PermissionDenied,
			}

			grpcStatus := gs.GRPCStatus()
			for _, code := range knownCodes {
				if st.Code() == code {
					msg = grpcStatus.Message()
					break
				}
			}
		}
	}

	// Make overrides
	switch {
	case v1.IsCasBackendErrorReasonRequired(err):
		msg = "you need to enable a CAS backend first. Refer to `chainloop cas-backend` command or contact your administrator."
	case v1.IsCasBackendErrorReasonInvalid(err):
		msg = "the CAS backend you provided is invalid. Refer to `chainloop cas-backend update` command or contact your administrator."
	case isWrappedErr(st, jwtMiddleware.ErrTokenExpired):
		msg = "your authentication token has expired, please run chainloop auth login again"
	case isWrappedErr(st, jwtMiddleware.ErrMissingJwtToken):
		msg = "authentication required, please run \"chainloop auth login\""
	case v1.IsUserWithNoMembershipErrorNotInOrg(err):
		msg = cmd.UserWithNoOrganizationMsg
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
