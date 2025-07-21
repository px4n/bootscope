package analyzer

import (
	"fmt"
	"sort"
	"time"

	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/constants"
	"github.com/px4n/bootscope/pkg/types"
)

func (a *Analyzer) generateRecommendations(podInfo *collector.PodInfo, profile *types.PodStartupProfile) []types.Recommendation {
	var recommendations []types.Recommendation

	// Check for slow image pulls
	for _, phase := range profile.Phases {
		if phase.Phase == types.StartupPhaseImagePull && phase.Duration > a.config.GetImagePullSlowThreshold() {
			timeSaved := phase.Duration * time.Duration(a.config.Performance.ImagePullOptimizationPercent) / 100
			recommendations = append(recommendations, types.Recommendation{
				Title:       "Optimize Image Pull Time",
				Description: "Image pull is taking a significant amount of time. Consider using a local registry or ensuring nodes have the image pre-pulled.",
				Impact:      fmt.Sprintf("Could save approximately %v", timeSaved),
				TimeSaved:   timeSaved,
				Link:        "https://kubernetes.io/docs/concepts/containers/images/#pre-pull-images",
				Priority:    1,
			})
		}
	}

	// Check ImageMetadata directly for large images and remote registries
	if profile.ImageMetadata != nil {
		for _, imgInfo := range profile.ImageMetadata {
			// Check for large images
			if imgInfo.Size > a.config.GetLargeImageThreshold() {
				recommendations = append(recommendations, types.Recommendation{
					Title: "Large Image Detected",
					Description: fmt.Sprintf("Image '%s' is approximately %.0fMB. Consider optimizing using multi-stage builds or slimmer base images.",
						imgInfo.Name, float64(imgInfo.Size)/float64(constants.BytesPerMB)),
					Impact:   "Could reduce image size by 50-70%",
					Link:     "https://docs.docker.com/develop/develop-images/dockerfile_best-practices/",
					Priority: 1,
				})
			}

			// Check for remote registry with significant pull time
			if !imgInfo.IsLocal && imgInfo.PullTime > a.config.GetSlowPhaseThreshold() {
				timeSaved := imgInfo.PullTime * time.Duration(a.config.Performance.LocalRegistryOptimizationPercent) / 100
				recommendations = append(recommendations, types.Recommendation{
					Title: "Use Local Registry",
					Description: fmt.Sprintf("Image '%s' is being pulled from remote registry '%s'. Consider using a local registry mirror or image cache.",
						imgInfo.Name, imgInfo.Registry),
					Impact:    fmt.Sprintf("Could save approximately %v per pod startup", timeSaved),
					TimeSaved: timeSaved,
					Link:      "https://kubernetes.io/docs/concepts/containers/images/#using-a-private-registry",
					Priority:  2,
				})
			}
		}
	}

	// Check for slow init containers
	for _, phase := range profile.Phases {
		if phase.Phase == types.StartupPhaseInitContainers {
			if phase.Duration >= a.config.GetInitContainerSlowThreshold() {
				timeSaved := phase.Duration * time.Duration(a.config.Performance.InitContainerOptimizationPercent) / 100
				recommendations = append(recommendations, types.Recommendation{
					Title:       "Optimize Init Container Logic",
					Description: "Init containers are taking a long time. Consider parallelizing operations or moving non-critical initialization to the main container.",
					Impact:      fmt.Sprintf("Could save approximately %v", timeSaved),
					TimeSaved:   timeSaved,
					Link:        "https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#init-container-patterns",
					Priority:    2,
				})
			}
		}
	}

	// Check for slow application startup
	for _, phase := range profile.Phases {
		if phase.Phase == types.StartupPhaseAppStart && phase.Duration > a.config.GetAppStartSlowThreshold() {
			recommendations = append(recommendations, types.Recommendation{
				Title:       "Optimize Application Startup",
				Description: "Application initialization is slow. Consider lazy loading, startup probes, or optimizing initialization logic.",
				Impact:      "Faster readiness means quicker traffic serving",
				Link:        "https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/",
				Priority:    3,
			})
		}
	}

	// Check for resource waiting
	if profile.ResourceWaiting != nil && profile.ResourceWaiting.Duration > a.config.GetResourceWaitingThreshold() {
		var title, description string
		switch profile.ResourceWaiting.Type {
		case "CPU", "Memory":
			title = fmt.Sprintf("Insufficient %s Resources", profile.ResourceWaiting.Type)
			description = fmt.Sprintf("Pod waited %v for %s resources. Consider increasing cluster capacity or adjusting resource requests.",
				profile.ResourceWaiting.Duration, profile.ResourceWaiting.Type)
		case "Node":
			title = "Node Availability Issues"
			description = fmt.Sprintf("Pod waited %v for suitable node. Check node health and capacity.",
				profile.ResourceWaiting.Duration)
		case "NodeSelector", "Taints", "Affinity":
			title = "Scheduling Constraints Too Restrictive"
			description = fmt.Sprintf("Pod waited %v due to %s constraints. Consider relaxing scheduling requirements.",
				profile.ResourceWaiting.Duration, profile.ResourceWaiting.Type)
		default:
			title = "Resource Scheduling Delay"
			description = fmt.Sprintf("Pod experienced %v delay in scheduling: %s",
				profile.ResourceWaiting.Duration, profile.ResourceWaiting.Reason)
		}

		recommendations = append(recommendations, types.Recommendation{
			Title:       title,
			Description: description,
			Impact:      fmt.Sprintf("Eliminating resource wait would save %v", profile.ResourceWaiting.Duration),
			TimeSaved:   profile.ResourceWaiting.Duration,
			Link:        "https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/",
			Priority:    1,
		})
	}

	// Sort by priority
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Priority < recommendations[j].Priority
	})

	return recommendations
}

// identifyBottlenecks finds the slowest phases
func (a *Analyzer) identifyBottlenecks(phases []types.PhaseInfo, totalTime time.Duration) []types.Bottleneck {
	var bottlenecks []types.Bottleneck

	// Skip if total time is 0
	if totalTime == 0 {
		return bottlenecks
	}

	// Count phases with actual time
	phasesWithTime := 0
	for _, phase := range phases {
		if phase.Duration > 0 {
			phasesWithTime++
		}
	}

	// Check if total time is below minimum threshold
	minTotalTime := a.config.GetBottleneckMinTotalTime()
	isFastStartup := totalTime < minTotalTime

	for _, phase := range phases {
		if phase.Duration == 0 {
			continue
		}

		percentage := float64(phase.Duration) / float64(totalTime) * 100
		isBottleneck := false
		severity := "info"

		// Special handling when only one phase has time
		if phasesWithTime == 1 {
			// For fast startups, only flag if the phase itself is slow
			if isFastStartup {
				if phase.Duration > a.config.GetSlowPhaseThreshold() {
					isBottleneck = true
					severity = "warning"
				}
			} else {
				// For slow startups, always flag the single phase
				isBottleneck = true
				if phase.Duration > a.config.GetSlowPhaseThreshold()*2 {
					severity = "critical"
				} else if phase.Duration > a.config.GetSlowPhaseThreshold() {
					severity = "warning"
				}
			}
		} else {
			// Multiple phases - use percentage-based detection
			if percentage > a.config.Thresholds.BottleneckCriticalPercentage {
				isBottleneck = true
				severity = "critical"
			} else if percentage > a.config.Thresholds.BottleneckPercentage {
				isBottleneck = true
				severity = "warning"
			}

			// Also check absolute duration
			if phase.Duration > a.config.GetSlowPhaseThreshold() && !isBottleneck {
				isBottleneck = true
				severity = "info"
			}
		}

		if isBottleneck {
			bottlenecks = append(bottlenecks, types.Bottleneck{
				Phase:       phase.Phase,
				Duration:    phase.Duration,
				Percentage:  percentage,
				Description: fmt.Sprintf("%s took %.1f%% of total startup time", phase.Phase, percentage),
				Severity:    severity,
			})
		}
	}

	// Sort by duration (longest first)
	sort.Slice(bottlenecks, func(i, j int) bool {
		return bottlenecks[i].Duration > bottlenecks[j].Duration
	})

	return bottlenecks
}
