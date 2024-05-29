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

package main

import (
	"flag"
	"os"
	"time"

	"github.com/bufbuild/protovalidate-go"
	"github.com/getsentry/sentry-go"

	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/server"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/credentials/manager"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

var (
	// Name is the name of the compiled software.
	Name string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

// Version is the version of the compiled software.
// go build ldflags "-X main.Version=x.y.z"
var Version = servicelogger.Dev

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

type app struct {
	*kratos.App
	backend.Providers
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, ms *server.HTTPMetricsServer, providers backend.Providers) *app {
	return &app{
		kratos.New(
			kratos.ID(id),
			kratos.Name(Name),
			kratos.Version(Version),
			kratos.Metadata(map[string]string{}),
			kratos.Logger(logger),
			kratos.Server(
				gs,
				hs,
				ms,
			),
		),
		providers,
	}
}

func main() {
	flag.Parse()
	c := config.New(
		config.WithSource(
			// -conf [config directory or path to file]
			file.NewSource(flagconf),
			// Load environments variables prefixed with CAS_
			// NOTE: They get resolved withouth the prefix, i.e CAS_DB_HOST -> DB_HOST
			env.NewSource("CAS_"),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	// validate configuration
	validator, err := protovalidate.New()
	if err != nil {
		panic(err)
	}

	if err := validator.Validate(&bc); err != nil {
		panic(err)
	}

	logger, err := servicelogger.InitZapLogger(Version)
	if err != nil {
		panic(err)
	}

	flush, err := initSentry(&bc, logger)
	defer flush()
	if err != nil {
		panic(err)
	}

	credentialsReader, err := manager.NewFromConfig(bc.GetCredentialsService(), credentials.RoleReader, logger)
	if err != nil {
		panic(err)
	}

	app, cleanup, err := wireApp(bc.Server, bc.Auth, credentialsReader, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	for k := range app.Providers {
		_ = logger.Log(log.LevelInfo, "msg", "CAS backend provider loaded", "provider", k)
	}

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func newProtoValidator() (*protovalidate.Validator, error) {
	return protovalidate.New()
}

func initSentry(c *conf.Bootstrap, logger log.Logger) (cleanupFunc func(), err error) {
	cleanupFunc = func() {
		sentry.Flush(2 * time.Second)
	}

	if c.Observability == nil || c.Observability.Sentry == nil {
		return
	}

	sentryOpts := c.Observability.Sentry
	err = sentry.Init(sentry.ClientOptions{
		Dsn:              sentryOpts.Dsn,
		Environment:      sentryOpts.Environment,
		Release:          Version,
		AttachStacktrace: true,
	})

	if err == nil {
		_ = logger.Log(log.LevelInfo, "msg", "Sentry initialized", "environment", sentryOpts.Environment, "release", Version)
	}

	return
}
