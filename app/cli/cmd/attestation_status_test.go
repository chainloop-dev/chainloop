package cmd

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/stretchr/testify/assert"
)

func TestVersionStringAttestation(t *testing.T) {
	testCases := []struct {
		name     string
		version  *action.ProjectVersion
		expected string
	}{
		{
			name: "empty version",
			version: &action.ProjectVersion{
				Version: "",
			},
			expected: "",
		},
		{
			name: "prerelease version to be released",
			version: &action.ProjectVersion{
				Version:        "1.0.0",
				Prerelease:     true,
				MarkAsReleased: true,
			},
			expected: "1.0.0 (will release)",
		},
		{
			name: "already released version",
			version: &action.ProjectVersion{
				Version:    "1.0.0",
				Prerelease: false,
			},
			expected: "1.0.0 (already released)",
		},
		{
			name: "prerelease version",
			version: &action.ProjectVersion{
				Version:    "1.0.0-rc1",
				Prerelease: true,
			},
			expected: "1.0.0-rc1 (prerelease)",
		},
		{
			name: "prerelease version not marked for release",
			version: &action.ProjectVersion{
				Version:        "2.0.0-beta",
				Prerelease:     true,
				MarkAsReleased: false,
			},
			expected: "2.0.0-beta (prerelease)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := versionStringAttestation(tc.version)
			assert.Equal(t, tc.expected, result)
		})
	}
}
