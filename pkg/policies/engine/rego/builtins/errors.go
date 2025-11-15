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

import "errors"

var (
	// ErrNilBuiltinDef is returned when a nil BuiltinDef is provided
	ErrNilBuiltinDef = errors.New("built-in definition cannot be nil")

	// ErrEmptyBuiltinName is returned when a built-in name is empty
	ErrEmptyBuiltinName = errors.New("built-in name cannot be empty")

	// ErrNilBuiltinDecl is returned when a built-in declaration is nil
	ErrNilBuiltinDecl = errors.New("built-in declaration cannot be nil")

	// ErrNilBuiltinImpl is returned when a built-in implementation is nil
	ErrNilBuiltinImpl = errors.New("built-in implementation cannot be nil")
)
