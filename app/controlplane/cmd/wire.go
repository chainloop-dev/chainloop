//
// Copyright 2024 The Chainloop Authors.
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
	"fmt"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/dispatcher"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/server"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
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
			wire.FieldsOf(new(*conf.Bootstrap), "Server", "Auth", "Data", "CasServer", "ReferrerSharedIndex", "Onboarding", "PrometheusIntegration", "PolicyProviders", "NatsServer", "CertificateAuthorities"),
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
			newSigningCAs,
		),
	)
}

func newDataConf(in *conf.Data_Database) *data.NewConfig {
	c := &data.NewConfig{Driver: in.Driver, Source: in.Source, MinOpenConns: in.MinOpenConns, MaxOpenConns: in.MaxOpenConns}
	if in.MaxConnIdleTime != nil {
		c.MaxConnIdleTime = in.MaxConnIdleTime.AsDuration()
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

func serviceOpts(l log.Logger) []service.NewOpt {
	return []service.NewOpt{
		service.WithLogger(l),
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

func newSigningCAs(cas []*conf.CA, logger log.Logger) (*ca.CertificateAuthorities, error) {
	authorities, err := ca.NewCertificateAuthoritiesFromConfig(cas, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA authorities: %w", err)
	}
	// No CA configured, keyless will be deactivated.
	if len(authorities.GetAuthorities()) == 0 {
		_ = logger.Log(log.LevelInfo, "msg", "Keyless Signing NOT configured")
		return nil, nil
	}
	return authorities, nil
}
