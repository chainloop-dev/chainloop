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
	"context"
	"fmt"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/bufbuild/protovalidate-go"
	"github.com/getsentry/sentry-go"
	"github.com/nats-io/nats.go"
	flag "github.com/spf13/pflag"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/server"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/chainloop-dev/chainloop/pkg/credentials/manager"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
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

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, ms *server.HTTPMetricsServer, profilerSvc *server.HTTPProfilerServer,
	expirer *biz.WorkflowRunExpirerUseCase, plugins sdk.AvailablePlugins, tokenSync *biz.APITokenSyncerUseCase,
	userAccessSyncer *biz.UserAccessSyncerUseCase, cfg *conf.Bootstrap) *app {
	servers := []transport.Server{gs, hs, ms}
	if cfg.EnableProfiler {
		servers = append(servers, profilerSvc)
	}

	return &app{
		kratos.New(
			kratos.ID(id),
			kratos.Name(Name),
			kratos.Version(Version),
			kratos.Metadata(map[string]string{}),
			kratos.Logger(logger),
			kratos.Server(servers...),
		), expirer, plugins, tokenSync, userAccessSyncer}
}

func main() {
	flag.Parse()
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
			// Load environments variables prefixed with CP_
			// NOTE: They get resolved without the prefix, i.e CP_DB_HOST -> DB_HOST
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

	logger = log.NewFilter(logger, log.FilterFunc(filterSensitiveArgs))

	flush, err := initSentry(&bc, logger)
	defer flush()
	if err != nil {
		panic(err)
	}

	credsWriter, err := manager.NewFromConfig(bc.GetCredentialsService(), credentials.RoleWriter, logger)
	if err != nil {
		panic(err)
	}

	// Load plugins
	availablePlugins, err := plugins.Load(bc.GetPluginsDir(), logger)
	if err != nil {
		panic(err)
	}
	// Kill plugins processes on exit
	defer availablePlugins.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, cleanup, err := wireApp(&bc, credsWriter, logger, availablePlugins)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// Run an expiration job every minute that expires unfinished runs older than 1 hour
	// TODO: Make it configurable from the application config
	app.runsExpirer.Run(ctx, &biz.WorkflowRunExpirerOpts{CheckInterval: 1 * time.Minute, ExpirationWindow: 1 * time.Hour})

	// Since policies management is not enabled yet but instead is based on a hardcoded list of permissions
	// We'll perform a reconciliation of the policies with the tokens stored in the database on startup
	// This will allow us to add more policies in the future and keep backwards compatibility with existing tokens
	go func() {
		if err := app.tokenAuthSyncer.SyncPolicies(); err != nil {
			_ = logger.Log(log.LevelError, "msg", "syncing policies", "error", err)
		}
	}()

	// Sync user access
	go func() {
		if err := app.userAccessSyncer.SyncUserAccess(ctx); err != nil {
			_ = logger.Log(log.LevelError, "msg", "syncing user access", "error", err)
		}
	}()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}

type app struct {
	*kratos.App
	// Periodic job that expires unfinished attestation processes older than a given threshold
	runsExpirer      *biz.WorkflowRunExpirerUseCase
	availablePlugins sdk.AvailablePlugins
	tokenAuthSyncer  *biz.APITokenSyncerUseCase
	userAccessSyncer *biz.UserAccessSyncerUseCase
}

// Connection to nats is optional, if not configured, pubsub will be disabled
func newNatsConnection(c *conf.Bootstrap_NatsServer) (*nats.Conn, error) {
	uri := c.GetUri()
	if uri == "" {
		return nil, nil
	}

	var opts []nats.Option
	if c.GetAuthentication() != nil {
		switch c.GetAuthentication().(type) {
		case *conf.Bootstrap_NatsServer_Token:
			opts = append(opts, nats.Token(c.GetToken()))
		default:
			return nil, fmt.Errorf("unsupported nats authentication type: %T", c.GetAuthentication())
		}
	}

	nc, err := nats.Connect(uri, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to nats: %w", err)
	}

	return nc, nil
}

func filterSensitiveArgs(_ log.Level, keyvals ...interface{}) bool {
	for i := 0; i < len(keyvals); i++ {
		if keyvals[i] == "operation" {
			switch keyvals[i+1] {
			case "/controlplane.v1.OCIRepositoryService/Save", "/controlplane.v1.AttestationService/Store":
				maskArgs(keyvals)
			case "/controlplane.v1.IntegrationsService/Register", "/controlplane.v1.IntegrationsService/Attach":
				maskArgs(keyvals)
			case "/controlplane.v1.CASBackendService/Create", "/controlplane.v1.CASBackendService/Update":
				maskArgs(keyvals)
			case "/controlplane.v1.AttestationStateService/Save":
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

func newProtoValidator() (*protovalidate.Validator, error) {
	return protovalidate.New()
}
