//
// Copyright 2024 The Chainloop Authors.
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

package telemetry_test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry"
	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTagsWithEnvironmentInfo(t *testing.T) {
	tags := telemetry.Tags{}.WithRuntimeInfo()

	assert.Equal(t, runtime.GOOS, tags["os"])
	assert.Equal(t, runtime.GOARCH, tags["arch"])
}

func TestTagsWithRunnerInformation(t *testing.T) {
	t.Setenv("CI", "true")
	t.Setenv("GITHUB_REPOSITORY", "chainloop.dev/chainloop")
	t.Setenv("GITHUB_RUN_ID", "123")

	tags := telemetry.Tags{}.WithEnvironmentInfo()

	assert.Contains(t, tags, "ci")
	assert.Contains(t, tags, "runner")
	assert.Equal(t, tags["runner"], "GITHUB_ACTION")
}

func TestCommandTrackerTrackWithDefaultTags(t *testing.T) {
	mockedClient := mocks.NewClient(t)
	expectedEventName := "command_executed" // nolint: goconst

	os.Unsetenv("CI")
	os.Unsetenv("GITHUB_REPOSITORY")
	os.Unsetenv("GITHUB_RUN_ID")

	mockedClient.
		On("TrackEvent", mock.Anything, "command_executed", mock.Anything, mock.Anything).
		Return(func(_ context.Context, eventName string, id string, tags telemetry.Tags) error {
			assert.NotEmpty(t, id)
			assert.Equal(t, expectedEventName, eventName)

			assert.Contains(t, tags, "os")
			assert.Contains(t, tags, "arch")
			assert.Contains(t, tags, "ci")
			assert.NotContains(t, tags, "runner")
			assert.Contains(t, tags, "command")

			return nil
		})
	tracker := telemetry.NewCommandTracker(mockedClient)
	assert.NotNil(t, tracker)

	err := tracker.Track(context.Background(), "test-command", telemetry.Tags{})
	assert.NoError(t, err)

	mockedClient.AssertNumberOfCalls(t, "TrackEvent", 1)
}

func TestCommandTrackerTrackWithCustomTags(t *testing.T) {
	mockedClient := mocks.NewClient(t)
	expectedEventName := "command_executed"

	os.Unsetenv("CI")
	os.Unsetenv("GITHUB_REPOSITORY")
	os.Unsetenv("GITHUB_RUN_ID")

	mockedClient.
		On("TrackEvent", mock.Anything, "command_executed", mock.Anything, mock.Anything).
		Return(func(_ context.Context, eventName string, id string, tags telemetry.Tags) error {
			assert.NotEmpty(t, id)
			assert.Equal(t, expectedEventName, eventName)

			assert.Contains(t, tags, "os")
			assert.Contains(t, tags, "arch")
			assert.Contains(t, tags, "ci")
			assert.NotContains(t, tags, "runner")
			assert.Contains(t, tags, "command")
			assert.Contains(t, tags, "tag1")
			assert.Contains(t, tags, "tag2")

			return nil
		})
	tracker := telemetry.NewCommandTracker(mockedClient)
	assert.NotNil(t, tracker)

	err := tracker.Track(context.Background(), "test-command", telemetry.Tags{
		"tag1": "value1",
		"tag2": "value2",
	})
	assert.NoError(t, err)

	mockedClient.AssertNumberOfCalls(t, "TrackEvent", 1)
}
