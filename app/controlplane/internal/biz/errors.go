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

package biz

import (
	"errors"
	"fmt"
)

type ErrNotFound struct {
	entity string
}

func NewErrNotFound(entity string) ErrNotFound {
	return ErrNotFound{entity}
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.entity)
}

func IsNotFound(err error) bool {
	return errors.As(err, &ErrNotFound{})
}

type ErrInvalidUUID struct {
	err error
}

func NewErrInvalidUUID(err error) ErrInvalidUUID {
	return ErrInvalidUUID{err}
}

func (e ErrInvalidUUID) Error() string {
	return fmt.Sprintf("invalid UUID: %s", e.err.Error())
}

func IsErrInvalidUUID(err error) bool {
	return errors.As(err, &ErrInvalidUUID{})
}

type errValidation struct {
	err error
}

func NewErrValidation(err error) errValidation {
	return errValidation{err}
}

func (e errValidation) Error() string {
	return fmt.Sprintf("validation error: %s", e.err.Error())
}

func IsErrValidation(err error) bool {
	return errors.As(err, &errValidation{})
}
