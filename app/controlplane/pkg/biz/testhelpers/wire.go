//
// Copyright 2024-2026 The Chainloop Authors.
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

package testhelpers

import (
	"context"
	"testing"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	config "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
	pkgConf "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/policies"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	robotaccount "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backends "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/chainloop-dev/chainloop/pkg/natsconn"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireTestData init testing data
func WireTestData(context.Context, *TestDatabase, *testing.T, log.Logger, credentials.ReaderWriter, *robotaccount.Builder, *conf.Auth, *conf.Bootstrap, []*config.OnboardingSpec, sdk.AvailablePlugins, backends.Providers) (*TestingUseCases, func(), error) {
	panic(
		wire.Build(
			data.ProviderSet,
			biz.ProviderSet,
			wire.Value(&conf.ReferrerSharedIndex{}),
			wire.Struct(new(TestingUseCases), "*"),
			wire.Struct(new(TestingRepos), "*"),
			NewConfData,
			NewDataConfig,
			NewPromSpec,
			NewPolicyProviderConfig,
			policies.NewRegistry,
			authz.NewCasbinEnforcer,
			newNatsReloadableConnection,
			auditor.NewAuditLogPublisher,
			NewCASBackendConfig,
			NewCASServerOptions,
			newAuthAllowList,
			newJWTConfig,
			authzConfig,
			authzUseCaseConfig,
			biz.NewIndexConfig,
		),
	)
}

func authzConfig() *authz.Config {
	return &authz.Config{RolesMap: authz.RolesMap}
}

func authzUseCaseConfig(conf *conf.Bootstrap, casbinEnforcer *authz.CasbinEnforcer, apiTokenRepo biz.APITokenRepo, logger log.Logger) *biz.AuthzUseCaseConfig {
	return &biz.AuthzUseCaseConfig{
		CasbinEnforcer:      casbinEnforcer,
		APITokenRepo:        apiTokenRepo,
		RestrictOrgCreation: conf.RestrictOrgCreation,
		Logger:              logger,
	}
}

func newJWTConfig(conf *conf.Auth) *biz.APITokenJWTConfig {
	return &biz.APITokenJWTConfig{
		SymmetricHmacKey: conf.GeneratedJwsHmacSecret,
	}
}

// newNatsReloadableConnection returns nil in tests (NATS is not available).
func newNatsReloadableConnection() *natsconn.ReloadableConnection {
	return nil
}

func newAuthAllowList(conf *conf.Bootstrap) *pkgConf.AllowList {
	return conf.Auth.GetAllowList()
}
