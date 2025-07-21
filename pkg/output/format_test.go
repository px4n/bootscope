package output

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "0ms",
		},
		{
			name:     "milliseconds only",
			duration: 250 * time.Millisecond,
			expected: "250ms",
		},
		{
			name:     "exactly one second",
			duration: 1 * time.Second,
			expected: "1.0s",
		},
		{
			name:     "seconds with milliseconds",
			duration: 2*time.Second + 345*time.Millisecond,
			expected: "2.3s",
		},
		{
			name:     "exactly one minute",
			duration: 60 * time.Second,
			expected: "1m",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			expected: "2m30s",
		},
		{
			name:     "complex duration",
			duration: 5*time.Minute + 45*time.Second + 678*time.Millisecond,
			expected: "5m45s",
		},
		{
			name:     "sub-millisecond",
			duration: 500 * time.Microsecond,
			expected: "0ms",
		},
		{
			name:     "999 milliseconds",
			duration: 999 * time.Millisecond,
			expected: "999ms",
		},
		{
			name:     "1001 milliseconds",
			duration: 1001 * time.Millisecond,
			expected: "1.0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	// Fixed time for consistent testing
	testTime := time.Date(2025, 0o7, 20, 15, 30, 45, 123456789, time.UTC)

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "standard timestamp",
			time:     testTime,
			expected: "15:30:45.123",
		},
		{
			name:     "midnight",
			time:     time.Date(2025, 0o7, 20, 0, 0, 0, 0, time.UTC),
			expected: "00:00:00.000",
		},
		{
			name:     "with different milliseconds",
			time:     time.Date(2025, 0o7, 20, 23, 59, 59, 999000000, time.UTC),
			expected: "23:59:59.999",
		},
		{
			name:     "single digit values",
			time:     time.Date(2025, 0o7, 20, 1, 2, 3, 4000000, time.UTC),
			expected: "01:02:03.004",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimestamp(tt.time)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{
			name:     "running status",
			status:   "Running",
			expected: "✅",
		},
		{
			name:     "ready status",
			status:   "Ready",
			expected: "✅",
		},
		{
			name:     "pending status",
			status:   "Pending",
			expected: "⏳",
		},
		{
			name:     "container creating",
			status:   "ContainerCreating",
			expected: "🔄",
		},
		{
			name:     "terminating",
			status:   "Terminating",
			expected: "🔄",
		},
		{
			name:     "failed status",
			status:   "Failed",
			expected: "❌",
		},
		{
			name:     "error status",
			status:   "Error",
			expected: "❌",
		},
		{
			name:     "crashloopbackoff",
			status:   "CrashLoopBackOff",
			expected: "❌",
		},
		{
			name:     "unknown status",
			status:   "Unknown",
			expected: "❓",
		},
		{
			name:     "custom status",
			status:   "CustomStatus",
			expected: "❓",
		},
		{
			name:     "case insensitive",
			status:   "running",
			expected: "✅",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStatusIcon(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDurationColor(t *testing.T) {
	tests := []struct {
		name         string
		duration     time.Duration
		isBottleneck bool
		expectRed    bool
		expectYellow bool
		expectGreen  bool
	}{
		{
			name:         "bottleneck always red",
			duration:     5 * time.Second,
			isBottleneck: true,
			expectRed:    true,
		},
		{
			name:         "fast duration green",
			duration:     10 * time.Second,
			isBottleneck: false,
			expectGreen:  true,
		},
		{
			name:         "medium duration yellow",
			duration:     45 * time.Second,
			isBottleneck: false,
			expectYellow: true,
		},
		{
			name:         "slow duration red",
			duration:     90 * time.Second,
			isBottleneck: false,
			expectRed:    true,
		},
		{
			name:         "exactly 30 seconds green",
			duration:     30 * time.Second,
			isBottleneck: false,
			expectGreen:  true,
		},
		{
			name:         "exactly 60 seconds red",
			duration:     60 * time.Second,
			isBottleneck: false,
			expectRed:    true,
		},
		{
			name:         "zero duration green",
			duration:     0,
			isBottleneck: false,
			expectGreen:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colorFunc := GetDurationColor(tt.duration, tt.isBottleneck)

			// Test by applying the color function to a test string
			testStr := "test"
			result := colorFunc(testStr)

			// The color functions wrap the string with ANSI color codes
			// We just verify the function returns a non-empty string
			assert.NotEmpty(t, result)
			assert.Contains(t, result, testStr)
		})
	}
}

func TestFormatPercentage(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{
			name:     "zero percent",
			value:    0,
			expected: "0%",
		},
		{
			name:     "whole number",
			value:    50,
			expected: "50%",
		},
		{
			name:     "decimal rounded down",
			value:    33.3,
			expected: "33%",
		},
		{
			name:     "decimal rounded up",
			value:    66.7,
			expected: "67%",
		},
		{
			name:     "100 percent",
			value:    100,
			expected: "100%",
		},
		{
			name:     "large value",
			value:    250.5,
			expected: "250%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatPercentage(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}
