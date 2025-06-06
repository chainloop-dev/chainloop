// Code generated by mockery v2.53.4. DO NOT EDIT.

package mocks

import (
	context "context"

	biz "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"

	mock "github.com/stretchr/testify/mock"
)

// CASBackendReader is an autogenerated mock type for the CASBackendReader type
type CASBackendReader struct {
	mock.Mock
}

// FindByIDInOrg provides a mock function with given fields: ctx, OrgID, ID
func (_m *CASBackendReader) FindByIDInOrg(ctx context.Context, OrgID string, ID string) (*biz.CASBackend, error) {
	ret := _m.Called(ctx, OrgID, ID)

	if len(ret) == 0 {
		panic("no return value specified for FindByIDInOrg")
	}

	var r0 *biz.CASBackend
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*biz.CASBackend, error)); ok {
		return rf(ctx, OrgID, ID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *biz.CASBackend); ok {
		r0 = rf(ctx, OrgID, ID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*biz.CASBackend)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, OrgID, ID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FindDefaultBackend provides a mock function with given fields: ctx, orgID
func (_m *CASBackendReader) FindDefaultBackend(ctx context.Context, orgID string) (*biz.CASBackend, error) {
	ret := _m.Called(ctx, orgID)

	if len(ret) == 0 {
		panic("no return value specified for FindDefaultBackend")
	}

	var r0 *biz.CASBackend
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*biz.CASBackend, error)); ok {
		return rf(ctx, orgID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *biz.CASBackend); ok {
		r0 = rf(ctx, orgID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*biz.CASBackend)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, orgID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PerformValidation provides a mock function with given fields: ctx, ID
func (_m *CASBackendReader) PerformValidation(ctx context.Context, ID string) error {
	ret := _m.Called(ctx, ID)

	if len(ret) == 0 {
		panic("no return value specified for PerformValidation")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, ID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewCASBackendReader creates a new instance of CASBackendReader. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewCASBackendReader(t interface {
	mock.TestingT
	Cleanup(func())
}) *CASBackendReader {
	mock := &CASBackendReader{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
