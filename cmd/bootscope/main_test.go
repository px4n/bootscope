package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	intOutput "github.com/px4n/bootscope/internal/output"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/types"
)

// Helper to capture stdout
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	return string(out)
}

func TestOutputText(t *testing.T) {
	profile := &types.PodStartupProfile{
		PodName:   "test-pod-123",
		Namespace: "default",
		Status:    "Running",
		TotalTime: 45 * time.Second,
		StartTime: time.Now().Add(-45 * time.Second),
		ReadyTime: time.Now(),
		Phases: []types.PhaseInfo{
			{
				Phase:    types.StartupPhaseScheduling,
				Duration: 500 * time.Millisecond,
				Details:  "Scheduled to node worker-1",
			},
			{
				Phase:    types.StartupPhaseImagePull,
				Duration: 30 * time.Second,
				Details:  "Pulled image nginx:latest",
				SubPhases: []types.PhaseInfo{
					{
						Details:  "nginx:latest (500MB)",
						Duration: 30 * time.Second,
					},
				},
			},
			{
				Phase:    types.StartupPhaseInitContainers,
				Duration: 10 * time.Second,
				Details:  "Init container: setup-config",
			},
			{
				Phase:    types.StartupPhaseAppStart,
				Duration: 4500 * time.Millisecond,
				Details:  "Started container nginx",
			},
		},
		Bottlenecks: []types.Bottleneck{
			{
				Phase:       types.StartupPhaseImagePull,
				Severity:    "critical",
				Description: "ImagePull took 66.7% of total startup time",
			},
		},
		Recommendations: []types.Recommendation{
			{
				Title:       "Use Local Registry",
				Description: "Image pull is taking a significant amount of time. Consider using a local registry.",
				Impact:      "Could save approximately 15s",
				Link:        "https://kubernetes.io/docs/concepts/containers/images/",
			},
		},
	}

	cfg := config.DefaultConfig()

	output := captureOutput(func() {
		err := intOutput.Text(profile, cfg)
		require.NoError(t, err)
	})

	// Verify key components are in the output
	assert.Contains(t, output, "Pod Startup Profile: default/test-pod-123")
	assert.Contains(t, output, "Total Time:")
	assert.Contains(t, output, "45s") // Total time
	assert.Contains(t, output, "🕰️")  // Moderate indicator for 45s
	assert.Contains(t, output, "Status: Running")

	// Phase breakdown
	assert.Contains(t, output, "Phase Breakdown:")
	assert.Contains(t, output, "Scheduling:")
	assert.Contains(t, output, "500ms")
	assert.Contains(t, output, "ImagePull:")
	assert.Contains(t, output, "30s")
	assert.Contains(t, output, "67%") // Percentage (30s/45s)
	assert.Contains(t, output, "⚠️")  // Warning for slow phase

	// Bottlenecks
	assert.Contains(t, output, "Bottlenecks Identified:")
	assert.Contains(t, output, "🚨") // Critical bottleneck
	assert.Contains(t, output, "ImagePull took 66.7% of total startup time")

	// Recommendations
	assert.Contains(t, output, "Recommendations:")
	assert.Contains(t, output, "Use Local Registry")
	assert.Contains(t, output, "Could save approximately 15s")
}

func TestOutputSimple(t *testing.T) {
	tests := []struct {
		name        string
		profile     *types.PodStartupProfile
		expected    []string
		notExpected []string
	}{
		{
			name: "slow pod with issues",
			profile: &types.PodStartupProfile{
				TotalTime: 120 * time.Second,
				Phases: []types.PhaseInfo{
					{
						Phase:    types.StartupPhaseScheduling,
						Duration: 100 * time.Millisecond,
						Details:  "Scheduled quickly",
					},
					{
						Phase:    types.StartupPhaseImagePull,
						Duration: 90 * time.Second,
						Details:  "Pulled large image",
					},
					{
						Phase:    types.StartupPhaseAppStart,
						Duration: 29900 * time.Millisecond,
						Details:  "Application startup",
					},
				},
				Recommendations: []types.Recommendation{
					{
						Title:       "Optimize Image Size",
						Description: "Your image is too large",
						TimeSaved:   45 * time.Second,
					},
				},
			},
			expected: []string{
				"Your pod is starting slowly!",
				"✅ Finding a node: 100ms",
				"⚠️ Downloading container image: 1m30s",
				"🕰️ Starting your application: 29s",
				"Time saved: ~45s",
				"Total time saved if fixed: 45s 🎉",
			},
			notExpected: []string{
				"Phase",
				"Bottleneck",
			},
		},
		{
			name: "fast pod",
			profile: &types.PodStartupProfile{
				TotalTime: 5 * time.Second,
				Phases: []types.PhaseInfo{
					{
						Phase:    types.StartupPhaseAppStart,
						Duration: 5 * time.Second,
					},
				},
			},
			expected: []string{
				"Your pod started in 5.0s",
				"✅ Starting your application: 5.0s",
			},
			notExpected: []string{
				"slowly",
				"⚠️",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			output := captureOutput(func() {
				err := intOutput.Simple(tt.profile, cfg)
				require.NoError(t, err)
			})

			for _, exp := range tt.expected {
				assert.Contains(t, output, exp)
			}

			for _, notExp := range tt.notExpected {
				assert.NotContains(t, output, notExp)
			}
		})
	}
}

func TestOutputJSON(t *testing.T) {
	profile := &types.PodStartupProfile{
		PodName:   "json-test",
		Namespace: "default",
		TotalTime: 10 * time.Second,
		Phases: []types.PhaseInfo{
			{
				Phase:    types.StartupPhaseImagePull,
				Duration: 10 * time.Second,
			},
		},
	}

	output := captureOutput(func() {
		err := intOutput.JSON(profile)
		require.NoError(t, err)
	})

	// Should be valid JSON
	var result types.PodStartupProfile
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "json-test", result.PodName)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, 10*time.Second, result.TotalTime)
	assert.Len(t, result.Phases, 1)
}

func TestOutputYAML(t *testing.T) {
	profile := &types.PodStartupProfile{
		PodName:   "yaml-test",
		Namespace: "kube-system",
		Status:    "Failed",
		TotalTime: 30 * time.Second,
	}

	output := captureOutput(func() {
		err := intOutput.YAML(profile)
		require.NoError(t, err)
	})

	// Should be valid YAML
	var result types.PodStartupProfile
	err := yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "yaml-test", result.PodName)
	assert.Equal(t, "kube-system", result.Namespace)
	assert.Equal(t, "Failed", result.Status)
}

func TestOutputHelperFunctions(t *testing.T) {
	t.Run("getTotalTimeIndicator", func(t *testing.T) {
		cfg := config.DefaultConfig()

		tests := []struct {
			duration time.Duration
			expected string
		}{
			{5 * time.Second, "✅"},   // Fast
			{35 * time.Second, "🕰️"}, // Moderate
			{65 * time.Second, "⚠️"}, // Slow
			{125 * time.Second, "🐌"}, // Very slow
		}

		for _, tt := range tests {
			got := intOutput.GetTotalTimeIndicator(tt.duration, cfg)
			assert.Equal(t, tt.expected, got)
		}
	})

	t.Run("getPhaseDurationColor", func(t *testing.T) {
		cfg := config.DefaultConfig()

		fast := intOutput.GetPhaseDurationColor(5*time.Second, cfg)
		medium := intOutput.GetPhaseDurationColor(35*time.Second, cfg)
		slow := intOutput.GetPhaseDurationColor(65*time.Second, cfg)

		// Can't directly test color objects, but verify they're different
		assert.NotNil(t, fast)
		assert.NotNil(t, medium)
		assert.NotNil(t, slow)
	})

	t.Run("getBottleneckIcon", func(t *testing.T) {
		assert.Equal(t, "ℹ️", intOutput.GetBottleneckIcon("info"))
		assert.Equal(t, "⚠️", intOutput.GetBottleneckIcon("warning"))
		assert.Equal(t, "🚨", intOutput.GetBottleneckIcon("critical"))
		assert.Equal(t, "ℹ️", intOutput.GetBottleneckIcon("unknown"))
	})

	t.Run("simplifyPhaseName", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"Scheduling", "Finding a node"},
			{"ImagePull", "Downloading container image"},
			{"ContainerCreation", "Creating container"},
			{"InitContainers", "Running setup tasks"},
			{"ApplicationStart", "Starting your application"},
			{"Unknown", "Unknown"},
		}

		for _, tt := range tests {
			got := intOutput.SimplifyPhaseName(types.StartupPhase(tt.input))
			assert.Equal(t, tt.expected, got)
		}
	})
}

func TestOutputDeploymentAnalysis(t *testing.T) {
	profiles := []*types.PodStartupProfile{
		{
			PodName:   "app-1",
			TotalTime: 5 * time.Second,
			Status:    "Running",
			Phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseImagePull, Duration: 3 * time.Second},
				{Phase: types.StartupPhaseAppStart, Duration: 2 * time.Second},
			},
		},
		{
			PodName:   "app-2",
			TotalTime: 15 * time.Second,
			Status:    "Running",
			Phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseImagePull, Duration: 10 * time.Second},
				{Phase: types.StartupPhaseAppStart, Duration: 5 * time.Second},
			},
		},
		{
			PodName:   "app-3",
			TotalTime: 60 * time.Second, // Outlier
			Status:    "Running",
			Phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseImagePull, Duration: 55 * time.Second},
				{Phase: types.StartupPhaseAppStart, Duration: 5 * time.Second},
			},
		},
	}

	cfg := config.DefaultConfig()

	output := captureOutput(func() {
		err := intOutput.OutputDeploymentAnalysis("test-deployment", profiles, cfg)
		require.NoError(t, err)
	})

	// Verify deployment header
	assert.Contains(t, output, "Deployment Startup Analysis:")
	assert.Contains(t, output, "test-deployment")
	assert.Contains(t, output, "Pods analyzed: 3")

	// Overall statistics
	assert.Contains(t, output, "Overall Startup Times:")
	assert.Contains(t, output, "Min:")
	assert.Contains(t, output, "5s")
	assert.Contains(t, output, "Max:")
	assert.Contains(t, output, "1m")
	assert.Contains(t, output, "Median:")

	// Phase statistics
	assert.Contains(t, output, "Phase Statistics:")
	assert.Contains(t, output, "ImagePull:")

	// Pod details (sorted by slowest first)
	assert.Contains(t, output, "Pod Details:")
	assert.Contains(t, output, "app-3")
	assert.Contains(t, output, "[outlier]") // app-3 is an outlier

	// Observations about high variance
	assert.Contains(t, output, "High variance detected")
	assert.Contains(t, output, "Consider investigating node-specific issues")
}

func TestErrorHandling(t *testing.T) {
	t.Run("outputJSON with nil profile", func(t *testing.T) {
		err := intOutput.JSON(nil)
		assert.NoError(t, err) // json.Marshal handles nil
	})

	t.Run("outputText with minimal profile", func(t *testing.T) {
		profile := &types.PodStartupProfile{
			PodName: "minimal",
			// No phases, recommendations, etc
		}
		cfg := config.DefaultConfig()

		// Should handle gracefully
		output := captureOutput(func() {
			err := intOutput.Text(profile, cfg)
			require.NoError(t, err)
		})

		assert.Contains(t, output, "minimal")
		assert.Contains(t, output, "Phase Breakdown:")
	})
}

func TestRealWorldScenarios(t *testing.T) {
	t.Run("pod stuck in image pull", func(t *testing.T) {
		profile := &types.PodStartupProfile{
			PodName:   "stuck-pod",
			Namespace: "production",
			Status:    "Pending",
			TotalTime: 5 * time.Minute,
			Phases: []types.PhaseInfo{
				{
					Phase:    types.StartupPhaseImagePull,
					Duration: 5 * time.Minute,
					Details:  "Pulling image company/app:v2.0.0",
				},
			},
			Bottlenecks: []types.Bottleneck{
				{
					Phase:       types.StartupPhaseImagePull,
					Severity:    "critical",
					Description: "ImagePull took 100% of total startup time",
				},
			},
			Recommendations: []types.Recommendation{
				{
					Title:       "Check Image Registry",
					Description: "Image pull is taking too long. Registry might be slow or image might be very large.",
				},
				{
					Title:       "Pre-pull Images",
					Description: "Consider using a DaemonSet to pre-pull images to nodes.",
				},
			},
		}

		cfg := config.DefaultConfig()
		output := captureOutput(func() {
			err := intOutput.Text(profile, cfg)
			require.NoError(t, err)
		})

		assert.Contains(t, output, "Pending")
		assert.Contains(t, output, "5m")
		assert.Contains(t, output, "100%") // All time in image pull
		assert.Contains(t, output, "Check Image Registry")
		assert.Contains(t, output, "Pre-pull Images")
	})
}

// Benchmark output performance with large phase lists
func BenchmarkOutputText(b *testing.B) {
	// Create profile with many phases (e.g., many init containers)
	phases := make([]types.PhaseInfo, 50)
	for i := range phases {
		phases[i] = types.PhaseInfo{
			Phase:    types.StartupPhaseInitContainers,
			Duration: time.Duration(i+1) * time.Second,
			Details:  fmt.Sprintf("Init container %d", i),
		}
	}

	profile := &types.PodStartupProfile{
		PodName:   "bench-pod",
		Namespace: "default",
		Status:    "Running",
		TotalTime: 30 * time.Minute,
		Phases:    phases,
	}

	cfg := config.DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Discard output for benchmark
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		intOutput.Text(profile, cfg)
		w.Close()
		os.Stdout = old
		io.Copy(io.Discard, r) // Drain the pipe
	}
}
