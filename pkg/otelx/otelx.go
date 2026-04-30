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
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// LayerKey is the span attribute key used to tag spans by architectural layer.
// Values: "service", "biz", "data", "interceptor", "middleware", "job", "consumer".
var LayerKey = attribute.Key("chainloop.layer")

// noopSpan is a pre-allocated span that is safe to call End() on without side effects.
var noopSpan trace.Span = noop.Span{}

var (
	disabledLayers   map[string]bool
	disabledLayersMu sync.RWMutex
)

// SetDisabledLayers configures which layers should not produce spans.
// Call once at startup from the server initialization.
func SetDisabledLayers(layers map[string]bool) {
	disabledLayersMu.Lock()
	defer disabledLayersMu.Unlock()
	disabledLayers = layers
}

func isLayerDisabled(layer string) bool {
	disabledLayersMu.RLock()
	defer disabledLayersMu.RUnlock()

	return disabledLayers[layer]
}

// LayeredTracer carries the layer name for automatic tagging and filtering.
// Created at package init time via Tracer(), but the disabled check happens
// lazily in Start so the config can load first.
type LayeredTracer struct {
	// ServiceName is the service prefix (e.g. "chainloop-controlplane").
	ServiceName string
	// Name is the full scope name (e.g. "biz/workflow").
	Name string
	// Layer is the architectural layer prefix (e.g. "biz", "data", "middleware").
	Layer string
}

// TraceCarrier holds W3C trace context for propagation across async boundaries.
// Embed this in job arg structs so the originating request's trace context is carried to the worker.
type TraceCarrier struct {
	TraceContext map[string]string `json:"trace_context,omitempty"`
}

// InjectTraceContext extracts the current span's trace context into a TraceCarrier.
// Call this when enqueueing a job to capture the originating request's trace.
func InjectTraceContext(ctx context.Context) TraceCarrier {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return TraceCarrier{TraceContext: map[string]string(carrier)}
}

// Tracer returns a LayeredTracer scoped to a chainloop service.
// The serviceName identifies the service ("chainloop-controlplane" or "chainloop-cas").
// The name should follow "layer/component" (e.g. "biz/workflow", "data/organization").
// Safe to call at package init time — the disabled check is deferred to span creation.
func Tracer(serviceName, name string) *LayeredTracer {
	layer := name
	if prefix, _, ok := strings.Cut(name, "/"); ok {
		layer = prefix
	}

	return &LayeredTracer{ServiceName: serviceName, Name: name, Layer: layer}
}

func (t *LayeredTracer) tracer() trace.Tracer {
	return otel.Tracer(t.ServiceName + "/" + t.Name)
}

// Start begins a new span tagged with the chainloop.layer attribute.
// If the layer is disabled, returns a no-op span (zero cost).
func Start(ctx context.Context, tracer *LayeredTracer, spanName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if isLayerDisabled(tracer.Layer) {
		return ctx, noopSpan
	}

	allAttrs := make([]attribute.KeyValue, 0, len(attrs)+1)
	allAttrs = append(allAttrs, LayerKey.String(tracer.Layer))
	allAttrs = append(allAttrs, attrs...)

	return tracer.tracer().Start(ctx, spanName, trace.WithAttributes(allAttrs...))
}

// RecordError records an error on the span and sets its status to Error.
func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
