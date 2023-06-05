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

package testhelpers

import (
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	robotaccount "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"testing"
)

// wireTestData init testing data
func WireTestData(*TestDatabase, *testing.T, log.Logger, credentials.ReaderWriter, *robotaccount.Builder, *conf.Auth, sdk.Initialized) (*TestingUseCases, func(), error) {
	panic(
		wire.Build(
			data.ProviderSet,
			biz.ProviderSet,
			wire.Bind(new(backend.Provider), new(*oci.BackendProvider)),
			wire.Bind(new(credentials.Reader), new(credentials.ReaderWriter)),
			oci.NewBackendProvider,
			wire.Struct(new(TestingUseCases), "*"),
			newConfData,
		),
	)
}
