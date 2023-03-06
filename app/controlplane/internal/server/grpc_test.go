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

package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequireCurrentUserMatcher(t *testing.T) {
	testCases := []struct {
		operation string
		matches   bool
	}{
		{"/controlplane.v1.WorkflowService/List", true},
		{"/controlplane.v1.WorkflowRunService/List", true},
		{"/controlplane.v1.StatusService/Infoz", false},
		{"/controlplane.v1.StatusService/Statusz", false},
		{"/controlplane.v1.AttestationService/Init", false},
		{"/controlplane.v1.AttestationService/Store", false},
	}

	matchFunc := requireCurrentUserMatcher()
	for _, op := range testCases {
		assert.Equal(t, matchFunc(context.Background(), op.operation), op.matches)
	}
}

func TestRequireFullyConfiguredOrgMatcher(t *testing.T) {
	testCases := []struct {
		operation string
		matches   bool
	}{
		{"/controlplane.v1.WorkflowService/List", true},
		{"/controlplane.v1.WorkflowRunService/List", true},
		{"/controlplane.v1.OCIRepositoryService/Save", false},
		{"/controlplane.v1.OrganizationService/ListMemberships", false},
		{"/controlplane.v1.OrganizationService/SetCurrent", false},
	}

	matchFunc := requireFullyConfiguredOrgMatcher()
	for _, op := range testCases {
		if got, want := matchFunc(context.Background(), op.operation), op.matches; got != want {
			assert.Equal(t, matchFunc(context.Background(), op.operation), op.matches)
		}
	}
}

func TestRequireRobotAccountMatcher(t *testing.T) {
	testCases := []struct {
		operation string
		matches   bool
	}{
		{"/controlplane.v1.WorkflowService/List", false},
		{"/controlplane.v1.StatusService/Infoz", false},
		{"/controlplane.v1.AttestationService/Init", true},
		{"/controlplane.v1.AttestationService/Store", true},
		{"/controlplane.v1.WorkflowRunService/List", false},
	}

	matchFunc := requireRobotAccountMatcher()
	for _, op := range testCases {
		assert.Equal(t, matchFunc(context.Background(), op.operation), op.matches)
	}
}
