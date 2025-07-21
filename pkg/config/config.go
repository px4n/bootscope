package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/px4n/bootscope/pkg/constants"
)

// Config holds all configuration for the pod startup profiler
type Config struct {
	// Thresholds for various analysis criteria
	Thresholds ThresholdConfig `toml:"thresholds"`

	// Display settings for output formatting
	Display DisplayConfig `toml:"display"`

	// Performance assumptions and estimates
	Performance PerformanceConfig `toml:"performance"`

	// Operational parameters
	Operations OperationsConfig `toml:"operations"`

	// Registry detection patterns
	Registry RegistryConfig `toml:"registry"`
}

// ThresholdConfig defines time and size thresholds for analysis
type ThresholdConfig struct {
	// Image size threshold in MB for large image detection
	LargeImageSizeMB int64 `toml:"large_image_size_mb"`

	// General threshold for considering a phase slow (seconds)
	SlowPhaseSeconds int `toml:"slow_phase_seconds"`

	// Specific phase thresholds (seconds)
	ImagePullSlowSeconds     int `toml:"image_pull_slow_seconds"`
	InitContainerSlowSeconds int `toml:"init_container_slow_seconds"`
	AppStartSlowSeconds      int `toml:"app_start_slow_seconds"`
	ResourceWaitingSeconds   int `toml:"resource_waiting_seconds"`

	// Bottleneck detection percentage thresholds
	BottleneckPercentage         float64 `toml:"bottleneck_percentage"`
	BottleneckCriticalPercentage float64 `toml:"bottleneck_critical_percentage"`

	// Minimum total time before showing bottlenecks (seconds)
	BottleneckMinTotalTimeSeconds int `toml:"bottleneck_min_total_time_seconds"`
}

// DisplayConfig defines UI/display settings
type DisplayConfig struct {
	// Phase duration color thresholds (seconds)
	PhaseDurationGreenSeconds  int `toml:"phase_duration_green_seconds"`
	PhaseDurationYellowSeconds int `toml:"phase_duration_yellow_seconds"`

	// Total time emoji thresholds (seconds)
	TotalTimeFastSeconds     int `toml:"total_time_fast_seconds"`
	TotalTimeModerateSeconds int `toml:"total_time_moderate_seconds"`
	TotalTimeSlowSeconds     int `toml:"total_time_slow_seconds"`

	// Formatting options
	ShowMillisecondsUnderSeconds int `toml:"show_milliseconds_under_seconds"`
	DeploymentTopPodsCount       int `toml:"deployment_top_pods_count"`
	DeploymentMinPhaseSeconds    int `toml:"deployment_min_phase_seconds"`
}

// PerformanceConfig defines performance estimates and assumptions
type PerformanceConfig struct {
	// Network speed estimates (MB/s)
	NetworkSpeedAverageMBps int `toml:"network_speed_average_mbps"`
	NetworkSpeedFastMBps    int `toml:"network_speed_fast_mbps"`

	// Time savings estimates (percentage)
	ImagePullOptimizationPercent     int `toml:"image_pull_optimization_percent"`
	InitContainerOptimizationPercent int `toml:"init_container_optimization_percent"`
	LocalRegistryOptimizationPercent int `toml:"local_registry_optimization_percent"`

	// Threshold for considering a pull "fast" (seconds)
	FastPullThresholdSeconds int `toml:"fast_pull_threshold_seconds"`
}

// OperationsConfig defines operational parameters
type OperationsConfig struct {
	// Watch mode settings
	WatchPollIntervalSeconds int    `toml:"watch_poll_interval_seconds"`
	DefaultWatchTimeout      string `toml:"default_watch_timeout"`

	// Pod ready estimation (seconds after container start)
	ReadyTimeEstimationSeconds int `toml:"ready_time_estimation_seconds"`
}

// RegistryConfig defines registry detection patterns
type RegistryConfig struct {
	// Local registry hostnames
	LocalRegistryHosts []string `toml:"local_registry_hosts"`

	// Private network CIDR patterns
	PrivateNetworkCIDRs []string `toml:"private_network_cidrs"`

	// Cluster domain suffixes that indicate local registries
	ClusterDomainSuffixes []string `toml:"cluster_domain_suffixes"`
}

func DefaultConfig() *Config {
	return &Config{
		Thresholds: ThresholdConfig{
			LargeImageSizeMB:              500,
			SlowPhaseSeconds:              10,
			ImagePullSlowSeconds:          30,
			InitContainerSlowSeconds:      20,
			AppStartSlowSeconds:           30,
			ResourceWaitingSeconds:        5,
			BottleneckPercentage:          30,
			BottleneckCriticalPercentage:  50,
			BottleneckMinTotalTimeSeconds: 10, // Don't show bottlenecks for pods < 10s
		},
		Display: DisplayConfig{
			PhaseDurationGreenSeconds:    30,
			PhaseDurationYellowSeconds:   60,
			TotalTimeFastSeconds:         30,
			TotalTimeModerateSeconds:     60,
			TotalTimeSlowSeconds:         120,
			ShowMillisecondsUnderSeconds: 10,
			DeploymentTopPodsCount:       3,
			DeploymentMinPhaseSeconds:    1,
		},
		Performance: PerformanceConfig{
			NetworkSpeedAverageMBps:          25,
			NetworkSpeedFastMBps:             50,
			ImagePullOptimizationPercent:     50,
			InitContainerOptimizationPercent: 33,
			LocalRegistryOptimizationPercent: 80,
			FastPullThresholdSeconds:         5,
		},
		Operations: OperationsConfig{
			WatchPollIntervalSeconds:   2,
			DefaultWatchTimeout:        "5m",
			ReadyTimeEstimationSeconds: 5,
		},
		Registry: RegistryConfig{
			LocalRegistryHosts: []string{
				"localhost",
				"127.0.0.1",
				"host.docker.internal",
			},
			PrivateNetworkCIDRs: []string{
				"10.0.0.0/8",
				"172.16.0.0/12",
				"192.168.0.0/16",
			},
			ClusterDomainSuffixes: []string{
				".cluster.local",
				".svc.cluster.local",
			},
		},
	}
}

// LoadConfig loads configuration from a TOML file, falling back to defaults
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	// If no path specified, try to find config in standard locations
	if path == "" {
		path = findConfigFile()
	}

	// If still no path or file doesn't exist, return defaults
	if path == "" {
		return config, nil
	}

	// Validate path is safe
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	// Security check: ensure the path doesn't contain directory traversal
	cleanPath := filepath.Clean(absPath)
	if cleanPath != absPath || strings.Contains(path, "..") {
		return nil, fmt.Errorf("config path contains directory traversal")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, use defaults
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse TOML into config
	if err := toml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// findConfigFile looks for config file in standard locations
func findConfigFile() string {
	// Check locations in order of priority
	locations := []string{
		"./bootscope.toml",
		"~/.kube/bootscope.toml",
		"/etc/kubectl-bootscope/config.toml",
	}

	for _, loc := range locations {
		// Expand home directory
		if loc[0] == '~' {
			home, err := os.UserHomeDir()
			if err == nil {
				loc = filepath.Join(home, loc[1:])
			}
		}

		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

// SaveDefaultConfig writes the default configuration to a file with comments
func SaveDefaultConfig(path string) error {
	if path == "" {
		path = "./bootscope.toml"
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the embedded template
	template := getConfigTemplate()
	if err := os.WriteFile(path, []byte(template), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func getConfigTemplate() string {
	return `# BootScope Configuration Example
# Copy this file to bootscope.toml and adjust values as needed
#
# Configuration file locations (in order of priority):
# 1. ./bootscope.toml (current directory)
# 2. ~/.kube/bootscope.toml
# 3. /etc/kubectl-bootscope/config.toml

# Threshold Configuration
# These values determine when the profiler considers various metrics to be problematic
[thresholds]

# Large image size threshold in MB
# Images larger than this will trigger optimization recommendations
# Default: 500MB (based on container image best practices)
large_image_size_mb = 500

# General slow phase threshold in seconds
# Any phase taking longer than this is considered slow
# Used for general bottleneck detection
# Default: 10 seconds
slow_phase_seconds = 10

# Image pull slow threshold in seconds
# Image pulls taking longer indicate network issues or oversized images
# Triggers image optimization and registry recommendations
# Default: 30 seconds (reasonable for up to 1GB images on good networks)
image_pull_slow_seconds = 30

# Init container slow threshold in seconds
# Init containers should perform quick setup tasks
# Longer durations suggest they're doing too much work
# Default: 20 seconds
init_container_slow_seconds = 20

# Application startup slow threshold in seconds
# Time from container start to ready state
# Longer times indicate initialization issues
# Default: 30 seconds
app_start_slow_seconds = 30

# Resource waiting threshold in seconds
# Time spent waiting for CPU, memory, or node resources
# Any wait longer than this indicates cluster capacity issues
# Default: 5 seconds
resource_waiting_seconds = 5

# Bottleneck detection percentage threshold
# Phases using more than this percentage of total time are flagged
# Default: 30% (any phase using >30% of total time is disproportionate)
bottleneck_percentage = 30.0

# Critical bottleneck percentage threshold
# Phases exceeding this are marked as critical severity
# Default: 50% (half or more of total startup time)
bottleneck_critical_percentage = 50.0

# Minimum total time before showing bottlenecks (seconds)
# If pod starts faster than this, only show bottlenecks for phases that are individually slow
# Default: 10 seconds
bottleneck_min_total_time_seconds = 10


# Display Configuration
# Controls how information is presented in the CLI output
[display]

# Phase duration color thresholds in seconds
# Green: duration < green_seconds
# Yellow: green_seconds <= duration < yellow_seconds
# Red: duration >= yellow_seconds
phase_duration_green_seconds = 30
phase_duration_yellow_seconds = 60

# Total pod startup time emoji thresholds in seconds
# ✅ Fast: < fast_seconds
# ⏱ Moderate: fast_seconds <= time < moderate_seconds
# ⚠️ Slow: moderate_seconds <= time < slow_seconds
# 🐌 Very slow: >= slow_seconds
total_time_fast_seconds = 30
total_time_moderate_seconds = 60
total_time_slow_seconds = 120

# Show milliseconds for durations under this many seconds
# Provides relevant precision for different time scales
# Default: 10 seconds
show_milliseconds_under_seconds = 10

# Number of slowest pods to show details for in deployment analysis
# Default: 3 (focus on the worst performers)
deployment_top_pods_count = 3

# Minimum phase duration in seconds to show in deployment view
# Filters out trivial phases to reduce noise
# Default: 1 second
deployment_min_phase_seconds = 1


# Performance Configuration
# Estimates and assumptions used for analysis and recommendations
[performance]

# Average network speed in MB/s for image size estimation
# Used when actual image size is unknown
# Default: 25 MB/s (200 Mbps - typical cloud provider speed)
network_speed_average_mbps = 25

# Fast network speed in MB/s for cached/local pulls
# Used for pulls completed in under fast_pull_threshold_seconds
# Default: 50 MB/s (400 Mbps - LAN or same-region speeds)
network_speed_fast_mbps = 50

# Estimated time savings from image pull optimization (percentage)
# Used in recommendations for impact calculation
# Default: 50% (conservative estimate for using slimmer images)
image_pull_optimization_percent = 50

# Estimated time savings from init container optimization (percentage)
# Used for calculating impact of parallelizing or optimizing init containers
# Default: 33% (assume 1/3 reduction through better design)
init_container_optimization_percent = 33

# Estimated time savings from using local registry (percentage)
# Impact of using cluster-local registry vs remote
# Default: 80% (dramatic improvement for large images)
local_registry_optimization_percent = 80

# Threshold for considering an image pull "fast" in seconds
# Pulls faster than this use the fast network speed estimate
# Default: 5 seconds
fast_pull_threshold_seconds = 5


# Operations Configuration
# Runtime behavior and operational parameters
[operations]

# Watch mode poll interval in seconds
# How often to check pod status when using --watch flag
# Default: 2 seconds (balance between responsiveness and API load)
watch_poll_interval_seconds = 2

# Default timeout for watch mode
# Maximum time to wait for pod to become ready
# Format: Go duration string (e.g., "5m", "300s", "1h30m")
# Default: "5m" (most pods should start within 5 minutes)
default_watch_timeout = "5m"

# Ready time estimation in seconds after container start
# Used as fallback when Ready condition has stale timestamp (pod restarts)
# Default: 5 seconds (reasonable for most applications)
ready_time_estimation_seconds = 5


# Registry Configuration
# Patterns for detecting local vs remote container registries
[registry]

# Local registry hostnames
# These indicate registries with fast, cluster-local access
# Add your private registry hostnames here
local_registry_hosts = [
    "localhost",
    "127.0.0.1",
    "host.docker.internal"
]

# Private network CIDR blocks
# IP ranges that indicate cluster-local or private registries
# Uses standard private IPv4 address ranges by default
private_network_cidrs = [
    "10.0.0.0/8",      # Class A private network
    "172.16.0.0/12",   # Class B private network
    "192.168.0.0/16"   # Class C private network
]

# Cluster domain suffixes
# Domain suffixes that indicate cluster-local services
# Add your cluster's domain if different
cluster_domain_suffixes = [
    ".cluster.local",
    ".svc.cluster.local"
]
`
}

// Convenience methods for getting values with proper types

func (c *Config) GetLargeImageThreshold() int64 {
	return c.Thresholds.LargeImageSizeMB * constants.BytesPerMB
}

func (c *Config) GetSlowPhaseThreshold() time.Duration {
	return time.Duration(c.Thresholds.SlowPhaseSeconds) * time.Second
}

func (c *Config) GetImagePullSlowThreshold() time.Duration {
	return time.Duration(c.Thresholds.ImagePullSlowSeconds) * time.Second
}

func (c *Config) GetInitContainerSlowThreshold() time.Duration {
	return time.Duration(c.Thresholds.InitContainerSlowSeconds) * time.Second
}

func (c *Config) GetAppStartSlowThreshold() time.Duration {
	return time.Duration(c.Thresholds.AppStartSlowSeconds) * time.Second
}

func (c *Config) GetResourceWaitingThreshold() time.Duration {
	return time.Duration(c.Thresholds.ResourceWaitingSeconds) * time.Second
}

func (c *Config) GetWatchPollInterval() time.Duration {
	return time.Duration(c.Operations.WatchPollIntervalSeconds) * time.Second
}

func (c *Config) GetReadyTimeEstimation() time.Duration {
	return time.Duration(c.Operations.ReadyTimeEstimationSeconds) * time.Second
}

func (c *Config) GetNetworkSpeedAverage() int64 {
	return int64(c.Performance.NetworkSpeedAverageMBps) * constants.BytesPerMB
}

func (c *Config) GetNetworkSpeedFast() int64 {
	return int64(c.Performance.NetworkSpeedFastMBps) * constants.BytesPerMB
}

func (c *Config) GetFastPullThreshold() time.Duration {
	return time.Duration(c.Performance.FastPullThresholdSeconds) * time.Second
}

func (c *Config) GetBottleneckMinTotalTime() time.Duration {
	return time.Duration(c.Thresholds.BottleneckMinTotalTimeSeconds) * time.Second
}
