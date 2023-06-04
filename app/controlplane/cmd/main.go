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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	flag "github.com/spf13/pflag"

	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/server"
	credsConfig "github.com/chainloop-dev/chainloop/internal/credentials/api/credentials/v1"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"

	deptrack "github.com/chainloop-dev/chainloop/app/controlplane/integrations/dependencytrack/cyclonedx/v1"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
)

var (
	// Name is the name of the compiled software.
	Name string
	// flagconf is the config flag.
	flagconf string
	id, _    = os.Hostname()
)

// Version is the version of the compiled software.
// go build ldflags "-X main.Version=x.y.z"
var Version = servicelogger.Dev

func init() {
	flag.StringVar(&flagconf, "conf", "../configs", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, ms *server.HTTPMetricsServer, expirer *biz.WorkflowRunExpirerUseCase) *app {
	return &app{
		kratos.New(
			kratos.ID(id),
			kratos.Name(Name),
			kratos.Version(Version),
			kratos.Metadata(map[string]string{}),
			kratos.Logger(logger),
			kratos.Server(gs, hs, ms),
		), expirer}
}

func main() {
	flag.Parse()
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
			// Load environments variables prefixed with CP_
			// NOTE: They get resolved withouth the prefix, i.e CP_DB_HOST -> DB_HOST
			env.NewSource("CP_"),
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

	logger, err := servicelogger.InitZapLogger(Version)
	if err != nil {
		panic(err)
	}

	logger = log.NewFilter(logger, log.FilterFunc(filterSensitiveArgs))

	flush, err := initSentry(&bc, logger)
	defer flush()
	if err != nil {
		panic(err)
	}

	credsWriter, err := credsConfig.NewFromConfig(bc.GetCredentialsService(), logger)
	if err != nil {
		panic(err)
	}

	app, cleanup, err := wireApp(&bc, credsWriter, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run an expiration job every minute that expires unfinished runs older than 1 hour
	// TODO: Make it configurable from the application config
	app.runsExpirer.Run(ctx, &biz.WorkflowRunExpirerOpts{CheckInterval: 1 * time.Minute, ExpirationWindow: 1 * time.Hour})

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}

// Load the available third party integrations
// In the future this code will iterate over a dynamic directory of plugins
// and try to load them one by one
func loadIntegrations(l log.Logger) (sdk.Initialized, error) {
	var res sdk.Initialized

	var d sdk.FanOut
	d, err := deptrack.NewIntegration(l)
	if err != nil {
		return nil, fmt.Errorf("failed to load dependency track integration: %w", err)
	}

	return append(res, d), nil
}

type app struct {
	*kratos.App
	// Periodic job that expires unfinished attestation processes older than a given threshold
	runsExpirer *biz.WorkflowRunExpirerUseCase
}

func filterSensitiveArgs(_ log.Level, keyvals ...interface{}) bool {
	for i := 0; i < len(keyvals); i++ {
		if keyvals[i] == "operation" {
			switch keyvals[i+1] {
			case "/controlplane.v1.OCIRepositoryService/Save", "/controlplane.v1.AttestationService/Store":
				maskArgs(keyvals)
			case "/controlplane.v1.IntegrationsService/Register", "/controlplane.v1.IntegrationsService.Attach":
				maskArgs(keyvals)
			}
		}
	}

	// False indicates that the log entry can be printed regardless of being modified or not
	return false
}

func maskArgs(keyvals []interface{}) {
	for i := 0; i < len(keyvals); i++ {
		if keyvals[i] == "args" {
			keyvals[i+1] = "***"
		}
	}
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
