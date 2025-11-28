//
// Copyright 2024-2025 The Chainloop Authors.
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

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

// BaseAllowedHostnames are the default hostnames allowed for HTTP requests in policies
var BaseAllowedHostnames = []string{
	"www.chainloop.dev",
	"www.cisa.gov",
}

// CommonEngineOptions contains configuration options shared by all policy engines
type CommonEngineOptions struct {
	AllowedHostnames       []string
	IncludeRawData         bool
	EnablePrint            bool
	ControlPlaneConnection *grpc.ClientConn
}

// Option is a unified functional option for configuring policy engines
type Option func(*Options)

// Options contains all configuration options for policy engines
type Options struct {
	// Common options
	*CommonEngineOptions

	// Rego-specific options
	// OperatingMode defines whether the Rego engine runs in restrictive (0) or permissive (1) mode
	OperatingMode int32

	// WASM-specific options
	ExecutionTimeout time.Duration
	Logger           *zerolog.Logger
}

// WithAllowedHostnames sets the list of allowed hostnames for HTTP requests
// User-provided hostnames are appended to BaseAllowedHostnames
func WithAllowedHostnames(hostnames ...string) Option {
	return func(opts *Options) {
		opts.AllowedHostnames = append(opts.AllowedHostnames, hostnames...)
	}
}

// WithIncludeRawData sets whether to include raw input/output data in results
func WithIncludeRawData(include bool) Option {
	return func(opts *Options) {
		opts.IncludeRawData = include
	}
}

// WithEnablePrint enables print/log statements in policies
func WithEnablePrint(enable bool) Option {
	return func(opts *Options) {
		opts.EnablePrint = enable
	}
}

// WithOperatingMode sets the Rego engine operating mode (restrictive or permissive)
func WithOperatingMode(mode int32) Option {
	return func(opts *Options) {
		opts.OperatingMode = mode
	}
}

// WithExecutionTimeout sets the WASM execution timeout
func WithExecutionTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.ExecutionTimeout = timeout
	}
}

// WithLogger sets the WASM engine logger
func WithLogger(logger *zerolog.Logger) Option {
	return func(opts *Options) {
		opts.Logger = logger
	}
}

// WithGRPCConn sets the gRPC connection for builtin functions like discover
func WithGRPCConn(conn *grpc.ClientConn) Option {
	return func(opts *Options) {
		opts.ControlPlaneConnection = conn
	}
}

// ApplyOptions applies options and returns the configured Options
// This automatically appends BaseAllowedHostnames to any user-provided hostnames
func ApplyOptions(opts ...Option) *Options {
	options := &Options{
		CommonEngineOptions: &CommonEngineOptions{
			AllowedHostnames:       make([]string, 0),
			IncludeRawData:         false,
			EnablePrint:            false,
			ControlPlaneConnection: nil,
		},
		OperatingMode:    0, // Default restrictive mode
		ExecutionTimeout: 0,
		Logger:           nil,
	}

	for _, opt := range opts {
		opt(options)
	}

	// Append base allowed hostnames to user-provided ones
	options.AllowedHostnames = append(options.AllowedHostnames, BaseAllowedHostnames...)

	return options
}

type PolicyEngine interface {
	// Verify verifies an input against a policy
	Verify(ctx context.Context, policy *Policy, input []byte, args map[string]any) (*EvaluationResult, error)
	// MatchesParameters evaluates the matches_parameters rule to determine if evaluation parameters match expected parameters
	MatchesParameters(ctx context.Context, policy *Policy, evaluationParams, expectedParams map[string]string) (bool, error)
	// MatchesEvaluation evaluates the matches_evaluation rule using policy violations and expected parameters
	MatchesEvaluation(ctx context.Context, policy *Policy, violations []string, expectedParams map[string]string) (bool, error)
}

type EvaluationResult struct {
	Violations []*PolicyViolation `json:"violations"`
	Skipped    bool               `json:"skipped"`
	SkipReason string             `json:"skipReason"`
	Ignore     bool               `json:"ignore"`
	RawData    *RawData           `json:"rawData"`
}

type RawData struct {
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
}

// PolicyViolation represents a policy failure
type PolicyViolation struct {
	Subject   string `json:"subject"`
	Violation string `json:"violation"`
}

// Policy represents a loaded policy in any of the supported engines.
type Policy struct {
	// the source code for this policy
	Source []byte `json:"module"`
	// The unique policy name
	Name string `json:"name"`
}

type ResultFormatError struct {
	Field string
}

func (e ResultFormatError) Error() string {
	return fmt.Sprintf("Policy result format error: %s not found or wrong format", e.Field)
}
