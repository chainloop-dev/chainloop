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

package builtins

import (
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// SecurityLevel defines when a built-in function is allowed to execute
type SecurityLevel int

const (
	// SecurityLevelPermissive functions are only available in permissive (development) mode
	// These functions may make external calls, modify state, or perform operations
	// that are not suitable for production policy evaluation
	SecurityLevelPermissive SecurityLevel = iota

	// SecurityLevelRestrictive functions are safe to use in restrictive (production) mode
	// These functions should be read-only, deterministic, and not make external calls
	// or access sensitive resources
	SecurityLevelRestrictive
)

// BuiltinDef defines a custom built-in function that can be registered with OPA
type BuiltinDef struct {
	// Name is the fully qualified name of the built-in (e.g., "chainloop.http_with_auth")
	Name string

	// Decl is the built-in declaration that defines the function signature
	Decl *ast.Builtin

	// Impl is the actual function implementation
	Impl topdown.BuiltinFunc

	// SecurityLevel defines when this function is allowed to execute
	SecurityLevel SecurityLevel

	// Description provides documentation for the function
	Description string
}

// Registry manages custom built-in functions for the OPA policy engine
type Registry struct {
	mu       sync.RWMutex
	builtins map[string]*BuiltinDef
}

// NewRegistry creates a new built-in function registry
func NewRegistry() *Registry {
	return &Registry{
		builtins: make(map[string]*BuiltinDef),
	}
}

// Register adds a built-in function to the registry
func (r *Registry) Register(def *BuiltinDef) error {
	if def == nil {
		return ErrNilBuiltinDef
	}
	if def.Name == "" {
		return ErrEmptyBuiltinName
	}
	if def.Decl == nil {
		return ErrNilBuiltinDecl
	}
	if def.Impl == nil {
		return ErrNilBuiltinImpl
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.builtins[def.Name] = def
	return nil
}

// GetByMode returns all built-in functions that are allowed in the specified security level
// Functions with SecurityLevelRestrictive are available in both modes
func (r *Registry) GetByMode(isPermissive bool) []*BuiltinDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*BuiltinDef
	for _, def := range r.builtins {
		// Restrictive functions are always available
		if def.SecurityLevel == SecurityLevelRestrictive {
			result = append(result, def)
			continue
		}

		// Permissive functions only available in permissive mode
		if isPermissive && def.SecurityLevel == SecurityLevelPermissive {
			result = append(result, def)
		}
	}

	return result
}

// Get returns a built-in function by name
func (r *Registry) Get(name string) (*BuiltinDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, ok := r.builtins[name]
	return def, ok
}

// All returns all registered built-in functions
func (r *Registry) All() []*BuiltinDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*BuiltinDef, 0, len(r.builtins))
	for _, def := range r.builtins {
		result = append(result, def)
	}

	return result
}

// RegisterGlobal registers built-ins globally with OPA based on security mode
// This should be called once during initialization
func (r *Registry) RegisterGlobal(isPermissive bool) error {
	defs := r.GetByMode(isPermissive)

	for _, def := range defs {
		// Register the built-in declaration with AST
		ast.RegisterBuiltin(def.Decl)

		// Register the implementation with topdown
		topdown.RegisterBuiltinFunc(def.Name, def.Impl)
	}

	return nil
}

// Global registry instance for default built-ins
var globalRegistry = NewRegistry()

// Register adds a built-in to the global registry
func Register(def *BuiltinDef) error {
	return globalRegistry.Register(def)
}

// GetByMode returns built-ins from the global registry by mode
func GetByMode(isPermissive bool) []*BuiltinDef {
	return globalRegistry.GetByMode(isPermissive)
}

// Get returns a built-in from the global registry by name
func Get(name string) (*BuiltinDef, bool) {
	return globalRegistry.Get(name)
}

// All returns all built-ins from the global registry
func All() []*BuiltinDef {
	return globalRegistry.All()
}

// RegisterGlobalBuiltins registers global registry built-ins with OPA
func RegisterGlobalBuiltins(isPermissive bool) error {
	return globalRegistry.RegisterGlobal(isPermissive)
}
