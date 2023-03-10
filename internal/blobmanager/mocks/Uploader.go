// Code generated by mockery v2.20.0. DO NOT EDIT.

package mocks

import (
	context "context"
	io "io"

	mock "github.com/stretchr/testify/mock"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
)

// Uploader is an autogenerated mock type for the Uploader type
type Uploader struct {
	mock.Mock
}

// CheckWritePermissions provides a mock function with given fields: ctx
func (_m *Uploader) CheckWritePermissions(ctx context.Context) error {
	ret := _m.Called(ctx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Exists provides a mock function with given fields: ctx, digest
func (_m *Uploader) Exists(ctx context.Context, digest string) (bool, error) {
	ret := _m.Called(ctx, digest)

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (bool, error)); ok {
		return rf(ctx, digest)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) bool); ok {
		r0 = rf(ctx, digest)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, digest)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Upload provides a mock function with given fields: ctx, r, resource
func (_m *Uploader) Upload(ctx context.Context, r io.Reader, resource *v1.CASResource) error {
	ret := _m.Called(ctx, r, resource)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, io.Reader, *v1.CASResource) error); ok {
		r0 = rf(ctx, r, resource)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewUploader interface {
	mock.TestingT
	Cleanup(func())
}

// NewUploader creates a new instance of Uploader. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewUploader(t mockConstructorTestingTNewUploader) *Uploader {
	mock := &Uploader{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
