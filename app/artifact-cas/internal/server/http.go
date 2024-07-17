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

package server

import (
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"os"

	api "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	middlewares_http "github.com/chainloop-dev/chainloop/pkg/middlewares/http"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new a HTTP server.
func NewHTTPServer(c *conf.Server, authConf *conf.Auth, downloadSvc *service.DownloadService, providers backend.Providers, logger log.Logger) (*http.Server, error) {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}

	// Load the key on initialization instead of on every request
	// TODO: implement jwks endpoint
	publicKeyPath := authConf.GetPublicKeyPath()
	if publicKeyPath == "" {
		// Maintain backwards compatibility
		publicKeyPath = authConf.RobotAccountPublicKeyPath
	}

	rawKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %w", err)
	}

	srv := http.NewServer(opts...)

	srv.Handle(service.DownloadPath, middlewares_http.AuthFromQueryParam(loadPublicKey(rawKey), claimsFunc(), casJWT.SigningMethod, downloadSvc))
	api.RegisterStatusServiceHTTPServer(srv, service.NewStatusService(Version, providers))
	return srv, nil
}

// claimsFunc returns the claims function for the JWT middleware that casts the claims to the correct type
func claimsFunc() middlewares_http.ClaimsFunc {
	return func() jwt.Claims {
		return &casJWT.Claims{}
	}
}
