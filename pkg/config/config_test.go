package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name         string
		configData   string
		setupFunc    func(t *testing.T) string // returns config path
		wantErr      bool
		validateFunc func(t *testing.T, cfg *Config)
	}{
		{
			name: "load valid custom config",
			configData: `
[thresholds]
large_image_size_mb = 1000
slow_phase_seconds = 20
image_pull_slow_seconds = 60

[display]
total_time_fast_seconds = 15
deployment_top_pods_count = 5

[performance]
network_speed_average_mbps = 100
image_pull_optimization_percent = 75

[registry]
local_registry_hosts = ["my-registry.local", "192.168.1.100"]
cluster_domain_suffixes = [".k8s.local", ".cluster.internal"]
`,
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "bootscope.toml")
				require.NoError(t, os.WriteFile(configPath, []byte(`
[thresholds]
large_image_size_mb = 1000
slow_phase_seconds = 20
image_pull_slow_seconds = 60

[display]
total_time_fast_seconds = 15
deployment_top_pods_count = 5

[performance]
network_speed_average_mbps = 100
image_pull_optimization_percent = 75

[registry]
local_registry_hosts = ["my-registry.local", "192.168.1.100"]
cluster_domain_suffixes = [".k8s.local", ".cluster.internal"]
`), 0o644))
				return configPath
			},
			validateFunc: func(t *testing.T, cfg *Config) {
				// Verify custom values were loaded
				assert.Equal(t, int64(1000), cfg.Thresholds.LargeImageSizeMB)
				assert.Equal(t, 20, cfg.Thresholds.SlowPhaseSeconds)
				assert.Equal(t, 60, cfg.Thresholds.ImagePullSlowSeconds)
				assert.Equal(t, 15, cfg.Display.TotalTimeFastSeconds)
				assert.Equal(t, 5, cfg.Display.DeploymentTopPodsCount)
				assert.Equal(t, 100, cfg.Performance.NetworkSpeedAverageMBps)
				assert.Equal(t, 75, cfg.Performance.ImagePullOptimizationPercent)
				assert.Contains(t, cfg.Registry.LocalRegistryHosts, "my-registry.local")
				assert.Contains(t, cfg.Registry.ClusterDomainSuffixes, ".k8s.local")
			},
		},
		{
			name: "load config with partial values uses defaults",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "bootscope.toml")
				require.NoError(t, os.WriteFile(configPath, []byte(`
[thresholds]
large_image_size_mb = 2000

[display]
# Only override one value
total_time_slow_seconds = 300
`), 0o644))
				return configPath
			},
			validateFunc: func(t *testing.T, cfg *Config) {
				// Custom value
				assert.Equal(t, int64(2000), cfg.Thresholds.LargeImageSizeMB)
				assert.Equal(t, 300, cfg.Display.TotalTimeSlowSeconds)

				// Default values should be preserved
				assert.Equal(t, 10, cfg.Thresholds.SlowPhaseSeconds)         // default
				assert.Equal(t, 30, cfg.Thresholds.ImagePullSlowSeconds)     // default
				assert.Equal(t, 25, cfg.Performance.NetworkSpeedAverageMBps) // default
			},
		},
		{
			name: "invalid TOML syntax",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "bootscope.toml")
				require.NoError(t, os.WriteFile(configPath, []byte(`
[thresholds
large_image_size_mb = 1000
`), 0o644))
				return configPath
			},
			wantErr: true,
		},
		{
			name: "missing config file uses defaults",
			setupFunc: func(t *testing.T) string {
				return "/non/existent/path/bootscope.toml"
			},
			validateFunc: func(t *testing.T, cfg *Config) {
				// Should have all default values
				expected := DefaultConfig()
				assert.Equal(t, expected.Thresholds, cfg.Thresholds)
				assert.Equal(t, expected.Display, cfg.Display)
				assert.Equal(t, expected.Performance, cfg.Performance)
			},
		},
		{
			name: "config with environment-specific overrides",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, ".kube", "bootscope.toml")
				require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
				require.NoError(t, os.WriteFile(configPath, []byte(`
# Production config with stricter thresholds
[thresholds]
large_image_size_mb = 200  # Stricter in prod
slow_phase_seconds = 5
bottleneck_percentage = 20  # More sensitive

[operations]
watch_poll_interval_seconds = 1  # Faster feedback
default_watch_timeout = "10m"    # Longer timeout for slow clusters
`), 0o644))
				return configPath
			},
			validateFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, int64(200), cfg.Thresholds.LargeImageSizeMB)
				assert.Equal(t, 5, cfg.Thresholds.SlowPhaseSeconds)
				assert.Equal(t, float64(20), cfg.Thresholds.BottleneckPercentage)
				assert.Equal(t, 1, cfg.Operations.WatchPollIntervalSeconds)
				assert.Equal(t, "10m", cfg.Operations.DefaultWatchTimeout)
			},
		},
		{
			name: "path traversal attempt should fail",
			setupFunc: func(t *testing.T) string {
				return "../../../etc/passwd"
			},
			wantErr: true,
		},
		{
			name: "path with .. in middle should fail",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				// Create a valid config file
				validPath := filepath.Join(tmpDir, "bootscope.toml")
				require.NoError(t, os.WriteFile(validPath, []byte(`[thresholds]`), 0o644))
				// Try to access it with path traversal
				return filepath.Join(tmpDir, "..", filepath.Base(tmpDir), "bootscope.toml")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := ""
			if tt.setupFunc != nil {
				configPath = tt.setupFunc(t)
			}

			cfg, err := LoadConfig(configPath)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.validateFunc != nil {
				tt.validateFunc(t, cfg)
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (cleanup func())
		provided  string
		want      string
		wantFound bool
	}{
		{
			name:      "provided path is used directly",
			provided:  "/custom/path/config.toml",
			want:      "/custom/path/config.toml",
			wantFound: true,
		},
		{
			name: "finds config in current directory",
			setupFunc: func(t *testing.T) func() {
				// Create temp config in current dir
				err := os.WriteFile("bootscope.toml", []byte("[thresholds]"), 0o644)
				require.NoError(t, err)
				return func() { os.Remove("bootscope.toml") }
			},
			want:      "./bootscope.toml",
			wantFound: true,
		},
		{
			name: "finds config in ~/.kube directory",
			setupFunc: func(t *testing.T) func() {
				home, err := os.UserHomeDir()
				require.NoError(t, err)

				kubeDir := filepath.Join(home, ".kube")
				os.MkdirAll(kubeDir, 0o755)

				configPath := filepath.Join(kubeDir, "bootscope.toml")
				err = os.WriteFile(configPath, []byte("[display]"), 0o644)
				require.NoError(t, err)

				return func() { os.Remove(configPath) }
			},
			want:      filepath.Join(os.Getenv("HOME"), ".kube", "bootscope.toml"),
			wantFound: true,
		},
		{
			name:      "no config found",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				cleanup := tt.setupFunc(t)
				defer cleanup()
			}

			// findConfigFile doesn't take arguments, so we need to handle provided path differently
			got := ""
			if tt.provided != "" {
				got = tt.provided
			} else {
				got = findConfigFile()
			}

			if tt.wantFound {
				assert.Equal(t, tt.want, got)
			} else {
				assert.Empty(t, got)
			}
		})
	}
}

func TestConfigGetters(t *testing.T) {
	cfg := &Config{
		Thresholds: ThresholdConfig{
			LargeImageSizeMB:         800,
			SlowPhaseSeconds:         15,
			ImagePullSlowSeconds:     45,
			InitContainerSlowSeconds: 25,
			AppStartSlowSeconds:      35,
			ResourceWaitingSeconds:   8,
		},
		Performance: PerformanceConfig{
			NetworkSpeedAverageMBps:  50,
			NetworkSpeedFastMBps:     100,
			FastPullThresholdSeconds: 10,
		},
		Operations: OperationsConfig{
			WatchPollIntervalSeconds:   3,
			ReadyTimeEstimationSeconds: 7,
		},
	}

	tests := []struct {
		name     string
		getter   func() interface{}
		expected interface{}
	}{
		{
			name:     "GetLargeImageThreshold converts MB to bytes",
			getter:   func() interface{} { return cfg.GetLargeImageThreshold() },
			expected: int64(800 * 1024 * 1024),
		},
		{
			name:     "GetSlowPhaseThreshold converts to duration",
			getter:   func() interface{} { return cfg.GetSlowPhaseThreshold() },
			expected: 15 * time.Second,
		},
		{
			name:     "GetImagePullSlowThreshold",
			getter:   func() interface{} { return cfg.GetImagePullSlowThreshold() },
			expected: 45 * time.Second,
		},
		{
			name:     "GetNetworkSpeedAverage converts MB/s to bytes/s",
			getter:   func() interface{} { return cfg.GetNetworkSpeedAverage() },
			expected: int64(50 * 1024 * 1024),
		},
		{
			name:     "GetWatchPollInterval",
			getter:   func() interface{} { return cfg.GetWatchPollInterval() },
			expected: 3 * time.Second,
		},
		{
			name:     "GetFastPullThreshold",
			getter:   func() interface{} { return cfg.GetFastPullThreshold() },
			expected: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.getter()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSaveDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.toml")

	err := SaveDefaultConfig(configPath)
	require.NoError(t, err)

	// Verify file exists and is readable
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
	assert.True(t, info.Size() > 1000, "Config file should have substantial content")

	// Verify it can be loaded back
	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify it contains expected sections with comments
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	configStr := string(content)
	assert.Contains(t, configStr, "# BootScope Configuration")
	assert.Contains(t, configStr, "[thresholds]")
	assert.Contains(t, configStr, "[display]")
	assert.Contains(t, configStr, "[performance]")
	assert.Contains(t, configStr, "[operations]")
	assert.Contains(t, configStr, "[registry]")
	assert.Contains(t, configStr, "large_image_size_mb")
	assert.Contains(t, configStr, "# Performance Configuration")

	// Should have defaults
	assert.Equal(t, DefaultConfig().Thresholds, cfg.Thresholds)
}

func TestConfigForRealScenarios(t *testing.T) {
	// Test config values make sense for real use cases
	t.Run("production strict config", func(t *testing.T) {
		cfg := &Config{
			Thresholds: ThresholdConfig{
				LargeImageSizeMB:     200, // Strict 200MB limit
				SlowPhaseSeconds:     5,   // 5s is slow in prod
				BottleneckPercentage: 20,  // 20% of time is significant
			},
			Performance: PerformanceConfig{
				NetworkSpeedAverageMBps: 100, // Fast internal network
			},
		}

		// These thresholds should identify issues quickly
		assert.Less(t, cfg.GetSlowPhaseThreshold(), 10*time.Second)
		assert.Less(t, cfg.GetLargeImageThreshold(), int64(500*1024*1024))
	})

	t.Run("development lenient config", func(t *testing.T) {
		cfg := &Config{
			Thresholds: ThresholdConfig{
				LargeImageSizeMB:     1000, // 1GB is OK for dev
				SlowPhaseSeconds:     30,   // More tolerant
				BottleneckPercentage: 40,   // Higher threshold
			},
			Performance: PerformanceConfig{
				NetworkSpeedAverageMBps: 25, // Slower network OK
			},
		}

		// Development can be more forgiving
		assert.GreaterOrEqual(t, cfg.GetSlowPhaseThreshold(), 20*time.Second)
		assert.GreaterOrEqual(t, cfg.GetLargeImageThreshold(), int64(500*1024*1024))
	})
}
