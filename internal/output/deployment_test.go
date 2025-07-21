package output

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/px4n/bootscope/internal/testutil"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/output"
	"github.com/px4n/bootscope/pkg/types"
)

func TestDeploymentAnalysis(t *testing.T) {
	// Create test profiles
	_ = config.DefaultConfig() // Use config to avoid import error
	profiles := []*types.PodStartupProfile{
		testutil.NewTestProfile(
			testutil.WithTotalTime(30*time.Second),
			testutil.WithPhases(
				types.PhaseInfo{
					Phase:    types.StartupPhaseScheduling,
					Duration: 5 * time.Second,
				},
				types.PhaseInfo{
					Phase:    types.StartupPhaseImagePull,
					Duration: 15 * time.Second,
				},
				types.PhaseInfo{
					Phase:    types.StartupPhaseAppStart,
					Duration: 10 * time.Second,
				},
			),
		),
		testutil.NewTestProfile(
			testutil.WithTotalTime(40*time.Second),
			testutil.WithPhases(
				types.PhaseInfo{
					Phase:    types.StartupPhaseScheduling,
					Duration: 3 * time.Second,
				},
				types.PhaseInfo{
					Phase:    types.StartupPhaseImagePull,
					Duration: 25 * time.Second,
				},
				types.PhaseInfo{
					Phase:    types.StartupPhaseAppStart,
					Duration: 12 * time.Second,
				},
			),
		),
		testutil.NewTestProfile(
			testutil.WithTotalTime(35*time.Second),
			testutil.WithPhases(
				types.PhaseInfo{
					Phase:    types.StartupPhaseScheduling,
					Duration: 4 * time.Second,
				},
				types.PhaseInfo{
					Phase:    types.StartupPhaseImagePull,
					Duration: 20 * time.Second,
				},
				types.PhaseInfo{
					Phase:    types.StartupPhaseAppStart,
					Duration: 11 * time.Second,
				},
			),
		),
	}

	cfg := testutil.NewTestConfig()

	output := testutil.CaptureStdout(func() {
		err := DeploymentAnalysis("test-deployment", profiles, cfg)
		require.NoError(t, err)
	})

	// Verify output contains expected sections
	assert.Contains(t, output, "Deployment Startup Analysis: /test-deployment")
	assert.Contains(t, output, "Pods analyzed: 3")
	assert.Contains(t, output, "Overall Startup Times:")
	assert.Contains(t, output, "Avg:")
	assert.Contains(t, output, "Min:")
	assert.Contains(t, output, "Max:")
	assert.Contains(t, output, "Phase Statistics:")
	assert.Contains(t, output, "Scheduling")
	assert.Contains(t, output, "ImagePull")
	assert.Contains(t, output, "ApplicationStart")
	assert.Contains(t, output, "Pod Details:")
}

func TestDeploymentAnalysisWithNamespace(t *testing.T) {
	profiles := []*types.PodStartupProfile{
		testutil.NewTestProfile(
			testutil.WithTotalTime(30 * time.Second),
		),
	}

	cfg := testutil.NewTestConfig()

	output := testutil.CaptureStdout(func() {
		err := DeploymentAnalysisWithNamespace("custom-ns", "test-deployment", profiles, cfg)
		require.NoError(t, err)
	})

	// Verify namespace is included
	assert.Contains(t, output, "Deployment Startup Analysis: custom-ns/test-deployment")
}

func TestDeploymentAnalysisEmpty(t *testing.T) {
	profiles := []*types.PodStartupProfile{}
	cfg := testutil.NewTestConfig()

	output := testutil.CaptureStdout(func() {
		err := DeploymentAnalysis("test-deployment", profiles, cfg)
		require.NoError(t, err)
	})

	// Should handle empty profiles gracefully
	assert.Contains(t, output, "Deployment Startup Analysis: /test-deployment")
	assert.Contains(t, output, "Pods analyzed: 0")
}

func TestCollectDeploymentStats(t *testing.T) {
	profiles := []*types.PodStartupProfile{
		testutil.NewTestProfile(
			testutil.WithTotalTime(30*time.Second),
			testutil.WithPhases(
				types.PhaseInfo{
					Phase:    types.StartupPhaseScheduling,
					Duration: 5 * time.Second,
				},
				types.PhaseInfo{
					Phase:    types.StartupPhaseImagePull,
					Duration: 15 * time.Second,
				},
			),
		),
		testutil.NewTestProfile(
			testutil.WithTotalTime(40*time.Second),
			testutil.WithPhases(
				types.PhaseInfo{
					Phase:    types.StartupPhaseScheduling,
					Duration: 3 * time.Second,
				},
				types.PhaseInfo{
					Phase:    types.StartupPhaseImagePull,
					Duration: 25 * time.Second,
				},
			),
		),
	}

	totalTimes, phaseStats := collectDeploymentStats(profiles)

	// Verify total times
	assert.Len(t, totalTimes, 2)
	assert.Contains(t, totalTimes, 30*time.Second)
	assert.Contains(t, totalTimes, 40*time.Second)

	// Verify phase stats
	assert.Len(t, phaseStats, 2)
	assert.Contains(t, phaseStats, types.StartupPhaseScheduling)
	assert.Contains(t, phaseStats, types.StartupPhaseImagePull)

	// Check scheduling stats
	schedStats := phaseStats[types.StartupPhaseScheduling]
	assert.Len(t, schedStats, 2)
	assert.Contains(t, schedStats, 5*time.Second)
	assert.Contains(t, schedStats, 3*time.Second)

	// Check image pull stats
	pullStats := phaseStats[types.StartupPhaseImagePull]
	assert.Len(t, pullStats, 2)
	assert.Contains(t, pullStats, 15*time.Second)
	assert.Contains(t, pullStats, 25*time.Second)
}

func TestOutputPhaseStats(t *testing.T) {
	phaseStats := map[types.StartupPhase][]time.Duration{
		types.StartupPhaseScheduling: {2 * time.Second, 3 * time.Second, 4 * time.Second},
		types.StartupPhaseImagePull:  {20 * time.Second, 25 * time.Second, 30 * time.Second},
	}

	output := testutil.CaptureStdout(func() {
		outputPhaseStats(phaseStats)
	})

	// Verify table structure
	assert.Contains(t, output, "Phase Statistics:")
	assert.Contains(t, output, "Avg:")
	assert.Contains(t, output, "Min:")
	assert.Contains(t, output, "Max:")

	// Verify phase data
	assert.Contains(t, output, "Scheduling")
	assert.Contains(t, output, "3.0s") // average
	assert.Contains(t, output, "2.0s") // min
	assert.Contains(t, output, "4.0s") // max

	assert.Contains(t, output, "ImagePull")
	assert.Contains(t, output, "25s") // average
	assert.Contains(t, output, "20s") // min
	assert.Contains(t, output, "30s") // max
}

func TestOutputDeploymentObservations(t *testing.T) {
	// Create stats with high variance
	times := []time.Duration{1 * time.Second, 2 * time.Second, 30 * time.Second}
	stats := output.CalculateStats(times)

	phaseStats := map[types.StartupPhase][]time.Duration{
		types.StartupPhaseScheduling: times,
	}

	cfg := testutil.NewTestConfig()

	output := testutil.CaptureStdout(func() {
		outputDeploymentObservations(stats, phaseStats, cfg)
	})

	// Should detect high variance
	assert.Contains(t, output, "Observations:")
	assert.Contains(t, output, "High variance detected")
}
