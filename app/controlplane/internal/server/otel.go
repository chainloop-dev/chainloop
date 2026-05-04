//
// Copyright 2026 The Chainloop Authors.
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

package server

import (
	"context"
	"fmt"
	"time"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const tracerShutdownTimeout = 5 * time.Second

// NewTracerProvider creates an OTel TracerProvider configured from the Bootstrap config.
// When tracing is disabled or not configured, it returns a noop TracerProvider.
func NewTracerProvider(c *conf.Bootstrap, logger log.Logger) (trace.TracerProvider, func(), error) {
	noopCleanup := func() {}

	tracingConf := c.GetObservability().GetTracing()
	if tracingConf == nil || !tracingConf.GetEnabled() {
		_ = logger.Log(log.LevelInfo, "msg", "Tracing is disabled")
		return noop.NewTracerProvider(), noopCleanup, nil
	}

	ctx := context.Background()

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(tracingConf.GetEndpoint()),
	}
	if tracingConf.GetInsecure() {
		opts = append(opts,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		)
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			attribute.String("service.name", "chainloop-controlplane"),
			attribute.String("service.version", Version),
		),
	)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("creating OTel resource: %w", err)
	}

	var sampler sdktrace.Sampler
	if tracingConf.SamplingRatio != nil {
		ratio := tracingConf.GetSamplingRatio()
		switch {
		case ratio <= 0:
			sampler = sdktrace.NeverSample()
		case ratio >= 1.0:
			sampler = sdktrace.AlwaysSample()
		default:
			sampler = sdktrace.TraceIDRatioBased(ratio)
		}
	} else {
		sampler = sdktrace.AlwaysSample()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	cleanup := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), tracerShutdownTimeout)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			_ = logger.Log(log.LevelError, "msg", "Error shutting down TracerProvider", "err", err)
		}
	}

	_ = logger.Log(log.LevelInfo, "msg", "TracerProvider initialized", "endpoint", tracingConf.GetEndpoint())

	return tp, cleanup, nil
}
