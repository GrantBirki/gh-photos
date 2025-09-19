package main

import (
	"strings"
	"testing"
	"time"

	"github.com/grantbirki/gh-photos/internal/audit"
	"github.com/stretchr/testify/assert"
)

func TestBuildSyncCommand(t *testing.T) {
	tests := []struct {
		name       string
		invocation audit.Invocation
		sourcePath string
		expected   string
	}{
		{
			name: "basic sync command",
			invocation: audit.Invocation{
				Remote: "s3:my-bucket/photos",
				Flags:  audit.InvocationFlags{},
			},
			sourcePath: "/path/to/extracted",
			expected:   "sync /path/to/extracted s3:my-bucket/photos",
		},
		{
			name: "sync command with common flags",
			invocation: audit.Invocation{
				Remote: "s3:my-bucket/photos",
				Flags: audit.InvocationFlags{
					IncludeHidden:          true,
					IncludeRecentlyDeleted: true,
					Parallel:               4,
					SkipExisting:           true,
				},
			},
			sourcePath: "/path/to/extracted",
			expected:   "sync /path/to/extracted s3:my-bucket/photos --include-hidden --include-recently-deleted --parallel=4 --skip-existing",
		},
		{
			name: "sync command with date filters",
			invocation: audit.Invocation{
				Remote: "gdrive:Photos",
				Flags: audit.InvocationFlags{
					StartDate: func() *time.Time { t := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC); return &t }(),
					EndDate:   func() *time.Time { t := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC); return &t }(),
					Types:     []string{"photo", "video"},
				},
			},
			sourcePath: "/path/to/extracted",
			expected:   "sync /path/to/extracted gdrive:Photos --types=photo,video --start-date=2023-01-01 --end-date=2023-12-31",
		},
		// Root flag removed; comprehensive test updated accordingly
		{
			name: "sync command with all flags",
			invocation: audit.Invocation{
				Remote: "dropbox:Photos",
				Flags: audit.InvocationFlags{
					IncludeHidden:          true,
					IncludeRecentlyDeleted: true,
					Parallel:               2,
					SkipExisting:           true,
					DryRun:                 true,
					LogLevel:               "debug",
					Types:                  []string{"photo"},
					Verify:                 true,
					Checksum:               true,
				},
			},
			sourcePath: "/path/to/backup",
			expected:   "sync /path/to/backup dropbox:Photos --include-hidden --include-recently-deleted --parallel=2 --skip-existing --dry-run --log-level=debug --types=photo --verify --checksum",
		},
		{
			name: "sync command with default parallel (should not include)",
			invocation: audit.Invocation{
				Remote: "s3:bucket",
				Flags: audit.InvocationFlags{
					Parallel: 1, // Default value should not be included
				},
			},
			sourcePath: "/path/to/extracted",
			expected:   "sync /path/to/extracted s3:bucket",
		},
		{
			name: "sync command with default log level (should not include)",
			invocation: audit.Invocation{
				Remote: "s3:bucket",
				Flags: audit.InvocationFlags{
					LogLevel: "info", // Default value should not be included
				},
			},
			sourcePath: "/path/to/extracted",
			expected:   "sync /path/to/extracted s3:bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSyncCommand(tt.invocation, tt.sourcePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildSyncCommandFlagOrder(t *testing.T) {
	// Test that flags are consistently ordered
	invocation := audit.Invocation{
		Remote: "s3:bucket",
		Flags: audit.InvocationFlags{
			Verify:       true,
			SkipExisting: true,
			DryRun:       true,
		},
	}

	result := buildSyncCommand(invocation, "/path")

	// Should maintain consistent flag order regardless of struct field order
	expectedFlags := []string{"--skip-existing", "--dry-run", "--verify"}

	for i, flag := range expectedFlags {
		parts := strings.Split(result, " ")
		foundIndex := -1
		for j, part := range parts {
			if part == flag {
				foundIndex = j
				break
			}
		}
		assert.NotEqual(t, -1, foundIndex, "Flag %s should be present", flag)

		// Check relative order (each flag should come after the previous one)
		if i > 0 {
			prevFlag := expectedFlags[i-1]
			prevIndex := -1
			for j, part := range parts {
				if part == prevFlag {
					prevIndex = j
					break
				}
			}
			assert.True(t, foundIndex > prevIndex, "Flag %s should come after %s", flag, prevFlag)
		}
	}
}
