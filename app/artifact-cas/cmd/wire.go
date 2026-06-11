//
// Copyright 2023-2026 The Chainloop Authors.
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

//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"context"

	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/server"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/loader"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/chainloop-dev/chainloop/pkg/natsconn"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Bootstrap, *conf.Server, *conf.Auth, credentials.Reader, log.Logger) (*app, func(), error) {
	panic(
		wire.Build(
			server.ProviderSet,
			service.ProviderSet,
			loader.LoadProviders,
			newApp,
			serviceOpts,
			newProtoValidator,
			newNatsConfig,
			natsconn.New,
			newAuditLogPublisher,
			service.NewAuditDispatcher,
		),
	)
}

func serviceOpts(l log.Logger, audit *service.AuditDispatcher) []service.NewOpt {
	return []service.NewOpt{
		service.WithLogger(l),
		service.WithAuditDispatcher(audit),
	}
}

// newNatsConfig converts the proto config to a plain natsconn.Config, nil when unset
func newNatsConfig(bc *conf.Bootstrap) *natsconn.Config {
	c := bc.GetNatsServer()
	if c.GetUri() == "" {
		return nil
	}

	cfg := &natsconn.Config{
		URI:  c.GetUri(),
		Name: "chainloop-artifact-cas",
	}

	if c.GetToken() != "" {
		cfg.Token = c.GetToken()
	}

	return cfg
}

// newAuditLogPublisher creates a publish-only audit log publisher: the control
// plane owns the chainloop-audit stream configuration, the CAS only publishes to it
func newAuditLogPublisher(rc *natsconn.ReloadableConnection, logger log.Logger) (*auditor.AuditLogPublisher, error) {
	return auditor.NewAuditLogPublisher(context.Background(), rc, logger, auditor.WithoutStreamManagement())
}
