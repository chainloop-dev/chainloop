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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracer(t *testing.T) {
	tracer := Tracer("test-service", "biz/pkg")
	require.NotNil(t, tracer)
	assert.Equal(t, "biz", tracer.Layer)
	assert.Equal(t, "biz/pkg", tracer.Name)
	assert.Equal(t, "test-service", tracer.ServiceName)
}

func TestTracerLayerExtraction(t *testing.T) {
	tests := []struct {
		name      string
		wantLayer string
	}{
		{"biz/workflow", "biz"},
		{"data/organization", "data"},
		{"middleware/authz", "middleware"},
		{"cas/service/bytestream", "cas"},
		{"standalone", "standalone"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tracer := Tracer("svc", tc.name)
			assert.Equal(t, tc.wantLayer, tracer.Layer)
		})
	}
}

func setupTestProvider(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(sdktrace.NewTracerProvider())
	})

	return exporter
}

func TestStartCreatesSpanWithLayerAttribute(t *testing.T) {
	exporter := setupTestProvider(t)

	tracer := Tracer("test-svc", "biz/test")
	_, span := Start(context.Background(), tracer, "TestOp")
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "TestOp", spans[0].Name)

	found := false
	for _, attr := range spans[0].Attributes {
		if attr.Key == LayerKey && attr.Value == attribute.StringValue("biz") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected chainloop.layer=biz attribute on span")
}

func TestStartDisabledLayer(t *testing.T) {
	exporter := setupTestProvider(t)

	SetDisabledLayers(map[string]bool{"data": true})
	t.Cleanup(func() { SetDisabledLayers(nil) })

	tracer := Tracer("test-svc", "data/test")
	_, span := Start(context.Background(), tracer, "ShouldBeSkipped")
	span.End()

	assert.Empty(t, exporter.GetSpans())
}

func TestRecordError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus codes.Code
	}{
		{
			name:       "nil error does not set error status",
			err:        nil,
			wantStatus: codes.Unset,
		},
		{
			name:       "non-nil error sets error status",
			err:        errors.New("something failed"),
			wantStatus: codes.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exporter := setupTestProvider(t)

			tracer := Tracer("test-svc", "biz/test")
			_, span := Start(context.Background(), tracer, "op")
			RecordError(span, tc.err)
			span.End()

			spans := exporter.GetSpans()
			require.Len(t, spans, 1)
			assert.Equal(t, tc.wantStatus, spans[0].Status.Code)
		})
	}
}
