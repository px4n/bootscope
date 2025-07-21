package analyzer

import (
	"sort"
	"time"

	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/types"
)

// Analyzer processes pod information to create startup profiles.
// It coordinates phase detection, bottleneck analysis, and recommendation generation.
type Analyzer struct {
	config *config.Config
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		config: config.DefaultConfig(),
	}
}

func NewAnalyzerWithConfig(cfg *config.Config) *Analyzer {
	return &Analyzer{
		config: cfg,
	}
}

// AnalyzePod creates a startup profile from pod information.
// It performs the following steps:
// 1. Detects startup phases from events and pod conditions
// 2. Fixes timing issues caused by pod restarts or overlapping phases
// 3. Identifies bottlenecks based on phase duration
// 4. Generates actionable recommendations
func (a *Analyzer) AnalyzePod(podInfo *collector.PodInfo) (*types.PodStartupProfile, error) {
	profile := &types.PodStartupProfile{
		PodName:   podInfo.Name,
		Namespace: podInfo.Namespace,
		StartTime: podInfo.CreationTimestamp,
		Status:    podInfo.Phase,
	}

	// Sort events by timestamp
	sort.Slice(podInfo.Events, func(i, j int) bool {
		return podInfo.Events[i].Timestamp.Before(podInfo.Events[j].Timestamp)
	})

	// Detect phases from events and conditions
	// This builds a timeline of what happened during pod startup
	phases := a.detectPhases(podInfo)

	// Fix timing issues (overlaps, negative durations)
	// This handles edge cases like pod restarts and concurrent phases
	phases = a.fixPhaseTimings(phases)

	// Consolidate duplicate phases (from restarts)
	// When a pod restarts, we only care about the most recent attempt
	phases = a.consolidateDuplicatePhases(phases)

	profile.Phases = phases

	// Calculate total time with better handling for never-ready pods
	readyTime := a.findReadyTime(podInfo)
	if readyTime.IsZero() {
		// Pod never became ready - use appropriate end time
		if podInfo.Phase == "Failed" || podInfo.Phase == "Succeeded" {
			// Use the last phase end time for terminated pods
			if len(phases) > 0 {
				readyTime = phases[len(phases)-1].EndTime
			}
		} else if podInfo.Phase == "Running" {
			// For running but not ready pods, check if containers are crashing
			hasTerminatedContainers := false
			for _, cs := range podInfo.ContainerStatuses {
				if cs.State == "Terminated" || cs.State == "Waiting" {
					hasTerminatedContainers = true
					break
				}
			}

			if hasTerminatedContainers && len(phases) > 0 {
				// Use last known activity time
				readyTime = phases[len(phases)-1].EndTime
			} else {
				// Pod is still trying to start, use current time
				readyTime = time.Now()
			}
		} else if podInfo.Phase == "Pending" {
			// For pending pods, use current time or last phase end time
			if len(phases) > 0 {
				readyTime = phases[len(phases)-1].EndTime
			} else {
				readyTime = time.Now()
			}
		}
	}

	// Set ready time if we found one
	if !readyTime.IsZero() {
		profile.ReadyTime = readyTime
		profile.TotalTime = readyTime.Sub(profile.StartTime)
	}

	// Ensure we don't have negative total time
	if profile.TotalTime < 0 {
		profile.TotalTime = 0
	}

	// Copy image metadata and resource waiting info BEFORE generating recommendations
	// This is important because recommendations depend on this data
	profile.ImageMetadata = podInfo.ImageMetadata
	profile.ResourceWaiting = podInfo.ResourceWaiting

	// Extract container status information
	profile.ContainerStatuses = a.extractContainerStatuses(podInfo)

	// Update pod status based on container states
	profile.Status = a.determinePodStatus(podInfo)

	// Identify bottlenecks - phases that took significant time
	profile.Bottlenecks = a.identifyBottlenecks(phases, profile.TotalTime)

	// Generate recommendations based on the analysis
	// This looks at image sizes, pull times, init containers, etc.
	profile.Recommendations = a.generateRecommendations(podInfo, profile)

	return profile, nil
}

// extractContainerStatuses converts collector container status to API format
func (a *Analyzer) extractContainerStatuses(podInfo *collector.PodInfo) []types.ContainerStatusInfo {
	var statuses []types.ContainerStatusInfo

	for _, cs := range podInfo.ContainerStatuses {
		status := types.ContainerStatusInfo{
			Name:         cs.Name,
			RestartCount: cs.RestartCount,
			Ready:        cs.Ready,
			State:        cs.State,
		}

		// Extract exit code and reason from state details
		if cs.State == "Terminated" && cs.StateDetails != nil {
			if exitCode, ok := cs.StateDetails["exitCode"].(float64); ok {
				exitCodeInt := int32(exitCode)
				status.ExitCode = &exitCodeInt
			}
			if reason, ok := cs.StateDetails["reason"].(string); ok {
				status.Reason = reason
			}
		} else if cs.State == "Waiting" && cs.StateDetails != nil {
			if reason, ok := cs.StateDetails["reason"].(string); ok {
				status.Reason = reason
			}
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// determinePodStatus provides more accurate pod status based on container states
func (a *Analyzer) determinePodStatus(podInfo *collector.PodInfo) string {
	// Check for CrashLoopBackOff
	for _, cs := range podInfo.ContainerStatuses {
		if cs.State == "Waiting" && cs.StateDetails != nil {
			if reason, ok := cs.StateDetails["reason"].(string); ok {
				if reason == "CrashLoopBackOff" {
					return "CrashLoopBackOff"
				}
			}
		}
	}

	// Check if all containers are terminated
	allTerminated := len(podInfo.ContainerStatuses) > 0
	hasRestarts := false
	for _, cs := range podInfo.ContainerStatuses {
		if cs.State != "Terminated" {
			allTerminated = false
		}
		if cs.RestartCount > 0 {
			hasRestarts = true
		}
	}

	if allTerminated {
		return "Failed"
	}

	if hasRestarts && podInfo.Phase == "Running" {
		return "Running (Restarted)"
	}

	// Default to the pod phase
	return podInfo.Phase
}
