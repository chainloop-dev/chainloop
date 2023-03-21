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

package servicelogger

import (
	"io"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/getsentry/sentry-go"
	zap_plugin "github.com/go-kratos/kratos/contrib/log/zap/v2"
	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

const Dev = "dev"

func InitZapLogger(env string) (log.Logger, error) {
	var err error

	cfg := zap.NewProductionConfig()
	if env == Dev {
		cfg = zap.NewDevelopmentConfig()
	}
	cfg.EncoderConfig.MessageKey = zapcore.OmitKey

	zapLogger, err := cfg.Build(zap.WithCaller(false))
	if err != nil {
		return nil, err
	}

	defer func() {
		err = zapLogger.Sync() // flushes buffer, if any
	}()

	return log.With(zap_plugin.NewLogger(zapLogger)), nil
}

// LogAndMaskErr records an error but masks it with a generic one
// to avoid leaking sensitive information
func LogAndMaskErr(in error, logger *log.Helper) error {
	if logger != nil {
		logger.Error(in)
	}

	sentry.CaptureException(in)
	return errors.InternalServer("internal error", "server error")
}

// ScopedHelper returns a new helper with information about the current component
func ScopedHelper(logger log.Logger, scope string) *log.Helper {
	if logger == nil {
		logger = log.NewStdLogger(io.Discard)
	}

	return log.NewHelper(log.With(logger, "component", scope))
}

func EmptyLogger() *log.Helper {
	return log.NewHelper(log.NewStdLogger(io.Discard))
}
