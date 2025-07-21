package testutil

import (
	"time"

	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/types"
)

// ProfileOption is a functional option for creating test profiles
type ProfileOption func(*types.PodStartupProfile)

func NewTestProfile(opts ...ProfileOption) *types.PodStartupProfile {
	profile := &types.PodStartupProfile{
		PodName:   "test-pod",
		Namespace: "default",
		Status:    "Running",
		StartTime: time.Now().Add(-45 * time.Second),
		ReadyTime: time.Now(),
		TotalTime: 45 * time.Second,
		Phases: []types.PhaseInfo{
			{
				Phase:    types.StartupPhaseScheduling,
				Duration: 2 * time.Second,
				Details:  "Scheduled to node worker-1",
			},
		},
	}

	for _, opt := range opts {
		opt(profile)
	}

	return profile
}

func WithPhases(phases ...types.PhaseInfo) ProfileOption {
	return func(p *types.PodStartupProfile) {
		p.Phases = phases
	}
}

func WithStatus(status string) ProfileOption {
	return func(p *types.PodStartupProfile) {
		p.Status = status
	}
}

// WithBottlenecks adds bottlenecks to a test profile
func WithBottlenecks(bottlenecks ...types.Bottleneck) ProfileOption {
	return func(p *types.PodStartupProfile) {
		p.Bottlenecks = bottlenecks
	}
}

// WithRecommendations adds recommendations to a test profile
func WithRecommendations(recommendations ...types.Recommendation) ProfileOption {
	return func(p *types.PodStartupProfile) {
		p.Recommendations = recommendations
	}
}

// WithContainerStatuses adds container statuses to a test profile
func WithContainerStatuses(statuses ...types.ContainerStatusInfo) ProfileOption {
	return func(p *types.PodStartupProfile) {
		p.ContainerStatuses = statuses
	}
}

func WithTotalTime(d time.Duration) ProfileOption {
	return func(p *types.PodStartupProfile) {
		p.TotalTime = d
	}
}

func NewTestPodInfo() *collector.PodInfo {
	now := time.Now()
	return &collector.PodInfo{
		Name:              "test-pod",
		Namespace:         "default",
		UID:               "abc123",
		ResourceVersion:   "12345",
		CreationTimestamp: now.Add(-60 * time.Second),
		Phase:             "Running",
		NodeName:          "worker-1",
		Conditions: []collector.PodCondition{
			{
				Type:               "PodScheduled",
				Status:             "True",
				LastTransitionTime: now.Add(-58 * time.Second),
			},
			{
				Type:               "Ready",
				Status:             "True",
				LastTransitionTime: now.Add(-30 * time.Second),
			},
		},
		Events: []collector.Event{
			{
				Type:      "Normal",
				Reason:    "Scheduled",
				Message:   "Successfully assigned default/test-pod to worker-1",
				Timestamp: now.Add(-58 * time.Second),
			},
		},
		ContainerStatuses: []collector.ContainerStatus{
			{
				Name:         "main",
				Ready:        true,
				RestartCount: 0,
				State:        "Running",
				StartedAt:    &[]time.Time{now.Add(-30 * time.Second)}[0],
			},
		},
		Images: []string{"nginx:latest"},
	}
}

func NewTestConfig() *config.Config {
	return config.DefaultConfig()
}

// Common test phases
var (
	SchedulingPhase = types.PhaseInfo{
		Phase:     types.StartupPhaseScheduling,
		StartTime: time.Now().Add(-60 * time.Second),
		EndTime:   time.Now().Add(-58 * time.Second),
		Duration:  2 * time.Second,
		Details:   "Scheduled to node worker-1",
	}

	ImagePullPhase = types.PhaseInfo{
		Phase:     types.StartupPhaseImagePull,
		StartTime: time.Now().Add(-58 * time.Second),
		EndTime:   time.Now().Add(-28 * time.Second),
		Duration:  30 * time.Second,
		Details:   "Pulled image nginx:latest",
	}

	ContainerCreatePhase = types.PhaseInfo{
		Phase:     types.StartupPhaseContainerCreate,
		StartTime: time.Now().Add(-28 * time.Second),
		EndTime:   time.Now().Add(-25 * time.Second),
		Duration:  3 * time.Second,
		Details:   "Created container main",
	}

	AppStartPhase = types.PhaseInfo{
		Phase:     types.StartupPhaseAppStart,
		StartTime: time.Now().Add(-25 * time.Second),
		EndTime:   time.Now(),
		Duration:  25 * time.Second,
		Details:   "Application starting",
	}

	InitContainerPhase = types.PhaseInfo{
		Phase:     types.StartupPhaseInitContainers,
		StartTime: time.Now().Add(-50 * time.Second),
		EndTime:   time.Now().Add(-30 * time.Second),
		Duration:  20 * time.Second,
		Details:   "init-db",
	}
)
