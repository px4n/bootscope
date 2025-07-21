// Package types defines the public types used by the pod startup profiler.
// These types are stable and can be used by external consumers.
package types

import (
	"time"
)

// ============================================================================
// Primary Types
// ============================================================================

// PodStartupProfile contains the complete analysis of a pod's startup sequence.
// It includes timing information, bottlenecks, and recommendations.
type PodStartupProfile struct {
	// Basic information
	PodName   string `json:"podName" yaml:"podName"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Status    string `json:"status" yaml:"status"`

	// Timing information
	StartTime time.Time     `json:"startTime" yaml:"startTime"`
	ReadyTime time.Time     `json:"readyTime,omitempty" yaml:"readyTime,omitempty"`
	TotalTime time.Duration `json:"totalTime" yaml:"totalTime"`

	// Startup phases breakdown
	Phases []PhaseInfo `json:"phases" yaml:"phases"`

	// Analysis results
	Bottlenecks     []Bottleneck     `json:"bottlenecks,omitempty" yaml:"bottlenecks,omitempty"`
	Recommendations []Recommendation `json:"recommendations,omitempty" yaml:"recommendations,omitempty"`

	// Additional metadata
	ContainerStatuses []ContainerStatusInfo `json:"containerStatuses,omitempty" yaml:"containerStatuses,omitempty"`
	ImageMetadata     []ImageInfo           `json:"imageMetadata,omitempty" yaml:"imageMetadata,omitempty"`
	ResourceWaiting   *ResourceWaitInfo     `json:"resourceWaiting,omitempty" yaml:"resourceWaiting,omitempty"`
}

// ============================================================================
// Phase Types
// ============================================================================

// StartupPhase represents different stages of pod startup
type StartupPhase string

// Pod startup phase constants
const (
	// StartupPhaseScheduling represents the time to find a suitable node
	StartupPhaseScheduling StartupPhase = "Scheduling"
	// StartupPhaseImagePull represents the time to pull container images
	StartupPhaseImagePull StartupPhase = "ImagePull"
	// StartupPhaseContainerCreate represents the time to create containers
	StartupPhaseContainerCreate StartupPhase = "ContainerCreation"
	// StartupPhaseInitContainers represents the time for init containers to complete
	StartupPhaseInitContainers StartupPhase = "InitContainers"
	// StartupPhaseAppStart represents the time from container start to ready
	StartupPhaseAppStart StartupPhase = "ApplicationStart"
	// StartupPhaseReady represents when the pod becomes ready
	StartupPhaseReady StartupPhase = "Ready"
)

// PhaseInfo represents a single phase in the pod startup process.
// Phases can have sub-phases for more detailed analysis.
type PhaseInfo struct {
	Phase     StartupPhase  `json:"phase" yaml:"phase"`
	StartTime time.Time     `json:"startTime" yaml:"startTime"`
	EndTime   time.Time     `json:"endTime" yaml:"endTime"`
	Duration  time.Duration `json:"duration" yaml:"duration"`
	Details   string        `json:"details,omitempty" yaml:"details,omitempty"`
	SubPhases []PhaseInfo   `json:"subPhases,omitempty" yaml:"subPhases,omitempty"`
}

// ============================================================================
// Analysis Types
// ============================================================================

// Bottleneck identifies a phase that took significant time during startup
type Bottleneck struct {
	Phase       StartupPhase  `json:"phase" yaml:"phase"`
	Duration    time.Duration `json:"duration" yaml:"duration"`
	Percentage  float64       `json:"percentage" yaml:"percentage"`
	Description string        `json:"description" yaml:"description"`
	Severity    string        `json:"severity" yaml:"severity"` // info, warning, critical
}

// Recommendation provides actionable advice for improving startup time
type Recommendation struct {
	Title       string        `json:"title" yaml:"title"`
	Description string        `json:"description" yaml:"description"`
	Impact      string        `json:"impact" yaml:"impact"`
	Priority    int           `json:"priority" yaml:"priority"` // 1 = highest priority
	TimeSaved   time.Duration `json:"timeSaved,omitempty" yaml:"timeSaved,omitempty"`
	Link        string        `json:"link,omitempty" yaml:"link,omitempty"`
}

// ============================================================================
// Container and Resource Types
// ============================================================================

// ContainerStatusInfo provides status information about containers
type ContainerStatusInfo struct {
	Name         string `json:"name" yaml:"name"`
	State        string `json:"state" yaml:"state"`
	Ready        bool   `json:"ready" yaml:"ready"`
	RestartCount int32  `json:"restartCount" yaml:"restartCount"`
	ExitCode     *int32 `json:"exitCode,omitempty" yaml:"exitCode,omitempty"`
	Reason       string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// ImageInfo contains metadata about container images
type ImageInfo struct {
	Name     string        `json:"name" yaml:"name"`
	Size     int64         `json:"size,omitempty" yaml:"size,omitempty"`
	Registry string        `json:"registry,omitempty" yaml:"registry,omitempty"`
	IsLocal  bool          `json:"isLocal,omitempty" yaml:"isLocal,omitempty"`
	PullTime time.Duration `json:"pullTime,omitempty" yaml:"pullTime,omitempty"`
}

// ResourceWaitInfo describes why a pod waited for resources
type ResourceWaitInfo struct {
	Type     string        `json:"type" yaml:"type"`
	Reason   string        `json:"reason" yaml:"reason"`
	Duration time.Duration `json:"duration" yaml:"duration"`
	Message  string        `json:"message" yaml:"message"`
}
