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

package otelx

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// LayerKey is the span attribute key used to tag spans by architectural layer.
// Values: "service", "biz", "data", "middleware".
var LayerKey = attribute.Key("chainloop.layer")

// LayeredTracer wraps a trace.Tracer with a layer name for automatic tagging.
type LayeredTracer struct {
	trace.Tracer
	Layer string
}

// TraceCarrier holds W3C trace context for propagation across async boundaries.
type TraceCarrier struct {
	TraceContext map[string]string `json:"trace_context,omitempty"`
}

// InjectTraceContext extracts the current span's trace context into a TraceCarrier.
func InjectTraceContext(ctx context.Context) TraceCarrier {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return TraceCarrier{TraceContext: map[string]string(carrier)}
}

// Tracer returns a LayeredTracer scoped to a chainloop service.
// The serviceName identifies the service ("chainloop-controlplane" or "chainloop-cas").
// The name should follow "layer/component" (e.g. "biz/workflow", "data/organization").
// The layer prefix is extracted and added as a span attribute on every Start call.
func Tracer(serviceName, name string) *LayeredTracer {
	layer := name
	if prefix, _, ok := strings.Cut(name, "/"); ok {
		layer = prefix
	}

	return &LayeredTracer{
		Tracer: otel.Tracer(serviceName + "/" + name),
		Layer:  layer,
	}
}

// Start begins a new span tagged with the chainloop.layer attribute.
func Start(ctx context.Context, tracer *LayeredTracer, spanName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	allAttrs := make([]attribute.KeyValue, 0, len(attrs)+1)
	allAttrs = append(allAttrs, LayerKey.String(tracer.Layer))
	allAttrs = append(allAttrs, attrs...)

	return tracer.Start(ctx, spanName, trace.WithAttributes(allAttrs...))
}

// RecordError records an error on the span and sets its status to Error.
func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
