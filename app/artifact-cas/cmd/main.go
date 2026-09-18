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

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"buf.build/go/protovalidate"
	"github.com/getsentry/sentry-go"

	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/server"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/chainloop-dev/chainloop/pkg/credentials/manager"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"go.opentelemetry.io/otel/trace"

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

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, ms *server.HTTPMetricsServer, providers backend.Providers, _ trace.TracerProvider) *app {
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

	_ = logger.Log(log.LevelInfo, "msg", "starting artifact-cas service", "version", Version)

	// Ensure the upload staging directory is configured and writable before we
	// accept any traffic: without it no upload can be verified.
	if err := prepareStagingDir(&bc); err != nil {
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

	app, cleanup, err := wireApp(&bc, bc.Server, bc.Auth, credentialsReader, logger)
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

func newProtoValidator() (protovalidate.Validator, error) {
	return protovalidate.New()
}

// prepareStagingDir creates the upload staging directory and proves it is
// writable. It must be the same directory the service is configured with (see
// serviceOpts / conf.staging_dir).
//
// staging_dir is required: uploads are verified by spilling them here first, so
// a missing or unwritable directory means no upload can succeed. Failing at
// startup surfaces that immediately, rather than letting the service report
// healthy and reject every upload. The CAS container runs with a read-only root
// filesystem and /tmp is a read-only secret mount, so the configured directory is
// the only place uploads can be staged.
//
// The directory stays clean on its own: each upload removes its staging file on
// every exit path, and the emptyDir backing it is cleared by Kubernetes when the
// Pod is removed from the node.
func prepareStagingDir(bc *conf.Bootstrap) error {
	dir := bc.GetStagingDir()
	if dir == "" {
		return errors.New("staging_dir is required: it must point at a writable, pod-local directory")
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating staging dir %q: %w", dir, err)
	}

	// MkdirAll succeeds on a directory that already exists but cannot be written
	// to, so probe it rather than discovering the problem on the first upload.
	probe, err := os.CreateTemp(dir, ".writable-probe-*")
	if err != nil {
		return fmt.Errorf("staging dir %q is not writable: %w", dir, err)
	}
	_ = probe.Close()
	if err := os.Remove(probe.Name()); err != nil {
		return fmt.Errorf("removing staging dir probe file: %w", err)
	}

	return nil
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
		BeforeSend:       servicelogger.SentryBeforeSend,
	})

	if err == nil {
		_ = logger.Log(log.LevelInfo, "msg", "Sentry initialized", "environment", sentryOpts.Environment, "release", Version)
	}

	return
}
