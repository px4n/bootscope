package output

import (
	"time"

	"github.com/fatih/color"

	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/output"
	"github.com/px4n/bootscope/pkg/types"
)

func GetTotalTimeIndicator(totalTime time.Duration, cfg *config.Config) string {
	return getTotalTimeIndicator(totalTime, cfg)
}

func GetPhaseDurationColor(duration time.Duration, cfg *config.Config) *color.Color {
	return getPhaseDurationColor(duration, cfg)
}

func GetBottleneckIcon(severity string) string {
	return getBottleneckIcon(severity)
}

func SimplifyPhaseName(phase types.StartupPhase) string {
	return simplifyPhaseName(phase)
}

func OutputDeploymentAnalysis(deploymentName string, profiles []*types.PodStartupProfile, cfg *config.Config) error {
	totalTimes, phaseStats := collectDeploymentStats(profiles)
	overallStats := output.CalculateStats(totalTimes)

	// Custom header without namespace
	outputDeploymentHeader(deploymentName, len(profiles))
	outputOverallStats(overallStats)
	outputPhaseStats(phaseStats)
	outputPodDetails(profiles, overallStats, cfg)
	outputDeploymentObservations(overallStats, phaseStats, cfg)

	return nil
}
