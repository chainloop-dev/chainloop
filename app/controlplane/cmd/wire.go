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
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/conf"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/server"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/service"
	backend "github.com/chainloop-dev/bedrock/internal/blobmanager"
	"github.com/chainloop-dev/bedrock/internal/blobmanager/oci"
	"github.com/chainloop-dev/bedrock/internal/credentials"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Auth, *conf.Data, credentials.ReaderWriter, log.Logger) (*app, func(), error) {
	panic(
		wire.Build(
			wire.Bind(new(credentials.Reader), new(credentials.ReaderWriter)),
			server.ProviderSet,
			data.ProviderSet,
			biz.ProviderSet,
			service.ProviderSet,
			wire.Bind(new(backend.Provider), new(*oci.BackendProvider)),
			oci.NewBackendProvider,
			serviceOpts,
			newApp,
		),
	)
}

func serviceOpts(l log.Logger) []service.NewOpt {
	return []service.NewOpt{
		service.WithLogger(l),
	}
}
