//
// Copyright 2025-2026 The Chainloop Authors.
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

package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine/builtins"

	extism "github.com/extism/go-sdk"
	"github.com/rs/zerolog"
)

// setLogLevelOnce ensures extism.SetLogLevel is called exactly once,
// since it modifies global state and is not safe for concurrent calls.
// Note: the log level is determined by the first NewEngine() call in the
// process. Subsequent engines with different log levels will not override it.
var setLogLevelOnce sync.Once

// Ensure Engine implements PolicyEngine interface
var _ engine.PolicyEngine = (*Engine)(nil)

// entryFunction is the name of the function to call in the WASM module.
// The engine expects this function to be present.
const entryFunction = "Execute"

// Engine implements the PolicyEngine interface for WASM policies
type Engine struct {
	executionTimeout time.Duration
	logger           *zerolog.Logger
	// Embed common engine options
	*engine.CommonEngineOptions
}

// NewEngine creates a new WASM policy engine with the given options
func NewEngine(opts ...engine.Option) *Engine {
	options := engine.ApplyOptions(opts...)

	// Extract WASM-specific options with defaults
	executionTimeout := options.ExecutionTimeout
	if executionTimeout == 0 {
		executionTimeout = 5 * time.Second
	}

	logger := options.Logger
	if logger == nil {
		noopLogger := zerolog.Nop()
		logger = &noopLogger
	}

	// Set WASM plugin log level exactly once across all engine instances.
	// extism.SetLogLevel modifies global state and is not safe for concurrent calls.
	setLogLevelOnce.Do(func() {
		switch {
		case logger.GetLevel() <= zerolog.TraceLevel:
			extism.SetLogLevel(extism.LogLevelTrace)
		case logger.GetLevel() <= zerolog.DebugLevel:
			extism.SetLogLevel(extism.LogLevelDebug)
		case logger.GetLevel() <= zerolog.InfoLevel:
			extism.SetLogLevel(extism.LogLevelInfo)
		case logger.GetLevel() <= zerolog.WarnLevel:
			extism.SetLogLevel(extism.LogLevelWarn)
		default:
			extism.SetLogLevel(extism.LogLevelError)
		}
	})

	return &Engine{
		executionTimeout:    executionTimeout,
		logger:              logger,
		CommonEngineOptions: options.CommonEngineOptions,
	}
}

// Verify executes a WASM policy against the provided input
func (e *Engine) Verify(ctx context.Context, policy *engine.Policy, input []byte, args map[string]any) (*engine.EvaluationResult, error) {
	e.logger.Debug().Str("policy", policy.Name).Int("wasm_size", len(policy.Source)).Int("input_size", len(input)).Int("args_count", len(args)).Msg("Starting WASM policy execution")

	// Prepare config with args if present
	configMap := make(map[string]string)
	if len(args) > 0 {
		// Marshal args to JSON and store in config
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal args: %w", err)
		}
		configMap["args"] = string(argsJSON)
		e.logger.Debug().Str("policy", policy.Name).Int("args_count", len(args)).Msg("Passing policy arguments via Extism config")
	}

	// Create Extism manifest with config and allowed hosts
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmData{Data: policy.Source},
		},
		Config:       configMap,
		AllowedHosts: e.AllowedHostnames,
		Timeout:      uint64(e.executionTimeout.Milliseconds()),
	}

	// Log allowed hosts configuration
	if len(e.AllowedHostnames) > 0 {
		e.logger.Debug().Str("policy", policy.Name).Int("allowed_hosts", len(e.AllowedHostnames)).Strs("hostnames", e.AllowedHostnames).Msg("Configured allowed hosts for HTTP requests")
	}

	config := extism.PluginConfig{
		EnableWasi: true,
	}

	// Register host functions
	// Registers in both "env" (for Go/TinyGo) and "extism:host/user" (for JavaScript) namespaces
	hostFunctions := builtins.CreateDiscoverHostFunctions(e.ControlPlaneConnection)

	// Create plugin with host functions
	plugin, err := extism.NewPlugin(ctx, manifest, config, hostFunctions)
	if err != nil {
		e.logger.Error().Err(err).Str("policy", policy.Name).Msg("Failed to create WASM plugin")
		return nil, fmt.Errorf("failed to create WASM plugin: %w", err)
	}
	defer plugin.Close(ctx)

	// Check if Execute function is exported
	if !plugin.FunctionExists(entryFunction) {
		e.logger.Error().Str("policy", policy.Name).Str("function", entryFunction).Msg("WASM module missing required function export")
		return nil, fmt.Errorf("wasm module validation failed: missing required '%s' function export", entryFunction)
	}

	// Set up logger for WASM plugin output
	plugin.SetLogger(func(level extism.LogLevel, message string) {
		switch level {
		case extism.LogLevelTrace:
			e.logger.Trace().Str("policy", policy.Name).Msg(message)
		case extism.LogLevelDebug:
			e.logger.Debug().Str("policy", policy.Name).Msg(message)
		case extism.LogLevelInfo:
			e.logger.Info().Str("policy", policy.Name).Msg(message)
		case extism.LogLevelWarn:
			e.logger.Warn().Str("policy", policy.Name).Msg(message)
		case extism.LogLevelError:
			e.logger.Error().Str("policy", policy.Name).Msg(message)
		}
	})

	e.logger.Debug().Str("policy", policy.Name).Msg("WASM plugin created successfully")

	// Execute with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.executionTimeout)
	defer cancel()

	e.logger.Debug().Str("policy", policy.Name).Dur("timeout", e.executionTimeout).Msg("Executing WASM policy with raw material input")

	// Pass raw material bytes as input (args are in config)
	exit, output, err := plugin.CallWithContext(execCtx, entryFunction, input)
	if err != nil {
		// Parse the error to provide user-friendly messages
		parsedErr := parseWasmError(err)

		// Log with error category for debugging
		e.logger.Debug().Err(err).Str("policy", policy.Name).Uint32("exit_code", exit).Str("error_category", string(parsedErr.Category)).Msg("WASM policy execution failed")

		// In debug mode, also log the original error with full details
		if e.logger.GetLevel() <= zerolog.DebugLevel {
			e.logger.Debug().Str("policy", policy.Name).Str("original_error", parsedErr.OriginalErr.Error()).Msg("Original WASM error (for debugging)")
		}

		// Return user-friendly error message
		return nil, fmt.Errorf("policy execution failed: %w", parsedErr)
	}

	e.logger.Debug().Str("policy", policy.Name).Int("output_size", len(output)).Msg("WASM policy execution completed")

	// Parse output
	var result struct {
		Skipped    bool              `json:"skipped"`
		Violations []json.RawMessage `json:"violations"`
		Findings   []json.RawMessage `json:"findings"`
		SkipReason string            `json:"skip_reason"`
		Ignore     bool              `json:"ignore"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		e.logger.Error().Err(err).Str("policy", policy.Name).Str("output", string(output)).Msg("Failed to parse WASM policy output")
		return nil, fmt.Errorf("failed to parse policy output: %w", err)
	}

	// Convert to engine.EvaluationResult
	evalResult := &engine.EvaluationResult{
		Skipped:    result.Skipped,
		SkipReason: result.SkipReason,
		Ignore:     result.Ignore,
		Violations: make([]*engine.PolicyViolation, 0),
	}

	// "findings" (structured objects) takes precedence over "violations" (legacy strings)
	if len(result.Findings) > 0 {
		for _, raw := range result.Findings {
			pv, err := parseWasmFinding(policy.Name, raw)
			if err != nil {
				return nil, fmt.Errorf("finding in policy %q: %w", policy.Name, err)
			}
			evalResult.Violations = append(evalResult.Violations, pv)
		}
	} else {
		for _, raw := range result.Violations {
			pv, err := parseWasmStringViolation(policy.Name, raw)
			if err != nil {
				return nil, fmt.Errorf("violation in policy %q: %w", policy.Name, err)
			}
			evalResult.Violations = append(evalResult.Violations, pv)
		}
	}

	e.logger.Debug().Str("policy", policy.Name).Int("violations", len(evalResult.Violations)).Bool("skipped", evalResult.Skipped).Msg("WASM policy evaluation complete")

	// Include raw data if requested
	if e.IncludeRawData {
		evalResult.RawData = &engine.RawData{
			Input:  input,
			Output: output,
		}
	}

	return evalResult, nil
}

// parseWasmFinding parses a single structured finding (object with "message" field).
func parseWasmFinding(policyName string, raw json.RawMessage) (*engine.PolicyViolation, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("finding must be a JSON object: %w", err)
	}
	return engine.NewStructuredViolation(policyName, obj)
}

// parseWasmStringViolation parses a legacy string violation.
func parseWasmStringViolation(policyName string, raw json.RawMessage) (*engine.PolicyViolation, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("violation must be a JSON string: %w", err)
	}
	return &engine.PolicyViolation{
		Subject:   policyName,
		Violation: s,
	}, nil
}

// MatchesParameters is a stub implementation for WASM policies
// WASM policies don't currently support parameter matching
func (e *Engine) MatchesParameters(_ context.Context, _ *engine.Policy,
	_, _ map[string]string) (bool, error) {
	// Default to true - WASM policies handle their own parameter validation
	return true, nil
}

// MatchesEvaluation is a stub implementation for WASM policies
// WASM policies don't currently support evaluation matching
func (e *Engine) MatchesEvaluation(_ context.Context, _ *engine.Policy,
	_ []string, _ map[string]string) (bool, error) {
	// Default to true - WASM policies handle their own evaluation logic
	return true, nil
}
