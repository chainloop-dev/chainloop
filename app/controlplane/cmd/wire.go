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

//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/dispatcher"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/server"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/loader"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/sigstore/fulcio/pkg/ca"
)

func wireApp(*conf.Bootstrap, credentials.ReaderWriter, log.Logger, sdk.AvailablePlugins, ca.CertificateAuthority) (*app, func(), error) {
	panic(
		wire.Build(
			wire.Bind(new(credentials.Reader), new(credentials.ReaderWriter)),
			server.ProviderSet,
			data.ProviderSet,
			biz.ProviderSet,
			loader.LoadProviders,
			service.ProviderSet,
			wire.Bind(new(biz.CASClient), new(*biz.CASClientUseCase)),
			wire.Bind(new(biz.SigningCertCreator), new(*biz.SigningUseCase)),
			serviceOpts,
			wire.Value([]biz.CASClientOpts{}),
			wire.FieldsOf(new(*conf.Bootstrap), "Server", "Auth", "Data", "CasServer", "ReferrerSharedIndex"),
			wire.FieldsOf(new(*conf.Data), "Database"),
			dispatcher.New,
			authz.NewDatabaseEnforcer,
			newApp,
			newProtoValidator,
		),
	)
}

func serviceOpts(l log.Logger) []service.NewOpt {
	return []service.NewOpt{
		service.WithLogger(l),
	}
}
