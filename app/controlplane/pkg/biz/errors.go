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

var ErrAlreadyExists = errors.New("duplicate")

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

type ErrValidation struct {
	err error
}

func NewErrValidationStr(errMsg string) ErrValidation {
	return ErrValidation{errors.New(errMsg)}
}

func NewErrValidation(err error) ErrValidation {
	return ErrValidation{err}
}

func (e ErrValidation) Error() string {
	return fmt.Sprintf("validation error: %s", e.err.Error())
}

func IsErrValidation(err error) bool {
	return errors.As(err, &ErrValidation{})
}

type ErrNotImplemented struct {
	msg string
}

func NewErrNotImplemented(msg string) ErrNotImplemented {
	return ErrNotImplemented{msg}
}

func (e ErrNotImplemented) Error() string {
	return fmt.Sprintf("not implemented: %s", e.msg)
}

func IsErrNotImplemented(err error) bool {
	return errors.As(err, &ErrNotImplemented{})
}

type ErrUnauthorized struct {
	err error
}

func NewErrUnauthorizedStr(errMsg string) ErrUnauthorized {
	return ErrUnauthorized{errors.New(errMsg)}
}

func NewErrUnauthorized(err error) ErrUnauthorized {
	return ErrUnauthorized{err}
}

func (e ErrUnauthorized) Error() string {
	return fmt.Sprintf("authorization error: %s", e.err.Error())
}

func IsErrUnauthorized(err error) bool {
	return errors.As(err, &ErrUnauthorized{})
}

// A referrer with the same digest points to two different artifact types
// and we require filtering out which one
type ErrAmbiguousReferrer struct {
	digest string
	// what kinds contain duplicates
	kinds []string
}

func NewErrReferrerAmbiguous(digest string, kinds []string) error {
	return ErrAmbiguousReferrer{digest, kinds}
}

func (e ErrAmbiguousReferrer) Error() string {
	return fmt.Sprintf("digest %s present in %d kinds %q", e.digest, len(e.kinds), e.kinds)
}
