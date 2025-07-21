package constants

import "time"

// Size constants
const (
	// BytesPerMB is the number of bytes that we will consider a megabyte
	BytesPerMB = 1024 * 1024
)

// Time constants
const (
	// DefaultPollInterval is the default interval for polling operations
	DefaultPollInterval = 2 * time.Second
	// DefaultFastPullThreshold is the default threshold for considering an image pull "fast"
	DefaultFastPullThreshold = 5 * time.Second
)

// Network constants
const (
	// DefaultNetworkSpeedMBps is the default assumed network speed in MB/s
	DefaultNetworkSpeedMBps = 25
	// FastNetworkSpeedMBps is the assumed fast network speed in MB/s
	FastNetworkSpeedMBps = 50
)

// Threshold constants
const (
	// DefaultLargeImageSizeMB is the default threshold for large images in MB
	DefaultLargeImageSizeMB = 500
	// DefaultSlowPhaseDurationSeconds is the default threshold for slow phases in seconds
	DefaultSlowPhaseDurationSeconds = 10
)
