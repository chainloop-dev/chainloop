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
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/server"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/loader"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/s3accesspoint"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
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
			newLoaderOptions,
			wire.FieldsOf(new(*conf.Bootstrap), "ManagedCasBackends"),
			newApp,
			serviceOpts,
			newProtoValidator,
		),
	)
}

// newLoaderOptions builds the loader.Options struct from the deployment
// Bootstrap. When `managed_cas_backends.s3_access_point` is absent (the
// common case for on-prem) S3AccessPoint stays nil and the provider is
// not registered, leaving the binary's behaviour identical to the
// pre-managed-CAS world.
func newLoaderOptions(in *conf.ManagedCASBackends, l log.Logger) *loader.Options {
	opts := &loader.Options{Logger: l}
	if in == nil || in.GetS3AccessPoint() == nil {
		return opts
	}
	ap := in.GetS3AccessPoint()
	opts.S3AccessPoint = &s3accesspoint.Config{
		BaseRoleARN:                  ap.GetBaseRoleArn(),
		Region:                       ap.GetRegion(),
		SessionDuration:              ap.GetSessionDuration().AsDuration(),
		DevModeUseAmbientCredentials: ap.GetDevModeUseAmbientCredentials(),
	}
	return opts
}

func serviceOpts(l log.Logger) []service.NewOpt {
	return []service.NewOpt{
		service.WithLogger(l),
	}
}
