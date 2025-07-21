// Package constants defines shared constants used throughout BootScope.
//
// This package contains default values and constants that are used by multiple
// packages within the application. These values serve as fallbacks when
// configuration is not provided.
//
// # Categories
//
// Size constants:
//   - BytesPerMB: Number of assumed bytes in a megabyte (1024*1024)
//
// Time constants:
//   - DefaultPollInterval: Default interval for watch operations (2s)
//   - DefaultFastPullThreshold: Threshold for "fast" image pulls (5s)
//
// Network constants:
//   - DefaultNetworkSpeedMBps: Assumed average network speed (25 MB/s)
//   - FastNetworkSpeedMBps: Assumed fast network speed (50 MB/s)
//
// Threshold constants:
//   - DefaultLargeImageSizeMB: Threshold for large images (500 MB)
//   - DefaultSlowPhaseDurationSeconds: Threshold for slow phases (10s)
//
// # Usage
//
// These constants are primarily used as defaults in configuration:
//
//	cfg := &Config{
//	    NetworkSpeedAverage: constants.DefaultNetworkSpeedMBps * constants.BytesPerMB,
//	    LargeImageThreshold: constants.DefaultLargeImageSizeMB * constants.BytesPerMB,
//	}
package constants
