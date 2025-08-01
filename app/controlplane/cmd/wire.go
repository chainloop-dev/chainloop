//
// Copyright 2024-2025 The Chainloop Authors.
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
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/dispatcher"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/server"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	pkgConf "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/policies"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/loader"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

func wireApp(*conf.Bootstrap, credentials.ReaderWriter, log.Logger, sdk.AvailablePlugins) (*app, func(), error) {
	panic(
		wire.Build(
			wire.Bind(new(credentials.Reader), new(credentials.ReaderWriter)),
			server.ProviderSet,
			data.ProviderSet,
			biz.ProviderSet,
			loader.LoadProviders,
			service.ProviderSet,
			wire.Bind(new(biz.CASClient), new(*biz.CASClientUseCase)),
			serviceOpts,
			wire.Value([]biz.CASClientOpts{}),
			wire.FieldsOf(new(*conf.Bootstrap), "Server", "Auth", "Data", "CasServer", "ReferrerSharedIndex", "Onboarding", "PrometheusIntegration", "PolicyProviders", "NatsServer", "FederatedAuthentication"),
			wire.FieldsOf(new(*conf.Data), "Database"),
			dispatcher.New,
			authz.NewDatabaseEnforcer,
			policies.NewRegistry,
			newApp,
			newProtoValidator,
			newDataConf,
			newPolicyProviderConfig,
			newNatsConnection,
			auditor.NewAuditLogPublisher,
			newCASServerOptions,
			newAuthAllowList,
			newJWTConfig,
			authzConfig,
		),
	)
}

func authzConfig(conf *conf.Bootstrap) *authz.Config {
	return &authz.Config{ManagedResources: authz.ManagedResources, RolesMap: authz.RolesMap, RestrictOrgCreation: conf.RestrictOrgCreation}
}

func newJWTConfig(conf *conf.Auth) *biz.APITokenJWTConfig {
	return &biz.APITokenJWTConfig{
		SymmetricHmacKey: conf.GeneratedJwsHmacSecret,
	}
}

func newDataConf(in *conf.Data_Database) *pkgConf.DatabaseConfig {
	c := &pkgConf.DatabaseConfig{Driver: in.Driver, Source: in.Source, MinOpenConns: in.MinOpenConns, MaxOpenConns: in.MaxOpenConns}
	if in.MaxConnIdleTime != nil {
		c.MaxConnIdleTime = in.MaxConnIdleTime
	}
	return c
}

func newPolicyProviderConfig(in []*conf.PolicyProvider) []*policies.NewRegistryConfig {
	out := make([]*policies.NewRegistryConfig, 0, len(in))
	for _, p := range in {
		out = append(out, &policies.NewRegistryConfig{Name: p.Name, Host: p.Host, Default: p.Default, URL: p.Url})
	}
	return out
}

func serviceOpts(l log.Logger, enforcer *authz.Enforcer, pUC *biz.ProjectUseCase, gUC *biz.GroupUseCase) []service.NewOpt {
	return []service.NewOpt{
		service.WithLogger(l),
		service.WithEnforcer(enforcer),
		service.WithProjectUseCase(pUC),
		service.WithGroupUseCase(gUC),
	}
}

func newCASServerOptions(in *conf.Bootstrap_CASServer) *biz.CASServerDefaultOpts {
	if in == nil {
		return &biz.CASServerDefaultOpts{}
	}
	return &biz.CASServerDefaultOpts{
		DefaultEntryMaxSize: in.GetDefaultEntryMaxSize(),
	}
}

func newAuthAllowList(conf *conf.Bootstrap) *pkgConf.AllowList {
	return conf.Auth.GetAllowList()
}
