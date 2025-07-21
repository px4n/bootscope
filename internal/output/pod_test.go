package output

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/px4n/bootscope/internal/testutil"
	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/types"
)

func TestText(t *testing.T) {
	tests := []struct {
		name     string
		profile  *types.PodStartupProfile
		cfg      *config.Config
		contains []string
		excludes []string
	}{
		{
			name: "normal pod with all phases",
			profile: testutil.NewTestProfile(
				testutil.WithPhases(
					testutil.SchedulingPhase,
					testutil.ImagePullPhase,
					testutil.ContainerCreatePhase,
					testutil.AppStartPhase,
				),
				testutil.WithTotalTime(60*time.Second),
			),
			cfg: testutil.NewTestConfig(),
			contains: []string{
				"Pod Startup Profile:",
				"default/test-pod",
				"Total Time:",
				"1m",
				"Status: Running",
				"Phase Breakdown:",
				"Scheduling:",
				"ImagePull:",
				"ContainerCreation:",
				"ApplicationStart:",
			},
			excludes: []string{
				"Container Restarts:",
				"Bottlenecks Identified:",
				"Recommendations:",
			},
		},
		{
			name: "pod with container restarts",
			profile: testutil.NewTestProfile(
				testutil.WithContainerStatuses(
					types.ContainerStatusInfo{
						Name:         "app",
						RestartCount: 3,
						Ready:        true,
						State:        "Running",
					},
				),
			),
			cfg: testutil.NewTestConfig(),
			contains: []string{
				"Container Restarts: 3",
			},
		},
		{
			name: "pod with bottlenecks and recommendations",
			profile: testutil.NewTestProfile(
				testutil.WithBottlenecks(
					types.Bottleneck{
						Phase:       types.StartupPhaseImagePull,
						Duration:    30 * time.Second,
						Percentage:  75,
						Description: "Image pull took 75% of total startup time",
						Severity:    "warning",
					},
				),
				testutil.WithRecommendations(
					types.Recommendation{
						Title:       "Use Local Registry",
						Description: "Consider using a local registry to speed up image pulls",
						Impact:      "Could save 20s",
						Priority:    1,
					},
				),
			),
			cfg: testutil.NewTestConfig(),
			contains: []string{
				"Bottlenecks Identified:",
				"Image pull took 75% of total startup time",
				"Recommendations:",
				"Use Local Registry",
				"Consider using a local registry",
				"Impact: Could save 20s",
			},
		},
		{
			name: "failed pod status",
			profile: testutil.NewTestProfile(
				testutil.WithStatus("Failed"),
			),
			cfg: testutil.NewTestConfig(),
			contains: []string{
				"Status: Failed",
			},
		},
		{
			name: "CrashLoopBackOff status",
			profile: testutil.NewTestProfile(
				testutil.WithStatus("CrashLoopBackOff"),
			),
			cfg: testutil.NewTestConfig(),
			contains: []string{
				"Status: CrashLoopBackOff ⚠️",
			},
		},
		{
			name: "running but not ready",
			profile: testutil.NewTestProfile(
				testutil.WithStatus("Running"),
				testutil.WithContainerStatuses(
					types.ContainerStatusInfo{
						Name:  "app",
						Ready: false,
						State: "Running",
					},
				),
			),
			cfg: testutil.NewTestConfig(),
			contains: []string{
				"Status: Running (Not Ready)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := testutil.CaptureStdout(func() {
				err := Text(tt.profile, tt.cfg)
				require.NoError(t, err)
			})

			for _, expected := range tt.contains {
				assert.Contains(t, output, expected, "Output should contain: %s", expected)
			}

			for _, excluded := range tt.excludes {
				assert.NotContains(t, output, excluded, "Output should not contain: %s", excluded)
			}
		})
	}
}

func TestText_NilHandling(t *testing.T) {
	// Test with minimal profile
	profile := &types.PodStartupProfile{
		PodName:   "minimal-pod",
		Namespace: "default",
		Status:    "Unknown",
		TotalTime: 0,
	}

	cfg := testutil.NewTestConfig()

	// Should not panic
	output := testutil.CaptureStdout(func() {
		err := Text(profile, cfg)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "minimal-pod")
	assert.Contains(t, output, "0ms")
}

func TestSimple(t *testing.T) {
	tests := []struct {
		name     string
		profile  *types.PodStartupProfile
		cfg      *config.Config
		contains []string
		excludes []string
	}{
		{
			name: "fast pod",
			profile: testutil.NewTestProfile(
				testutil.WithTotalTime(10*time.Second),
				testutil.WithPhases(
					types.PhaseInfo{
						Phase:    types.StartupPhaseScheduling,
						Duration: 1 * time.Second,
					},
					types.PhaseInfo{
						Phase:    types.StartupPhaseAppStart,
						Duration: 9 * time.Second,
					},
				),
			),
			cfg: testutil.NewTestConfig(),
			contains: []string{
				"Your pod started in 10s",
				"Finding a node: 1.0s",
				"Starting your application: 9.0s",
				"Total time saved if fixed: 0ms",
			},
			excludes: []string{
				"Your pod is starting slowly",
			},
		},
		{
			name: "slow pod",
			profile: testutil.NewTestProfile(
				testutil.WithTotalTime(2*time.Minute),
				testutil.WithPhases(
					types.PhaseInfo{
						Phase:    types.StartupPhaseImagePull,
						Duration: 90 * time.Second,
					},
					types.PhaseInfo{
						Phase:    types.StartupPhaseAppStart,
						Duration: 30 * time.Second,
					},
				),
			),
			cfg: testutil.NewTestConfig(),
			contains: []string{
				"Your pod is starting slowly! Here's why:",
				"⚠️ Downloading container image: 1m30s",
				"⚠️ Starting your application: 30s",
			},
		},
		{
			name: "pod with recommendations",
			profile: testutil.NewTestProfile(
				testutil.WithTotalTime(60*time.Second),
				testutil.WithRecommendations(
					types.Recommendation{
						Title:       "Use pre-pulled images",
						Description: "Consider using a DaemonSet to pre-pull images",
						TimeSaved:   30 * time.Second,
					},
				),
			),
			cfg: testutil.NewTestConfig(),
			contains: []string{
				"How to potentially make it faster:",
				"Use pre-pulled images",
				"Time saved: ~30s",
				"Total time saved if fixed: 30s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := testutil.CaptureStdout(func() {
				err := Simple(tt.profile, tt.cfg)
				require.NoError(t, err)
			})

			for _, expected := range tt.contains {
				assert.Contains(t, output, expected, "Output should contain: %s", expected)
			}

			for _, excluded := range tt.excludes {
				assert.NotContains(t, output, excluded, "Output should not contain: %s", excluded)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	profile := testutil.NewTestProfile(
		testutil.WithTotalTime(45*time.Second),
		testutil.WithPhases(
			types.PhaseInfo{
				Phase:     types.StartupPhaseScheduling,
				StartTime: time.Now().Add(-60 * time.Second),
				EndTime:   time.Now().Add(-58 * time.Second),
				Duration:  2 * time.Second,
				Details:   "Scheduled to node",
			},
		),
		testutil.WithBottlenecks(
			types.Bottleneck{
				Phase:       types.StartupPhaseImagePull,
				Duration:    30 * time.Second,
				Percentage:  66.67,
				Description: "Image pull dominated startup time",
			},
		),
	)

	output := testutil.CaptureStdout(func() {
		err := JSON(profile)
		require.NoError(t, err)
	})

	// Parse the JSON output
	var result types.PodStartupProfile
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, "test-pod", result.PodName)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, 45*time.Second, result.TotalTime)
	assert.Equal(t, "Running", result.Status)
	assert.Len(t, result.Phases, 1)
	assert.Equal(t, types.StartupPhaseScheduling, result.Phases[0].Phase)
	assert.Len(t, result.Bottlenecks, 1)
	assert.Equal(t, types.StartupPhaseImagePull, result.Bottlenecks[0].Phase)

	// Verify JSON is properly formatted (indented)
	assert.Contains(t, output, "  \"podName\":")
	assert.Contains(t, output, "  \"namespace\":")
}

func TestYAML(t *testing.T) {
	profile := testutil.NewTestProfile(
		testutil.WithTotalTime(30*time.Second),
		testutil.WithPhases(
			types.PhaseInfo{
				Phase:     types.StartupPhaseAppStart,
				StartTime: time.Now().Add(-30 * time.Second),
				EndTime:   time.Now(),
				Duration:  30 * time.Second,
				Details:   "Application initialization",
			},
		),
		testutil.WithRecommendations(
			types.Recommendation{
				Title:       "Optimize startup",
				Description: "Use lazy loading",
				Priority:    1,
			},
		),
	)

	output := testutil.CaptureStdout(func() {
		err := YAML(profile)
		require.NoError(t, err)
	})

	// Parse the YAML output
	var result types.PodStartupProfile
	err := yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, "test-pod", result.PodName)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, 30*time.Second, result.TotalTime)
	assert.Len(t, result.Phases, 1)
	assert.Equal(t, types.StartupPhaseAppStart, result.Phases[0].Phase)
	assert.Len(t, result.Recommendations, 1)
	assert.Equal(t, "Optimize startup", result.Recommendations[0].Title)

	// Verify YAML format
	assert.Contains(t, output, "podName: test-pod")
	assert.Contains(t, output, "namespace: default")
}

func TestDebug(t *testing.T) {
	testTime := time.Now()
	profile := testutil.NewTestProfile(
		testutil.WithTotalTime(60*time.Second),
		testutil.WithPhases(
			types.PhaseInfo{
				Phase:     types.StartupPhaseScheduling,
				StartTime: testTime.Add(-60 * time.Second),
				EndTime:   testTime.Add(-58 * time.Second),
				Duration:  2 * time.Second,
				Details:   "Node selection details",
			},
		),
		testutil.WithContainerStatuses(
			types.ContainerStatusInfo{
				Name:         "main",
				RestartCount: 2,
				Ready:        true,
				State:        "Running",
			},
		),
	)

	// Add some metadata
	profile.StartTime = testTime.Add(-60 * time.Second)
	profile.ReadyTime = testTime
	profile.ResourceWaiting = &types.ResourceWaitInfo{
		Type:     "CPU",
		Reason:   "Insufficient CPU",
		Duration: 5 * time.Second,
		Message:  "Waiting for node with available CPU",
	}
	profile.ImageMetadata = []types.ImageInfo{
		{
			Name:     "nginx:latest",
			Size:     150 * 1024 * 1024,
			PullTime: 10 * time.Second,
		},
	}

	// Create a test PodInfo and verify podInfo is from collector package
	podInfo := testutil.NewTestPodInfo()
	var _ *collector.PodInfo = podInfo

	output := testutil.CaptureStdout(func() {
		Debug(podInfo, profile)
	})

	// Verify debug output contains expected sections
	assert.Contains(t, output, "DEBUG INFORMATION")
	assert.Contains(t, output, "Pod: default/test-pod")
	assert.Contains(t, output, "Created:")
	assert.Contains(t, output, "Node: worker-1")
	assert.Contains(t, output, "Events (sorted by time):")
	assert.Contains(t, output, "Scheduled")
	assert.Contains(t, output, "Conditions:")
	assert.Contains(t, output, "Container Statuses:")
	assert.Contains(t, output, "Container: main")
	assert.Contains(t, output, "RestartCount: 0")
	assert.Contains(t, output, "Detected Phases:")
	assert.Contains(t, output, "Scheduling")
	assert.Contains(t, output, "Duration: 2.000s")
	assert.Contains(t, output, "Timing Calculations:")
	assert.Contains(t, output, "Total time: 1m0s")
}
