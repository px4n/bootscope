package collector

import (
	"time"

	"github.com/px4n/bootscope/pkg/types"
)

// ============================================================================
// Primary Data Structures
// ============================================================================

// PodInfo contains raw pod information collected from Kubernetes.
// This is the main data structure used to pass collected data to the analyzer.
type PodInfo struct {
	// Basic pod information
	Name              string    `json:"name" yaml:"name"`
	Namespace         string    `json:"namespace" yaml:"namespace"`
	UID               string    `json:"uid" yaml:"uid"`
	ResourceVersion   string    `json:"resourceVersion" yaml:"resourceVersion"`
	CreationTimestamp time.Time `json:"creationTimestamp" yaml:"creationTimestamp"`
	Phase             string    `json:"phase" yaml:"phase"`
	NodeName          string    `json:"nodeName" yaml:"nodeName"`

	// Pod conditions and status
	Conditions        []PodCondition    `json:"conditions" yaml:"conditions"`
	ContainerStatuses []ContainerStatus `json:"containerStatuses" yaml:"containerStatuses"`
	InitContainers    []ContainerStatus `json:"initContainers" yaml:"initContainers"`

	// Events and images
	Events []Event  `json:"events" yaml:"events"`
	Images []string `json:"images" yaml:"images"`

	// These are populated by the collector from analysis of events and pod state
	ImageMetadata   []types.ImageInfo       `json:"imageMetadata,omitempty" yaml:"imageMetadata,omitempty"`
	ResourceWaiting *types.ResourceWaitInfo `json:"resourceWaiting,omitempty" yaml:"resourceWaiting,omitempty"`
}

// ============================================================================
// Kubernetes State Types
// ============================================================================

// PodCondition represents a pod condition.
// This is an internal type used during collection.
type PodCondition struct {
	Type               string    `json:"type" yaml:"type"`
	Status             string    `json:"status" yaml:"status"`
	LastTransitionTime time.Time `json:"lastTransitionTime" yaml:"lastTransitionTime"`
	Reason             string    `json:"reason,omitempty" yaml:"reason,omitempty"`
	Message            string    `json:"message,omitempty" yaml:"message,omitempty"`
}

// ContainerStatus represents container state information.
// This is an internal type used during collection.
type ContainerStatus struct {
	Name         string                 `json:"name" yaml:"name"`
	Ready        bool                   `json:"ready" yaml:"ready"`
	RestartCount int32                  `json:"restartCount" yaml:"restartCount"`
	State        string                 `json:"state" yaml:"state"`
	StateDetails map[string]interface{} `json:"stateDetails,omitempty" yaml:"stateDetails,omitempty"`
	StartedAt    *time.Time             `json:"startedAt,omitempty" yaml:"startedAt,omitempty"`
	FinishedAt   *time.Time             `json:"finishedAt,omitempty" yaml:"finishedAt,omitempty"`
}

// Event represents a Kubernetes event.
// This is an internal type used during data collection.
type Event struct {
	Type      string    `json:"type" yaml:"type"`
	Reason    string    `json:"reason" yaml:"reason"`
	Message   string    `json:"message" yaml:"message"`
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
	Object    string    `json:"object" yaml:"object"`
}
