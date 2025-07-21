package analyzer

import (
	"fmt"
	"strings"
	"time"

	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/types"
)

// detectPhases identifies startup phases from events and conditions
func (a *Analyzer) detectPhases(podInfo *collector.PodInfo) []types.PhaseInfo {
	var phases []types.PhaseInfo

	// Phase 1: Scheduling
	schedulingPhase := a.detectSchedulingPhase(podInfo)
	if schedulingPhase != nil {
		phases = append(phases, *schedulingPhase)
	}

	// Phase 2: Image Pull
	imagePullPhases := a.detectImagePullPhases(podInfo)
	phases = append(phases, imagePullPhases...)

	// Phase 3: Container Creation
	containerCreatePhase := a.detectContainerCreationPhase(podInfo)
	if containerCreatePhase != nil {
		phases = append(phases, *containerCreatePhase)
	}

	// Phase 4: Init Containers
	initPhases := a.detectInitContainerPhases(podInfo)
	if len(initPhases) > 0 {
		// Group init containers into a single phase with sub-phases
		initPhase := types.PhaseInfo{
			Phase:     types.StartupPhaseInitContainers,
			SubPhases: initPhases,
		}
		if len(initPhases) > 0 {
			initPhase.StartTime = initPhases[0].StartTime
			initPhase.EndTime = initPhases[len(initPhases)-1].EndTime
			initPhase.Duration = initPhase.EndTime.Sub(initPhase.StartTime)
		}
		phases = append(phases, initPhase)
	}

	// Phase 5: Application Start
	appStartPhase := a.detectApplicationStartPhase(podInfo)
	if appStartPhase != nil {
		phases = append(phases, *appStartPhase)
	}

	// Phase 6: Ready
	readyPhase := a.detectReadyPhase(podInfo)
	if readyPhase != nil {
		phases = append(phases, *readyPhase)
	}

	return phases
}

// detectSchedulingPhase identifies when the pod was scheduled to a node.
// It looks for:
// 1. PodScheduled condition (most reliable)
// 2. Scheduled event as fallback
// 3. Any event if pod already has a node (for pre-scheduled pods)
func (a *Analyzer) detectSchedulingPhase(podInfo *collector.PodInfo) *types.PhaseInfo {
	phase := &types.PhaseInfo{
		Phase:     types.StartupPhaseScheduling,
		StartTime: podInfo.CreationTimestamp,
	}

	// First, check PodScheduled condition as it's more reliable
	// This condition is set by the scheduler when a pod is assigned to a node
	for _, condition := range podInfo.Conditions {
		if condition.Type == "PodScheduled" && condition.Status == "True" {
			phase.EndTime = condition.LastTransitionTime
			phase.Duration = phase.EndTime.Sub(phase.StartTime)
			phase.Details = fmt.Sprintf("Scheduled to node %s", podInfo.NodeName)

			// If duration is 0 or negative, it was likely pre-scheduled or very fast
			if phase.Duration <= 0 {
				phase.Duration = 0
				phase.Details = fmt.Sprintf("Immediately scheduled to node %s", podInfo.NodeName)
			}
			return phase
		}
	}

	// Fall back to scheduled event if available
	for _, event := range podInfo.Events {
		if event.Reason == "Scheduled" && !event.Timestamp.IsZero() {
			phase.EndTime = event.Timestamp
			phase.Duration = phase.EndTime.Sub(phase.StartTime)
			phase.Details = fmt.Sprintf("Scheduled to node %s", podInfo.NodeName)
			return phase
		}
	}

	// If we have a node but no scheduling event/condition, assume immediate scheduling
	if podInfo.NodeName != "" {
		// Use the first event timestamp as a proxy for scheduling completion
		for _, event := range podInfo.Events {
			if !event.Timestamp.IsZero() {
				phase.EndTime = event.Timestamp
				phase.Duration = phase.EndTime.Sub(phase.StartTime)
				phase.Details = fmt.Sprintf("Scheduled to node %s (estimated)", podInfo.NodeName)
				return phase
			}
		}
	}

	return nil
}

// detectImagePullPhases identifies when container images were pulled.
// It matches "Pulling" and "Pulled" events to calculate pull duration.
// Multiple images may be pulled in parallel, so we track each separately.
func (a *Analyzer) detectImagePullPhases(podInfo *collector.PodInfo) []types.PhaseInfo {
	// Pre-allocate slice if we have image pulls
	phases := make([]types.PhaseInfo, 0, len(podInfo.ContainerStatuses))
	imagePullStart := make(map[string]time.Time, len(podInfo.ContainerStatuses))

	for _, event := range podInfo.Events {
		switch event.Reason {
		case "Pulling":
			// Extract image name from message
			if image := a.extractImageFromMessage(event.Message); image != "" {
				imagePullStart[image] = event.Timestamp
			}
		case "Pulled":
			// Match with pulling event
			if image := a.extractImageFromMessage(event.Message); image != "" {
				if startTime, ok := imagePullStart[image]; ok {
					phase := types.PhaseInfo{
						Phase:     types.StartupPhaseImagePull,
						StartTime: startTime,
						EndTime:   event.Timestamp,
						Duration:  event.Timestamp.Sub(startTime),
						Details:   fmt.Sprintf("Pulled image %s", image),
					}
					phases = append(phases, phase)
					// Remove from map to save memory
					delete(imagePullStart, image)
				}
			}
		}
	}

	// Handle incomplete pulls (still pulling)
	for image, startTime := range imagePullStart {
		// If we have a start but no end, the pull is still in progress
		phase := types.PhaseInfo{
			Phase:     types.StartupPhaseImagePull,
			StartTime: startTime,
			EndTime:   time.Now(), // Use current time as end
			Details:   fmt.Sprintf("Still pulling image %s", image),
		}
		phase.Duration = phase.EndTime.Sub(phase.StartTime)
		phases = append(phases, phase)
	}

	// If we have multiple image pulls, create a parent phase
	if len(phases) > 1 {
		parentPhase := types.PhaseInfo{
			Phase:     types.StartupPhaseImagePull,
			SubPhases: phases,
			StartTime: phases[0].StartTime,
			EndTime:   phases[len(phases)-1].EndTime,
		}
		parentPhase.Duration = parentPhase.EndTime.Sub(parentPhase.StartTime)
		return []types.PhaseInfo{parentPhase}
	}

	return phases
}

// detectContainerCreationPhase identifies container creation phase
func (a *Analyzer) detectContainerCreationPhase(podInfo *collector.PodInfo) *types.PhaseInfo {
	var created, started *collector.Event

	for i, event := range podInfo.Events {
		if event.Reason == "Created" && created == nil {
			created = &podInfo.Events[i]
		} else if event.Reason == "Started" && started == nil {
			started = &podInfo.Events[i]
			if created != nil {
				break
			}
		}
	}

	if created != nil && started != nil {
		return &types.PhaseInfo{
			Phase:     types.StartupPhaseContainerCreate,
			StartTime: created.Timestamp,
			EndTime:   started.Timestamp,
			Duration:  started.Timestamp.Sub(created.Timestamp),
			Details:   "Container created and started",
		}
	}

	return nil
}

// detectInitContainerPhases identifies init container execution phases
func (a *Analyzer) detectInitContainerPhases(podInfo *collector.PodInfo) []types.PhaseInfo {
	var phases []types.PhaseInfo

	for _, initContainer := range podInfo.InitContainers {
		if initContainer.State == "Terminated" && initContainer.StartedAt != nil && initContainer.FinishedAt != nil {
			phase := types.PhaseInfo{
				Phase:     types.StartupPhaseInitContainers,
				StartTime: *initContainer.StartedAt,
				EndTime:   *initContainer.FinishedAt,
				Duration:  initContainer.FinishedAt.Sub(*initContainer.StartedAt),
				Details:   fmt.Sprintf("Init container: %s", initContainer.Name),
			}
			phases = append(phases, phase)
		}
	}

	return phases
}

// detectApplicationStartPhase identifies when the main application started.
// This is the time between container start and pod ready.
// For pods with multiple containers, we use the earliest start time.
func (a *Analyzer) detectApplicationStartPhase(podInfo *collector.PodInfo) *types.PhaseInfo {
	// Find when main containers started
	var earliestStart, latestReady time.Time

	// Track each container's startup
	for _, container := range podInfo.ContainerStatuses {
		if container.StartedAt != nil {
			if earliestStart.IsZero() || container.StartedAt.Before(earliestStart) {
				earliestStart = *container.StartedAt
			}
		}
	}

	// Look for readiness
	for _, condition := range podInfo.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			latestReady = condition.LastTransitionTime
			break
		}
	}

	if !earliestStart.IsZero() && !latestReady.IsZero() {
		// If ready time is before start time, it's from a previous pod incarnation
		// This happens when a pod restarts but the Ready condition keeps the old timestamp
		// In this case, we should look for Ready events after the container start
		if latestReady.Before(earliestStart) {
			// Look for Ready events in the event stream
			for _, event := range podInfo.Events {
				if event.Reason == "Ready" && event.Timestamp.After(earliestStart) {
					latestReady = event.Timestamp
					break
				}
			}

			// If still no valid ready time, use current time if pod is ready
			if latestReady.Before(earliestStart) && podInfo.Phase == "Running" {
				// Check if all containers are ready
				allReady := true
				for _, container := range podInfo.ContainerStatuses {
					if !container.Ready {
						allReady = false
						break
					}
				}
				if allReady {
					// Use configured time after container start
					latestReady = earliestStart.Add(a.config.GetReadyTimeEstimation())
				} else {
					// Pod not ready yet, skip this phase
					return nil
				}
			}
		}

		details := "Application initialization"
		if len(podInfo.ContainerStatuses) > 0 {
			containerNames := make([]string, 0, len(podInfo.ContainerStatuses))
			for _, cs := range podInfo.ContainerStatuses {
				containerNames = append(containerNames, cs.Name)
			}

			if len(containerNames) == 1 {
				details = fmt.Sprintf("Application initialization (%s)", containerNames[0])
			} else if len(containerNames) <= 5 {
				details = fmt.Sprintf("Application initialization (%s)", strings.Join(containerNames, ", "))
			} else {
				// Show first 3 containers and count of remaining
				details = fmt.Sprintf("Application initialization (%s, +%d more)",
					strings.Join(containerNames[:3], ", "), len(containerNames)-3)
			}
		}

		duration := latestReady.Sub(earliestStart)
		if duration < 0 {
			// safeguard
			duration = 0
		}

		return &types.PhaseInfo{
			Phase:     types.StartupPhaseAppStart,
			StartTime: earliestStart,
			EndTime:   latestReady,
			Duration:  duration,
			Details:   details,
		}
	}

	return nil
}

// detectReadyPhase identifies when pod became ready
func (a *Analyzer) detectReadyPhase(podInfo *collector.PodInfo) *types.PhaseInfo {
	for _, condition := range podInfo.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return &types.PhaseInfo{
				Phase:     types.StartupPhaseReady,
				StartTime: condition.LastTransitionTime,
				EndTime:   condition.LastTransitionTime,
				Duration:  0,
				Details:   "Pod is ready",
			}
		}
	}
	return nil
}

// findReadyTime finds when the pod became ready
func (a *Analyzer) findReadyTime(podInfo *collector.PodInfo) time.Time {
	for _, condition := range podInfo.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return condition.LastTransitionTime
		}
	}
	return time.Time{}
}

// extractImageFromMessage extracts image name from event messages
func (a *Analyzer) extractImageFromMessage(message string) string {
	// Common patterns in Kubernetes events
	patterns := []string{
		"Pulling image \"",
		"Successfully pulled image \"",
		"pulling image \"",
		"pulled image \"",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(message, pattern); idx != -1 {
			start := idx + len(pattern)
			if end := strings.Index(message[start:], "\""); end != -1 {
				return message[start : start+end]
			}
		}
	}

	return ""
}
