package output

import (
	"fmt"
	"sort"
	"time"

	"github.com/fatih/color"

	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/output"
	"github.com/px4n/bootscope/pkg/types"
)

func DeploymentAnalysis(deploymentName string, profiles []*types.PodStartupProfile, cfg *config.Config) error {
	totalTimes, phaseStats := collectDeploymentStats(profiles)
	overallStats := output.CalculateStats(totalTimes)

	outputDeploymentHeader(deploymentName, len(profiles))
	outputOverallStats(overallStats)
	outputPhaseStats(phaseStats)
	outputPodDetails(profiles, overallStats, cfg)
	outputDeploymentObservations(overallStats, phaseStats, cfg)

	fmt.Println()
	return nil
}

func collectDeploymentStats(profiles []*types.PodStartupProfile) ([]time.Duration, map[types.StartupPhase][]time.Duration) {
	var totalTimes []time.Duration
	phaseStats := make(map[types.StartupPhase][]time.Duration)

	for _, profile := range profiles {
		totalTimes = append(totalTimes, profile.TotalTime)
		for _, phase := range profile.Phases {
			phaseStats[phase.Phase] = append(phaseStats[phase.Phase], phase.Duration)
		}
	}

	return totalTimes, phaseStats
}

func outputDeploymentHeader(deploymentName string, podCount int) {
	fmt.Printf("\n%s %s/%s\n",
		color.New(color.Bold).Sprint("Deployment Startup Analysis:"),
		"", // namespace will be added by caller
		deploymentName)
	fmt.Printf("%s %d\n\n",
		color.New(color.Bold).Sprint("Pods analyzed:"),
		podCount)
}

func outputOverallStats(stats output.Stats) {
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("Overall Startup Times:"))
	fmt.Printf("  Min:    %s\n", output.FormatDuration(stats.Min))
	fmt.Printf("  Avg:    %s\n", output.FormatDuration(stats.Avg))
	fmt.Printf("  Median: %s\n", output.FormatDuration(stats.Median))
	fmt.Printf("  P95:    %s\n", output.FormatDuration(stats.P95))
	fmt.Printf("  P99:    %s\n", output.FormatDuration(stats.P99))
	fmt.Printf("  Max:    %s\n", output.FormatDuration(stats.Max))
	fmt.Println()
}

func outputPhaseStats(phaseStats map[types.StartupPhase][]time.Duration) {
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("Phase Statistics:"))
	phases := []types.StartupPhase{
		types.StartupPhaseScheduling,
		types.StartupPhaseImagePull,
		types.StartupPhaseContainerCreate,
		types.StartupPhaseInitContainers,
		types.StartupPhaseAppStart,
	}

	for _, phase := range phases {
		if durations, ok := phaseStats[phase]; ok && len(durations) > 0 {
			stats := output.CalculateStats(durations)
			fmt.Printf("  %-20s Avg: %-8s Min: %-8s Max: %-8s\n",
				string(phase)+":",
				output.FormatDuration(stats.Avg),
				output.FormatDuration(stats.Min),
				output.FormatDuration(stats.Max))
		}
	}
	fmt.Println()
}

func outputPodDetails(profiles []*types.PodStartupProfile, overallStats output.Stats, cfg *config.Config) {
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("Pod Details:"))

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].TotalTime > profiles[j].TotalTime
	})

	for i, profile := range profiles {
		outputPodSummary(profile, overallStats)
		if shouldShowPodDetail(i, len(profiles), cfg) {
			outputPodPhases(profile, cfg)
		}
	}
}

func outputPodSummary(profile *types.PodStartupProfile, overallStats output.Stats) {
	icon := getPodIcon(profile.TotalTime, overallStats)
	fmt.Printf("  %s %-40s %s", icon, profile.PodName, output.FormatDuration(profile.TotalTime))

	if profile.Status != "Running" {
		fmt.Printf(" (%s)", color.New(color.FgRed).Sprint(profile.Status))
	}

	if profile.TotalTime > overallStats.P95 {
		fmt.Printf(" %s", color.New(color.FgYellow).Sprint("[outlier]"))
	}

	fmt.Println()
}

func getPodIcon(totalTime time.Duration, stats output.Stats) string {
	if totalTime > stats.P95 {
		return "⚠️"
	} else if totalTime > stats.Median {
		return "⏱"
	}
	return "✅"
}

func shouldShowPodDetail(index, total int, cfg *config.Config) bool {
	return index < cfg.Display.DeploymentTopPodsCount && total > cfg.Display.DeploymentTopPodsCount
}

func outputPodPhases(profile *types.PodStartupProfile, cfg *config.Config) {
	for _, phase := range profile.Phases {
		if phase.Duration >= time.Duration(cfg.Display.DeploymentMinPhaseSeconds)*time.Second {
			fmt.Printf("     └─ %s: %s\n", phase.Phase, output.FormatDuration(phase.Duration))
		}
	}
}

func outputDeploymentObservations(overallStats output.Stats, phaseStats map[types.StartupPhase][]time.Duration, cfg *config.Config) {
	if hasHighVariance(overallStats) {
		fmt.Println("\n" + color.New(color.Bold, color.Underline).Sprint("Observations:"))
		fmt.Printf("⚠️  High variance detected: fastest pod (%s) vs slowest (%s)\n",
			output.FormatDuration(overallStats.Min),
			output.FormatDuration(overallStats.Max))
		fmt.Println("   Consider investigating node-specific issues or resource constraints")
	}

	for phase, durations := range phaseStats {
		stats := output.CalculateStats(durations)
		if stats.Avg >= time.Duration(cfg.Display.ShowMillisecondsUnderSeconds)*time.Second {
			fmt.Printf("🚨 %s is slow across all pods (avg: %s)\n", phase, output.FormatDuration(stats.Avg))
		}
	}
}

func hasHighVariance(stats output.Stats) bool {
	return stats.Max > 2*stats.Min && stats.Min > 0
}

func DeploymentAnalysisWithNamespace(namespace, deploymentName string, profiles []*types.PodStartupProfile, cfg *config.Config) error {
	totalTimes, phaseStats := collectDeploymentStats(profiles)
	overallStats := output.CalculateStats(totalTimes)

	// Custom header with namespace
	fmt.Printf("\n%s %s/%s\n",
		color.New(color.Bold).Sprint("Deployment Startup Analysis:"),
		namespace,
		deploymentName)
	fmt.Printf("%s %d\n\n",
		color.New(color.Bold).Sprint("Pods analyzed:"),
		len(profiles))

	outputOverallStats(overallStats)
	outputPhaseStats(phaseStats)
	outputPodDetails(profiles, overallStats, cfg)
	outputDeploymentObservations(overallStats, phaseStats, cfg)

	fmt.Println()
	return nil
}
