package collector

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"

	"github.com/px4n/bootscope/pkg/constants"
	"github.com/px4n/bootscope/pkg/types"
)

// Collector gathers pod startup information from Kubernetes.
// It retrieves pods, events, and converts them to bootscope internal format.
//
// Thread Safety: The Collector is thread-safe for concurrent use.
// The Kubernetes clientset is thread-safe by design, and the Collector
// only reads from its config without modifying any shared state during operations.
// No mutex is needed as all methods are effectively stateless operations
// that use the immutable client and config references.
//
// Note: I made this comment in case security scanners/linters generate a false positive.
type Collector struct {
	clientset kubernetes.Interface
	config    *Config
}

// Config holds configuration for the collector
type Config struct {
	// Watch poll interval
	WatchPollInterval time.Duration

	// Network speed estimates for image size calculation
	NetworkSpeedAverage int64
	NetworkSpeedFast    int64
	FastPullThreshold   time.Duration

	// Registry patterns
	LocalRegistryHosts    []string
	PrivateNetworkCIDRs   []string
	ClusterDomainSuffixes []string
}

// DefaultCollectorConfig returns a Config with default values for testing
func DefaultCollectorConfig() *Config {
	return &Config{
		WatchPollInterval:     constants.DefaultPollInterval,
		NetworkSpeedAverage:   constants.DefaultNetworkSpeedMBps * constants.BytesPerMB,
		NetworkSpeedFast:      constants.FastNetworkSpeedMBps * constants.BytesPerMB,
		FastPullThreshold:     constants.DefaultFastPullThreshold,
		LocalRegistryHosts:    []string{"localhost", "127.0.0.1", "host.docker.internal"},
		PrivateNetworkCIDRs:   []string{"10.", "172.", "192.168."},
		ClusterDomainSuffixes: []string{".cluster.local"},
	}
}

func NewCollectorWithConfig(clientset kubernetes.Interface, cfg *Config) *Collector {
	return &Collector{
		clientset: clientset,
		config:    cfg,
	}
}

// CollectPodInfo gathers all startup-related information for a pod.
// This includes:
// - Pod spec and status
// - All events related to the pod
// - Container states and transitions
// - Image metadata and pull times
// - Resource waiting information
func (c *Collector) CollectPodInfo(ctx context.Context, namespace, podName string) (*PodInfo, error) {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	events, err := c.collectPodEvents(ctx, namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	// Build PodInfo
	podInfo := &PodInfo{
		Name:              pod.Name,
		Namespace:         pod.Namespace,
		UID:               string(pod.UID),
		ResourceVersion:   pod.ResourceVersion,
		CreationTimestamp: pod.CreationTimestamp.Time,
		Phase:             string(pod.Status.Phase),
		NodeName:          pod.Spec.NodeName,
		Events:            events,
		Images:            c.extractImages(pod),
	}

	// Collect image metadata
	podInfo.ImageMetadata = c.collectImageMetadata(pod, events)

	// Check for resource waiting
	podInfo.ResourceWaiting = c.detectResourceWaiting(pod, events)

	// Convert pod conditions
	for _, cond := range pod.Status.Conditions {
		podInfo.Conditions = append(podInfo.Conditions, PodCondition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			LastTransitionTime: cond.LastTransitionTime.Time,
			Reason:             cond.Reason,
			Message:            cond.Message,
		})
	}

	// Convert container statuses
	podInfo.ContainerStatuses = c.convertContainerStatuses(pod.Status.ContainerStatuses)
	podInfo.InitContainers = c.convertContainerStatuses(pod.Status.InitContainerStatuses)

	return podInfo, nil
}

// collectPodEvents retrieves events for a specific pod
func (c *Collector) collectPodEvents(ctx context.Context, namespace, podName string) ([]Event, error) {
	fieldSelector := fields.OneTermEqualSelector("involvedObject.name", podName).String()

	eventList, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	var events []Event
	for _, e := range eventList.Items {
		event := Event{
			Type:    e.Type,
			Reason:  e.Reason,
			Message: e.Message,
			Object:  e.InvolvedObject.Name,
		}

		// Use EventTime if available, otherwise fall back to FirstTimestamp
		if !e.EventTime.IsZero() {
			event.Timestamp = e.EventTime.Time
		} else if !e.FirstTimestamp.IsZero() {
			event.Timestamp = e.FirstTimestamp.Time
		} else {
			event.Timestamp = e.LastTimestamp.Time
		}

		events = append(events, event)
	}

	return events, nil
}

// convertContainerStatuses converts Kubernetes container statuses to our type
func (c *Collector) convertContainerStatuses(statuses []corev1.ContainerStatus) []ContainerStatus {
	var result []ContainerStatus

	for _, status := range statuses {
		cs := ContainerStatus{
			Name:         status.Name,
			Ready:        status.Ready,
			RestartCount: status.RestartCount,
		}

		// Determine current state
		if status.State.Running != nil {
			cs.State = "Running"
			startTime := status.State.Running.StartedAt.Time
			cs.StartedAt = &startTime
		} else if status.State.Waiting != nil {
			cs.State = "Waiting"
			cs.StateDetails = map[string]interface{}{
				"reason":  status.State.Waiting.Reason,
				"message": status.State.Waiting.Message,
			}
		} else if status.State.Terminated != nil {
			cs.State = "Terminated"
			startTime := status.State.Terminated.StartedAt.Time
			finishTime := status.State.Terminated.FinishedAt.Time
			cs.StartedAt = &startTime
			cs.FinishedAt = &finishTime
			cs.StateDetails = map[string]interface{}{
				"exitCode": status.State.Terminated.ExitCode,
				"reason":   status.State.Terminated.Reason,
				"message":  status.State.Terminated.Message,
			}
		}

		// Add LastTerminationState if container was restarted
		// This helps us understand why containers failed previously
		if status.LastTerminationState.Terminated != nil {
			// Initialize map if needed to avoid nil pointer panic
			if cs.StateDetails == nil {
				cs.StateDetails = make(map[string]interface{})
			}
			cs.StateDetails["lastTermination"] = map[string]interface{}{
				"exitCode":   status.LastTerminationState.Terminated.ExitCode,
				"reason":     status.LastTerminationState.Terminated.Reason,
				"finishedAt": status.LastTerminationState.Terminated.FinishedAt.Time,
			}
		}

		result = append(result, cs)
	}

	return result
}

// extractImages gets all container images from a pod
func (c *Collector) extractImages(pod *corev1.Pod) []string {
	images := make(map[string]bool)

	// Init containers
	for _, container := range pod.Spec.InitContainers {
		images[container.Image] = true
	}

	// Regular containers
	for _, container := range pod.Spec.Containers {
		images[container.Image] = true
	}

	// Convert map to slice
	var result []string
	for image := range images {
		result = append(result, image)
	}

	return result
}

// WatchPod monitors a pod until it reaches ready state or times out
func (c *Collector) WatchPod(ctx context.Context, namespace, podName string, timeout time.Duration) (*PodInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Set up a ticker to periodically check pod status
	ticker := time.NewTicker(c.config.WatchPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout reached, return current state
			return c.CollectPodInfo(context.Background(), namespace, podName)
		case <-ticker.C:
			podInfo, err := c.CollectPodInfo(ctx, namespace, podName)
			if err != nil {
				return nil, err
			}

			// Check if pod is ready
			if c.isPodReady(podInfo) {
				return podInfo, nil
			}

			// Check if pod has failed
			if podInfo.Phase == "Failed" || podInfo.Phase == "Succeeded" {
				return podInfo, nil
			}
		}
	}
}

// isPodReady checks if all containers in the pod are ready
func (c *Collector) isPodReady(podInfo *PodInfo) bool {
	// Check if pod has ready condition
	for _, condition := range podInfo.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
}

// collectImageMetadata gathers metadata about container images
func (c *Collector) collectImageMetadata(pod *corev1.Pod, events []Event) []types.ImageInfo {
	var imageInfos []types.ImageInfo
	imageMap := make(map[string]*types.ImageInfo)

	// Extract all unique images
	allImages := c.extractImages(pod)
	for _, image := range allImages {
		info := &types.ImageInfo{
			Name: image,
		}

		// Parse registry from image name
		registry, isLocal := c.parseImageRegistry(image)
		info.Registry = registry
		info.IsLocal = isLocal

		// Extract pull time from events
		info.PullTime = c.extractImagePullTime(image, events)

		// Try to get actual size from events first
		actualSize := c.extractImageSizeFromEvents(image, events)
		if actualSize > 0 {
			info.Size = actualSize
		} else {
			// Estimate size based on pull time (rough heuristic)
			// This is a placeholder - in production, we'd use container runtime API
			info.Size = c.estimateImageSize(info.PullTime)
		}

		imageMap[image] = info
	}

	// Convert map to slice
	for _, info := range imageMap {
		imageInfos = append(imageInfos, *info)
	}

	return imageInfos
}

// parseImageRegistry extracts registry information from image name.
// It determines if the registry is local (fast) or remote (potentially slow).
// Examples:
// - "nginx:latest" -> "docker.io", false (remote)
// - "localhost:5000/myapp" -> "localhost:5000", true (local)
// - "gcr.io/project/image" -> "gcr.io", false (remote)
func (c *Collector) parseImageRegistry(image string) (string, bool) {
	// Use configured local registries
	localRegistries := c.config.LocalRegistryHosts

	// Extract registry from image name
	parts := strings.Split(image, "/")
	if len(parts) > 1 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		registry := parts[0]

		// Check if it's a local registry
		for _, local := range localRegistries {
			if strings.HasPrefix(registry, local) {
				return registry, true
			}
		}

		// Check for cluster domain suffixes
		for _, suffix := range c.config.ClusterDomainSuffixes {
			if strings.Contains(registry, suffix) {
				return registry, true
			}
		}

		// Check for private network CIDRs
		for _, cidr := range c.config.PrivateNetworkCIDRs {
			if strings.HasPrefix(registry, cidr) {
				return registry, true
			}
		}

		return registry, false
	}

	// Default to Docker Hub
	return "docker.io", false
}

// extractImagePullTime finds how long an image took to pull
func (c *Collector) extractImagePullTime(image string, events []Event) time.Duration {
	var pullStart, pullEnd time.Time

	for _, event := range events {
		if strings.Contains(event.Message, image) {
			if event.Reason == "Pulling" && pullStart.IsZero() {
				pullStart = event.Timestamp
			} else if event.Reason == "Pulled" && !pullStart.IsZero() {
				pullEnd = event.Timestamp
				return pullEnd.Sub(pullStart)
			}
		}
	}

	return 0
}

// extractImageSizeFromEvents looks for actual image size in event messages
func (c *Collector) extractImageSizeFromEvents(image string, events []Event) int64 {
	for _, event := range events {
		if event.Reason == "Pulled" && strings.Contains(event.Message, image) {
			// Look for "Image size: X bytes" pattern in the message
			if strings.Contains(event.Message, "Image size:") {
				// Extract size from message like "Image size: 607722365 bytes"
				parts := strings.Split(event.Message, "Image size: ")
				if len(parts) > 1 {
					sizePart := strings.Split(parts[1], " bytes")[0]
					if size, err := strconv.ParseInt(sizePart, 10, 64); err == nil {
						return size
					}
				}
			}
		}
	}
	return 0
}

// estimateImageSize provides a rough estimate based on pull time.
// This is used when we can't get the actual size from events.
// Assumes average network speed of 25MB/s (200Mbps) by default.
func (c *Collector) estimateImageSize(pullTime time.Duration) int64 {
	if pullTime == 0 {
		return 0
	}

	// Use configured network speeds
	bytesPerSecond := c.config.NetworkSpeedAverage

	// For very short pull times, likely from cache or very small
	if pullTime < c.config.FastPullThreshold {
		bytesPerSecond = c.config.NetworkSpeedFast
	}

	return int64(pullTime.Seconds()) * bytesPerSecond
}

// detectResourceWaiting checks if pod was waiting for resources
func (c *Collector) detectResourceWaiting(pod *corev1.Pod, events []Event) *types.ResourceWaitInfo {
	// Common resource-related reasons
	resourceReasons := map[string]string{
		"FailedScheduling":         "Scheduling",
		"InsufficientCPU":          "CPU",
		"InsufficientMemory":       "Memory",
		"NodeNotReady":             "Node",
		"NodeNotFound":             "Node",
		"TaintsNotTolerated":       "Taints",
		"NodeSelectorMismatch":     "NodeSelector",
		"VolumeNodeAffinity":       "Volume",
		"NodeAffinity":             "Affinity",
		"PodAffinityRulesNotMatch": "PodAffinity",
	}

	var waitInfo *types.ResourceWaitInfo
	var waitStart, waitEnd time.Time
	var waitReason, waitMessage string

	// Check events for resource-related issues
	for _, event := range events {
		// First check if it's a scheduling failure
		if event.Reason == "FailedScheduling" && waitStart.IsZero() {
			waitStart = event.Timestamp
			waitMessage = event.Message

			// Parse the specific reason from the message
			for reason, resourceType := range resourceReasons {
				if strings.Contains(event.Message, reason) {
					waitReason = resourceType
					break
				}
			}

			// Default to Scheduling if no specific reason found
			if waitReason == "" {
				waitReason = "Scheduling"
			}
		}

		// Check if pod was scheduled (end of waiting)
		if event.Reason == "Scheduled" && !waitStart.IsZero() {
			waitEnd = event.Timestamp
			break
		}
	}

	// Check current pod conditions for ongoing resource issues
	if waitEnd.IsZero() && pod.Status.Phase == "Pending" {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "PodScheduled" && condition.Status != "True" {
				if waitStart.IsZero() {
					waitStart = pod.CreationTimestamp.Time
				}
				waitEnd = time.Now()
				if waitReason == "" {
					// Parse reason from message
					waitReason = "Scheduling"
					waitMessage = condition.Message

					// Check message for specific reasons
					for reason, resourceType := range resourceReasons {
						if strings.Contains(condition.Message, reason) {
							waitReason = resourceType
							break
						}
					}
				}
			}
		}
	}

	// If we found resource waiting, create the info
	if !waitStart.IsZero() && !waitEnd.IsZero() {
		waitInfo = &types.ResourceWaitInfo{
			Type:     waitReason,
			Reason:   waitMessage,
			Duration: waitEnd.Sub(waitStart),
			Message:  fmt.Sprintf("Pod waited %v for %s resources", waitEnd.Sub(waitStart), waitReason),
		}
	}

	return waitInfo
}
