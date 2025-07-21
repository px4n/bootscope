package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"gopkg.in/yaml.v3"

	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/errors"
	"github.com/px4n/bootscope/pkg/output"
	"github.com/px4n/bootscope/pkg/types"
)

func Text(profile *types.PodStartupProfile, cfg *config.Config) error {
	outputHeader(profile)
	outputTotalTime(profile, cfg)
	outputStatus(profile)
	outputPhases(profile, cfg)
	outputBottlenecks(profile)
	outputRecommendations(profile)
	fmt.Println()
	return nil
}

func outputHeader(profile *types.PodStartupProfile) {
	fmt.Printf("\n%s %s/%s\n",
		color.New(color.Bold).Sprint("Pod Startup Profile:"),
		profile.Namespace,
		profile.PodName)

	// Show restart information if present
	totalRestarts := int32(0)
	for _, cs := range profile.ContainerStatuses {
		totalRestarts += cs.RestartCount
	}
	if totalRestarts > 0 {
		fmt.Printf("%s %d\n",
			color.New(color.FgYellow).Sprint("⚠️  Container Restarts:"),
			totalRestarts)
	}
}

func outputTotalTime(profile *types.PodStartupProfile, cfg *config.Config) {
	totalTimeStr := output.FormatDuration(profile.TotalTime)
	indicator := getTotalTimeIndicator(profile.TotalTime, cfg)
	fmt.Printf("%s %s %s\n",
		color.New(color.Bold).Sprint("Total Time:"),
		totalTimeStr,
		indicator)
}

func getTotalTimeIndicator(totalTime time.Duration, cfg *config.Config) string {
	switch {
	case totalTime >= time.Duration(cfg.Display.TotalTimeSlowSeconds)*time.Second:
		return "🐌" // Very slow
	case totalTime >= time.Duration(cfg.Display.TotalTimeModerateSeconds)*time.Second:
		return "⚠️" // Slow
	case totalTime >= time.Duration(cfg.Display.TotalTimeFastSeconds)*time.Second:
		return "🕰️" // Moderate
	default:
		return "✅" // Fast
	}
}

func outputStatus(profile *types.PodStartupProfile) {
	statusColor := color.New(color.FgGreen)
	statusText := profile.Status

	// Special handling for specific statuses
	switch profile.Status {
	case "CrashLoopBackOff":
		statusColor = color.New(color.FgRed, color.Bold)
		statusText = "CrashLoopBackOff ⚠️"
	case "Failed":
		statusColor = color.New(color.FgRed)
	case "Running (Restarted)":
		statusColor = color.New(color.FgYellow)
	case "Running":
		if len(profile.ContainerStatuses) > 0 {
			allReady := true
			for _, cs := range profile.ContainerStatuses {
				if !cs.Ready {
					allReady = false
					break
				}
			}
			if !allReady {
				statusColor = color.New(color.FgYellow)
				statusText = "Running (Not Ready)"
			}
		}
	}

	fmt.Printf("%s %s\n\n",
		color.New(color.Bold).Sprint("Status:"),
		statusColor.Sprint(statusText))
}

func outputPhases(profile *types.PodStartupProfile, cfg *config.Config) {
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("Phase Breakdown:"))
	for i, phase := range profile.Phases {
		outputPhase(profile, phase, i, len(profile.Phases), cfg)
	}
}

func outputPhase(profile *types.PodStartupProfile, phase types.PhaseInfo, index, total int, cfg *config.Config) {
	prefix := "├─"
	if index == total-1 {
		prefix = "└─"
	}

	phaseDuration := output.FormatDuration(phase.Duration)
	var percentage float64
	if profile.TotalTime > 0 {
		percentage = float64(phase.Duration) / float64(profile.TotalTime) * 100
	}

	durationColor := getPhaseDurationColor(phase.Duration, cfg)
	fmt.Printf("%s %s: %s (%s)",
		prefix,
		phase.Phase,
		durationColor.Sprint(phaseDuration),
		output.FormatPercentage(percentage))

	if phase.Duration >= time.Duration(cfg.Display.PhaseDurationGreenSeconds)*time.Second {
		fmt.Print(" " + color.New(color.FgRed).Sprint("⚠️"))
	}

	if phase.Details != "" {
		fmt.Printf(" - %s", color.New(color.Faint).Sprint(phase.Details))
	}
	fmt.Println()

	outputSubPhases(phase.SubPhases, index == total-1)
}

func getPhaseDurationColor(duration time.Duration, cfg *config.Config) *color.Color {
	if duration >= time.Duration(cfg.Display.PhaseDurationYellowSeconds)*time.Second {
		return color.New(color.FgRed)
	} else if duration >= time.Duration(cfg.Display.PhaseDurationGreenSeconds)*time.Second {
		return color.New(color.FgYellow)
	}
	return color.New(color.FgGreen)
}

func outputSubPhases(subPhases []types.PhaseInfo, isLastPhase bool) {
	for j, subPhase := range subPhases {
		subPrefix := getSubPhasePrefix(j, len(subPhases), isLastPhase)
		fmt.Printf("%s %s: %s\n",
			subPrefix,
			color.New(color.Faint).Sprint(subPhase.Details),
			output.FormatDuration(subPhase.Duration))
	}
}

func getSubPhasePrefix(index, total int, isLastPhase bool) string {
	if isLastPhase {
		if index == total-1 {
			return "   └─"
		}
		return "   ├─"
	}
	if index == total-1 {
		return "│  └─"
	}
	return "│  ├─"
}

func outputBottlenecks(profile *types.PodStartupProfile) {
	if len(profile.Bottlenecks) == 0 {
		return
	}

	fmt.Println("\n" + color.New(color.Bold, color.Underline).Sprint("Bottlenecks Identified:"))
	for _, bottleneck := range profile.Bottlenecks {
		icon := getBottleneckIcon(bottleneck.Severity)
		fmt.Printf("%s %s\n", icon, bottleneck.Description)
	}
}

func getBottleneckIcon(severity string) string {
	switch severity {
	case "warning":
		return "⚠️"
	case "critical":
		return "🚨"
	default:
		return "ℹ️"
	}
}

func outputRecommendations(profile *types.PodStartupProfile) {
	if len(profile.Recommendations) == 0 {
		return
	}

	fmt.Println("\n" + color.New(color.Bold, color.Underline).Sprint("Recommendations:"))
	for i, rec := range profile.Recommendations {
		fmt.Printf("\n%d. %s\n", i+1, color.New(color.Bold).Sprint(rec.Title))
		fmt.Printf("   %s\n", rec.Description)
		if rec.Impact != "" {
			fmt.Printf("   %s %s\n",
				color.New(color.FgGreen).Sprint("Impact:"),
				rec.Impact)
		}
		if rec.Link != "" {
			fmt.Printf("   %s %s\n",
				color.New(color.FgBlue).Sprint("Learn more:"),
				rec.Link)
		}
	}
}

func Simple(profile *types.PodStartupProfile, cfg *config.Config) error {
	if profile.TotalTime >= time.Duration(cfg.Display.TotalTimeModerateSeconds)*time.Second {
		fmt.Println("Your pod is starting slowly! Here's why:")
	} else {
		fmt.Printf("Your pod started in %s. Here's the breakdown:\n", output.FormatDuration(profile.TotalTime))
	}
	fmt.Println()

	// Simple phase breakdown
	for i, phase := range profile.Phases {
		icon := "✅"
		if phase.Duration >= time.Duration(cfg.Display.PhaseDurationGreenSeconds)*time.Second {
			icon = "⚠️"
		} else if phase.Duration >= time.Duration(cfg.Display.ShowMillisecondsUnderSeconds)*time.Second {
			icon = "🕰️"
		}

		phaseName := simplifyPhaseName(phase.Phase)
		fmt.Printf("%d. %s %s: %s\n",
			i+1,
			icon,
			phaseName,
			output.FormatDuration(phase.Duration))

		// Add simple explanation for slow phases
		if phase.Duration >= time.Duration(cfg.Display.PhaseDurationGreenSeconds)*time.Second {
			fmt.Printf("   %s\n", getSimpleExplanation(phase))
		}
	}

	// Simple recommendations
	if len(profile.Recommendations) > 0 {
		fmt.Println("\n How to potentially make it faster:")
		for i, rec := range profile.Recommendations {
			fmt.Printf("\n%d. %s\n", i+1, rec.Title)
			fmt.Printf("   Fix: %s\n", simplifyRecommendation(rec))
			if rec.TimeSaved > 0 {
				fmt.Printf("   Time saved: ~%s\n", output.FormatDuration(rec.TimeSaved))
			}
			if rec.Link != "" {
				fmt.Printf("   Example: %s\n", rec.Link)
			}
		}
	}

	// Summary
	fmt.Printf("\nTotal time saved if fixed: ")
	var totalSaved time.Duration
	for _, rec := range profile.Recommendations {
		totalSaved += rec.TimeSaved
	}
	fmt.Printf("%s 🎉\n", output.FormatDuration(totalSaved))

	return nil
}

func JSON(profile *types.PodStartupProfile) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(profile)
}

func YAML(profile *types.PodStartupProfile) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	if err := encoder.Encode(profile); err != nil {
		return errors.WrapFailure("encode YAML", err)
	}
	return encoder.Close()
}

// Helper functions
func simplifyPhaseName(phase types.StartupPhase) string {
	switch phase {
	case types.StartupPhaseScheduling:
		return "Finding a node"
	case types.StartupPhaseImagePull:
		return "Downloading container image"
	case types.StartupPhaseContainerCreate:
		return "Creating container"
	case types.StartupPhaseInitContainers:
		return "Running setup tasks"
	case types.StartupPhaseAppStart:
		return "Starting your application"
	case types.StartupPhaseReady:
		return "Ready to serve traffic"
	default:
		return string(phase)
	}
}

func getSimpleExplanation(phase types.PhaseInfo) string {
	switch phase.Phase {
	case types.StartupPhaseImagePull:
		return "Your container image is large or being downloaded from far away"
	case types.StartupPhaseInitContainers:
		return "Setup tasks are taking a long time"
	case types.StartupPhaseAppStart:
		return "Your application is slow to initialize"
	default:
		return "This step is taking longer than expected"
	}
}

func simplifyRecommendation(rec types.Recommendation) string {
	if strings.Contains(rec.Title, "Image") {
		return "Make your Docker image smaller using multi-stage builds"
	}
	if strings.Contains(rec.Title, "Init Container") {
		return "Move slow setup tasks to happen after your app starts"
	}
	if strings.Contains(rec.Title, "Application Startup") {
		return "Use lazy loading or optimize your app's initialization code"
	}
	return rec.Description
}

func Debug(podInfo *collector.PodInfo, profile *types.PodStartupProfile) {
	fmt.Println(color.New(color.Bold, color.FgYellow).Sprint("=== DEBUG INFORMATION ==="))

	// Pod basic info
	fmt.Printf("\nPod: %s/%s\n", podInfo.Namespace, podInfo.Name)
	fmt.Printf("Created: %s\n", podInfo.CreationTimestamp.Format(time.RFC3339))
	fmt.Printf("Node: %s\n", podInfo.NodeName)
	fmt.Printf("Phase: %s\n", podInfo.Phase)

	// Events with timestamps
	fmt.Println("\nEvents (sorted by time):")
	fmt.Println(strings.Repeat("-", 80))
	for _, event := range podInfo.Events {
		timestamp := "nil"
		if !event.Timestamp.IsZero() {
			timestamp = event.Timestamp.Format(time.RFC3339)
		}
		fmt.Printf("%s | %-12s | %s\n", timestamp, event.Reason, event.Message)
	}

	// Conditions
	fmt.Println("\nConditions:")
	fmt.Println(strings.Repeat("-", 80))
	for _, condition := range podInfo.Conditions {
		fmt.Printf("%s | %-25s | %s\n",
			condition.LastTransitionTime.Format(time.RFC3339),
			condition.Type,
			condition.Status)
	}

	// Container statuses
	fmt.Println("\nContainer Statuses:")
	fmt.Println(strings.Repeat("-", 80))
	for _, cs := range podInfo.ContainerStatuses {
		fmt.Printf("Container: %s\n", cs.Name)
		fmt.Printf("  State: %s\n", cs.State)
		fmt.Printf("  RestartCount: %d\n", cs.RestartCount)
		if cs.StartedAt != nil {
			fmt.Printf("  Started: %s\n", cs.StartedAt.Format(time.RFC3339))
		}
		if cs.FinishedAt != nil {
			fmt.Printf("  Finished: %s\n", cs.FinishedAt.Format(time.RFC3339))
		}
		fmt.Printf("  Ready: %v\n", cs.Ready)
		if len(cs.StateDetails) > 0 {
			fmt.Printf("  Details: %v\n", cs.StateDetails)
		}
	}

	// Detected phases
	fmt.Println("\nDetected Phases:")
	fmt.Println(strings.Repeat("-", 80))
	for _, phase := range profile.Phases {
		fmt.Printf("%-20s | Start: %s | End: %s | Duration: %s\n",
			phase.Phase,
			phase.StartTime.Format(time.RFC3339),
			phase.EndTime.Format(time.RFC3339),
			output.FormatDurationPrecise(phase.Duration))
	}

	// Calculations
	fmt.Println("\nTiming Calculations:")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total time: %s (from %s to %s)\n",
		profile.TotalTime,
		profile.StartTime.Format(time.RFC3339),
		profile.ReadyTime.Format(time.RFC3339))
}
