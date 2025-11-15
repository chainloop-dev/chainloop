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
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name    string
		def     *BuiltinDef
		wantErr error
	}{
		{
			name: "valid built-in",
			def: &BuiltinDef{
				Name: "test.func",
				Decl: &ast.Builtin{
					Name: "test.func",
					Decl: types.NewFunction(types.Args(types.S), types.S),
				},
				Impl:          func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error { return nil },
				SecurityLevel: SecurityLevelRestrictive,
				Description:   "Test function",
			},
			wantErr: nil,
		},
		{
			name:    "nil built-in",
			def:     nil,
			wantErr: ErrNilBuiltinDef,
		},
		{
			name: "empty name",
			def: &BuiltinDef{
				Name: "",
				Decl: &ast.Builtin{},
				Impl: func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error { return nil },
			},
			wantErr: ErrEmptyBuiltinName,
		},
		{
			name: "nil decl",
			def: &BuiltinDef{
				Name: "test.func",
				Decl: nil,
				Impl: func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error { return nil },
			},
			wantErr: ErrNilBuiltinDecl,
		},
		{
			name: "nil impl",
			def: &BuiltinDef{
				Name: "test.func",
				Decl: &ast.Builtin{},
				Impl: nil,
			},
			wantErr: ErrNilBuiltinImpl,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			err := r.Register(tt.def)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistry_GetByMode(t *testing.T) {
	restrictiveDef := &BuiltinDef{
		Name: "test.restrictive",
		Decl: &ast.Builtin{
			Name: "test.restrictive",
			Decl: types.NewFunction(types.Args(types.S), types.S),
		},
		Impl:          func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error { return nil },
		SecurityLevel: SecurityLevelRestrictive,
		Description:   "Restrictive function",
	}

	permissiveDef := &BuiltinDef{
		Name: "test.permissive",
		Decl: &ast.Builtin{
			Name: "test.permissive",
			Decl: types.NewFunction(types.Args(types.S), types.S),
		},
		Impl:          func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error { return nil },
		SecurityLevel: SecurityLevelPermissive,
		Description:   "Permissive function",
	}

	t.Run("restrictive mode - only restrictive functions", func(t *testing.T) {
		r := NewRegistry()
		require.NoError(t, r.Register(restrictiveDef))
		require.NoError(t, r.Register(permissiveDef))

		defs := r.GetByMode(false) // restrictive mode
		assert.Len(t, defs, 1)
		assert.Equal(t, "test.restrictive", defs[0].Name)
	})

	t.Run("permissive mode - all functions", func(t *testing.T) {
		r := NewRegistry()
		require.NoError(t, r.Register(restrictiveDef))
		require.NoError(t, r.Register(permissiveDef))

		defs := r.GetByMode(true) // permissive mode
		assert.Len(t, defs, 2)

		names := make([]string, len(defs))
		for i, def := range defs {
			names[i] = def.Name
		}
		assert.Contains(t, names, "test.restrictive")
		assert.Contains(t, names, "test.permissive")
	})
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	def := &BuiltinDef{
		Name: "test.func",
		Decl: &ast.Builtin{
			Name: "test.func",
			Decl: types.NewFunction(types.Args(types.S), types.S),
		},
		Impl:          func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error { return nil },
		SecurityLevel: SecurityLevelRestrictive,
		Description:   "Test function",
	}

	require.NoError(t, r.Register(def))

	t.Run("existing function", func(t *testing.T) {
		got, ok := r.Get("test.func")
		assert.True(t, ok)
		assert.Equal(t, def, got)
	})

	t.Run("non-existing function", func(t *testing.T) {
		got, ok := r.Get("test.nonexistent")
		assert.False(t, ok)
		assert.Nil(t, got)
	})
}

func TestRegistry_RegisterGlobal(t *testing.T) {
	r := NewRegistry()

	def1 := &BuiltinDef{
		Name: "test.restrictive_global",
		Decl: &ast.Builtin{
			Name: "test.restrictive_global",
			Decl: types.NewFunction(types.Args(types.S), types.S),
		},
		Impl:          func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error { return nil },
		SecurityLevel: SecurityLevelRestrictive,
	}

	def2 := &BuiltinDef{
		Name: "test.permissive_global",
		Decl: &ast.Builtin{
			Name: "test.permissive_global",
			Decl: types.NewFunction(types.Args(types.S), types.S),
		},
		Impl:          func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error { return nil },
		SecurityLevel: SecurityLevelPermissive,
	}

	require.NoError(t, r.Register(def1))
	require.NoError(t, r.Register(def2))

	t.Run("restrictive mode", func(t *testing.T) {
		err := r.RegisterGlobal(false)
		assert.NoError(t, err)
	})

	t.Run("permissive mode", func(t *testing.T) {
		err := r.RegisterGlobal(true)
		assert.NoError(t, err)
	})
}
