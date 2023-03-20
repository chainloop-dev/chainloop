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

package biz

import (
	"context"
	"fmt"
	"io"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/chainloop-dev/chainloop/internal/grpcconn"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
)

type CASClientUseCase struct {
	credsProvider *CASCredentialsUseCase
	casServerConf *conf.Bootstrap_CASServer
	logger        *log.Helper
}

type CASDescriber interface {
	Configured() bool
}
type CASUploader interface {
	CASDescriber
	Upload(ctx context.Context, secretID string, content io.Reader, filename, digest string) error
}

func NewCASClientUseCase(credsProvider *CASCredentialsUseCase, config *conf.Bootstrap_CASServer, l log.Logger) *CASClientUseCase {
	return &CASClientUseCase{credsProvider, config, servicelogger.ScopedHelper(l, "biz/cas-client")}
}

// The secretID is embedded in the JWT token and is used to identify the secret by the CAS server
func (uc *CASClientUseCase) Upload(ctx context.Context, secretID string, content io.Reader, filename, digest string) error {
	uc.logger.Infow("msg", "upload initialized", "filename", filename, "digest", digest)

	// client with temporary set of credentials
	client, err := uc.casAPIClient(secretID, casJWT.Uploader)
	if err != nil {
		return fmt.Errorf("failed to create cas client: %w", err)
	}

	status, err := client.Upload(ctx, content, filename, digest)
	if err != nil {
		return fmt.Errorf("failed to upload content: %w", err)
	}

	uc.logger.Infow("msg", "upload finished", "status", status)

	return nil
}

// create a client with a temporary set of credentials for a specific operation
func (uc *CASClientUseCase) casAPIClient(secretID string, role casJWT.Role) (*casclient.Client, error) {
	token, err := uc.credsProvider.GenerateTemporaryCredentials(secretID, role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate temporary credentials: %w", err)
	}

	// Initialize connection to CAS server
	return casClient(uc.casServerConf, token)
}

func casClient(conf *conf.Bootstrap_CASServer, token string) (*casclient.Client, error) {
	conn, err := grpcconn.New(conf.GetGrpc().GetAddr(), token, conf.GetInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc connection: %w", err)
	}

	return casclient.New(conn), nil
}

// If the CAS client configuration is present and valid
func (uc *CASClientUseCase) Configured() bool {
	if uc.casServerConf == nil {
		return false
	}

	err := uc.casServerConf.ValidateAll()
	if err != nil {
		uc.logger.Infow("msg", "Invalid CAS client configuration", "err", err.Error())
	}

	return err == nil
}
