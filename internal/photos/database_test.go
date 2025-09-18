package photos

import (
	"testing"
	"time"

	"github.com/grantbirki/gh-photos/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestCoreDataTimeToGoTime(t *testing.T) {
	tests := []struct {
		name     string
		seconds  float64
		expected time.Time
	}{
		{
			name:     "epoch time",
			seconds:  0,
			expected: time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "one day later",
			seconds:  86400, // 24 * 60 * 60
			expected: time.Date(2001, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "one hour later",
			seconds:  3600, // 60 * 60
			expected: time.Date(2001, 1, 1, 1, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coreDataTimeToGoTime(tt.seconds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClassifyAsset(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		flags    types.AssetFlags
		expected types.AssetType
	}{
		{
			name:     "screenshot takes precedence",
			filename: "IMG_001.HEIC",
			flags:    types.AssetFlags{Screenshot: true},
			expected: types.AssetTypeScreenshot,
		},
		{
			name:     "live photo takes precedence over extension",
			filename: "IMG_002.HEIC",
			flags:    types.AssetFlags{LivePhoto: true},
			expected: types.AssetTypeLivePhoto,
		},
		{
			name:     "burst photo",
			filename: "IMG_003.HEIC",
			flags:    types.AssetFlags{Burst: true},
			expected: types.AssetTypeBurst,
		},
		{
			name:     "regular photo",
			filename: "IMG_004.HEIC",
			flags:    types.AssetFlags{},
			expected: types.AssetTypePhoto,
		},
		{
			name:     "regular video",
			filename: "VID_001.MOV",
			flags:    types.AssetFlags{},
			expected: types.AssetTypeVideo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyAsset(tt.filename, tt.flags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Note: Testing the database functions would require creating a test SQLite database
// with the appropriate schema, which is more complex and would be done in integration tests
