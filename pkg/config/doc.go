// Package config provides configuration management for the BootScope tool.
//
// This package handles loading, parsing, and providing access to configuration
// values from TOML files. It supports default values, environment-specific
// overrides, and automatic configuration file discovery.
//
// # Configuration File
//
// BootScope looks for configuration in the following locations:
//  1. Path specified via --config flag
//  2. ./bootscope.toml (current directory)
//  3. ~/.kube/bootscope.toml (home directory)
//
// # Configuration Structure
//
// The configuration file uses TOML format:
//
//	[analysis]
//	large_image_threshold_mb = 500
//	slow_phase_threshold_seconds = 10
//
//	[operations]
//	default_watch_timeout = "5m"
//	watch_poll_interval = "2s"
//
//	[display]
//	total_time_slow_seconds = 60
//	phase_duration_yellow_seconds = 30
//
//	[registry]
//	local_registry_hosts = ["localhost", "registry.local"]
//	cluster_domain_suffixes = [".cluster.local"]
//
// # Usage
//
// Load configuration:
//
//	cfg, err := config.LoadConfig("") // Use default search paths
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access configuration values
//	threshold := cfg.GetLargeImageThreshold() // Returns bytes
//	timeout := cfg.GetWatchPollInterval()     // Returns time.Duration
//
// Generate a default configuration file:
//
//	err := config.SaveDefaultConfig("./bootscope.toml")
package config
