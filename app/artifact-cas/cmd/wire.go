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
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/server"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/loader"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Auth, credentials.Reader, log.Logger) (*app, func(), error) {
	panic(
		wire.Build(
			server.ProviderSet,
			service.ProviderSet,
			loader.LoadProviders,
			newApp,
			serviceOpts,
		),
	)
}

func serviceOpts(l log.Logger) []service.NewOpt {
	return []service.NewOpt{
		service.WithLogger(l),
	}
}
