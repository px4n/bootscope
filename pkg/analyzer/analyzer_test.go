package analyzer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/types"
)

func TestAnalyzePod(t *testing.T) {
	tests := []struct {
		name           string
		podInfo        *collector.PodInfo
		expectedStatus string
		hasBottlenecks bool
		minPhases      int
	}{
		{
			name: "running pod with all phases",
			podInfo: &collector.PodInfo{
				Name:              "test-pod",
				Namespace:         "default",
				CreationTimestamp: time.Now().Add(-60 * time.Second),
				Phase:             "Running",
				NodeName:          "test-node",
				Conditions: []collector.PodCondition{
					{
						Type:               "PodScheduled",
						Status:             "True",
						LastTransitionTime: time.Now().Add(-59 * time.Second),
					},
					{
						Type:               "Ready",
						Status:             "True",
						LastTransitionTime: time.Now().Add(-30 * time.Second),
					},
				},
				Events: []collector.Event{
					{
						Type:      "Normal",
						Reason:    "Scheduled",
						Message:   "Successfully assigned pod to node",
						Timestamp: time.Now().Add(-59 * time.Second),
					},
					{
						Type:      "Normal",
						Reason:    "Pulling",
						Message:   "Pulling image \"nginx:latest\"",
						Timestamp: time.Now().Add(-55 * time.Second),
					},
					{
						Type:      "Normal",
						Reason:    "Pulled",
						Message:   "Successfully pulled image \"nginx:latest\"",
						Timestamp: time.Now().Add(-40 * time.Second),
					},
					{
						Type:      "Normal",
						Reason:    "Created",
						Message:   "Created container",
						Timestamp: time.Now().Add(-35 * time.Second),
					},
					{
						Type:      "Normal",
						Reason:    "Started",
						Message:   "Started container",
						Timestamp: time.Now().Add(-30 * time.Second),
					},
				},
				ContainerStatuses: []collector.ContainerStatus{
					{
						Name:      "nginx",
						State:     "Running",
						Ready:     true,
						StartedAt: &[]time.Time{time.Now().Add(-30 * time.Second)}[0],
					},
				},
			},
			expectedStatus: "Running",
			hasBottlenecks: false, // Total time calculation might be 0 in tests
			minPhases:      3,     // At least Scheduling, ImagePull, ContainerCreation
		},
		{
			name: "pod still starting",
			podInfo: &collector.PodInfo{
				Name:              "starting-pod",
				Namespace:         "default",
				CreationTimestamp: time.Now().Add(-10 * time.Second),
				Phase:             "Pending",
				NodeName:          "test-node",
				Conditions: []collector.PodCondition{
					{
						Type:               "PodScheduled",
						Status:             "True",
						LastTransitionTime: time.Now().Add(-9 * time.Second),
					},
				},
				Events: []collector.Event{
					{
						Type:      "Normal",
						Reason:    "Scheduled",
						Message:   "Successfully assigned pod to node",
						Timestamp: time.Now().Add(-9 * time.Second),
					},
					{
						Type:      "Normal",
						Reason:    "Pulling",
						Message:   "Pulling image",
						Timestamp: time.Now().Add(-8 * time.Second),
					},
				},
			},
			expectedStatus: "Pending",
			hasBottlenecks: false,
			minPhases:      1, // At least Scheduling
		},
		{
			name: "failed pod",
			podInfo: &collector.PodInfo{
				Name:              "failed-pod",
				Namespace:         "default",
				CreationTimestamp: time.Now().Add(-30 * time.Second),
				Phase:             "Failed",
				NodeName:          "test-node",
				Conditions: []collector.PodCondition{
					{
						Type:               "PodScheduled",
						Status:             "True",
						LastTransitionTime: time.Now().Add(-29 * time.Second),
					},
				},
				Events: []collector.Event{
					{
						Type:      "Normal",
						Reason:    "Scheduled",
						Message:   "Successfully assigned pod to node",
						Timestamp: time.Now().Add(-29 * time.Second),
					},
					{
						Type:      "Warning",
						Reason:    "Failed",
						Message:   "Error: ImagePullBackOff",
						Timestamp: time.Now().Add(-20 * time.Second),
					},
				},
			},
			expectedStatus: "Failed",
			hasBottlenecks: false,
			minPhases:      1, // At least Scheduling
		},
	}

	analyzer := NewAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := analyzer.AnalyzePod(tt.podInfo)

			require.NoError(t, err)
			require.NotNil(t, profile)
			assert.Equal(t, tt.podInfo.Name, profile.PodName)
			assert.Equal(t, tt.podInfo.Namespace, profile.Namespace)
			assert.Equal(t, tt.expectedStatus, profile.Status)
			assert.GreaterOrEqual(t, len(profile.Phases), tt.minPhases)

			if tt.hasBottlenecks {
				assert.NotEmpty(t, profile.Bottlenecks, "Expected bottlenecks to be identified")
			}
		})
	}
}

func TestDetectPhases(t *testing.T) {
	tests := []struct {
		name                 string
		podInfo              *collector.PodInfo
		expectedPhaseTypes   []types.StartupPhase
		unexpectedPhaseTypes []types.StartupPhase
	}{
		{
			name: "all phases detected",
			podInfo: &collector.PodInfo{
				CreationTimestamp: time.Now().Add(-61 * time.Second),
				NodeName:          "test-node",
				Events: []collector.Event{
					{Reason: "Scheduled", Timestamp: time.Now().Add(-60 * time.Second)},
					{Reason: "Pulling", Message: "Pulling image \"nginx:latest\"", Timestamp: time.Now().Add(-55 * time.Second)},
					{Reason: "Pulled", Message: "Successfully pulled image \"nginx:latest\"", Timestamp: time.Now().Add(-40 * time.Second)},
					{Reason: "Created", Timestamp: time.Now().Add(-35 * time.Second)},
					{Reason: "Started", Timestamp: time.Now().Add(-30 * time.Second)},
				},
				Conditions: []collector.PodCondition{
					{
						Type:               "Ready",
						Status:             "True",
						LastTransitionTime: time.Now(),
					},
				},
			},
			expectedPhaseTypes: []types.StartupPhase{
				types.StartupPhaseScheduling,
				types.StartupPhaseImagePull,
				types.StartupPhaseContainerCreate,
				types.StartupPhaseReady,
			},
			unexpectedPhaseTypes: []types.StartupPhase{
				types.StartupPhaseInitContainers, // No init containers in this test
			},
		},
		{
			name: "init containers",
			podInfo: &collector.PodInfo{
				CreationTimestamp: time.Now().Add(-61 * time.Second),
				NodeName:          "test-node",
				Events: []collector.Event{
					{Reason: "Scheduled", Timestamp: time.Now().Add(-60 * time.Second)},
					{Reason: "Pulling", Message: "Pulling image \"init:setup\"", Timestamp: time.Now().Add(-55 * time.Second)},
					{Reason: "Started", Message: "Started container init-setup", Timestamp: time.Now().Add(-45 * time.Second)},
					{Reason: "Started", Message: "Started container main", Timestamp: time.Now().Add(-30 * time.Second)},
				},
				InitContainers: []collector.ContainerStatus{
					{
						Name:       "init-setup",
						State:      "Terminated",
						Ready:      false,
						StartedAt:  &[]time.Time{time.Now().Add(-50 * time.Second)}[0],
						FinishedAt: &[]time.Time{time.Now().Add(-45 * time.Second)}[0],
					},
				},
				Conditions: []collector.PodCondition{
					{
						Type:               "Ready",
						Status:             "True",
						LastTransitionTime: time.Now(),
					},
				},
			},
			expectedPhaseTypes: []types.StartupPhase{
				types.StartupPhaseScheduling,
				types.StartupPhaseInitContainers,
				types.StartupPhaseReady,
			},
			unexpectedPhaseTypes: []types.StartupPhase{},
		},
		{
			name: "missing events handled gracefully",
			podInfo: &collector.PodInfo{
				CreationTimestamp: time.Now().Add(-31 * time.Second),
				NodeName:          "test-node",
				Events: []collector.Event{
					{Reason: "Scheduled", Timestamp: time.Now().Add(-30 * time.Second)},
					{Reason: "Started", Timestamp: time.Now().Add(-10 * time.Second)},
				},
				Conditions: []collector.PodCondition{
					{
						Type:               "Ready",
						Status:             "True",
						LastTransitionTime: time.Now().Add(-5 * time.Second),
					},
				},
			},
			expectedPhaseTypes: []types.StartupPhase{
				types.StartupPhaseScheduling,
				types.StartupPhaseReady,
			},
			unexpectedPhaseTypes: []types.StartupPhase{
				types.StartupPhaseImagePull,       // No pull events
				types.StartupPhaseContainerCreate, // No create event
			},
		},
	}

	analyzer := NewAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phases := analyzer.detectPhases(tt.podInfo)

			phaseTypes := make(map[types.StartupPhase]bool)
			for _, phase := range phases {
				phaseTypes[phase.Phase] = true
			}

			// Check expected phases are present
			for _, expectedPhase := range tt.expectedPhaseTypes {
				assert.True(t, phaseTypes[expectedPhase], "Expected phase %s not found", expectedPhase)
			}

			// Check unexpected phases are not present
			for _, unexpectedPhase := range tt.unexpectedPhaseTypes {
				assert.False(t, phaseTypes[unexpectedPhase], "Unexpected phase %s found", unexpectedPhase)
			}
		})
	}
}

func TestContainerStatusExtraction(t *testing.T) {
	tests := []struct {
		name     string
		podInfo  *collector.PodInfo
		expected []types.ContainerStatusInfo
	}{
		{
			name: "single container with restarts",
			podInfo: &collector.PodInfo{
				ContainerStatuses: []collector.ContainerStatus{
					{
						Name:         "app",
						RestartCount: 3,
						Ready:        false,
						State:        "Terminated",
						StateDetails: map[string]interface{}{
							"exitCode": float64(1),
							"reason":   "Error",
						},
					},
				},
			},
			expected: []types.ContainerStatusInfo{
				{
					Name:         "app",
					RestartCount: 3,
					Ready:        false,
					State:        "Terminated",
					ExitCode:     func() *int32 { v := int32(1); return &v }(),
					Reason:       "Error",
				},
			},
		},
		{
			name: "container in CrashLoopBackOff",
			podInfo: &collector.PodInfo{
				ContainerStatuses: []collector.ContainerStatus{
					{
						Name:         "crasher",
						RestartCount: 5,
						Ready:        false,
						State:        "Waiting",
						StateDetails: map[string]interface{}{
							"reason": "CrashLoopBackOff",
						},
					},
				},
			},
			expected: []types.ContainerStatusInfo{
				{
					Name:         "crasher",
					RestartCount: 5,
					Ready:        false,
					State:        "Waiting",
					Reason:       "CrashLoopBackOff",
				},
			},
		},
	}

	analyzer := NewAnalyzer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.extractContainerStatuses(tt.podInfo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPodStatusDetermination(t *testing.T) {
	tests := []struct {
		name           string
		podInfo        *collector.PodInfo
		expectedStatus string
	}{
		{
			name: "CrashLoopBackOff detection",
			podInfo: &collector.PodInfo{
				Phase: "Running",
				ContainerStatuses: []collector.ContainerStatus{
					{
						State: "Waiting",
						StateDetails: map[string]interface{}{
							"reason": "CrashLoopBackOff",
						},
					},
				},
			},
			expectedStatus: "CrashLoopBackOff",
		},
		{
			name: "Running with restarts",
			podInfo: &collector.PodInfo{
				Phase: "Running",
				ContainerStatuses: []collector.ContainerStatus{
					{
						State:        "Running",
						RestartCount: 2,
					},
				},
			},
			expectedStatus: "Running (Restarted)",
		},
		{
			name: "All containers terminated",
			podInfo: &collector.PodInfo{
				Phase: "Running",
				ContainerStatuses: []collector.ContainerStatus{
					{
						State: "Terminated",
					},
					{
						State: "Terminated",
					},
				},
			},
			expectedStatus: "Failed",
		},
		{
			name: "Normal running pod",
			podInfo: &collector.PodInfo{
				Phase: "Running",
				ContainerStatuses: []collector.ContainerStatus{
					{
						State:        "Running",
						RestartCount: 0,
						Ready:        true,
					},
				},
			},
			expectedStatus: "Running",
		},
	}

	analyzer := NewAnalyzer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.determinePodStatus(tt.podInfo)
			assert.Equal(t, tt.expectedStatus, result)
		})
	}
}

func TestRestartDetection(t *testing.T) {
	tests := []struct {
		name               string
		containerStatuses  []collector.ContainerStatus
		phases             []types.PhaseInfo
		expectedPhaseCount int
	}{
		{
			name: "no restarts",
			containerStatuses: []collector.ContainerStatus{
				{Name: "main", RestartCount: 0},
			},
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseScheduling, StartTime: time.Now(), EndTime: time.Now().Add(1 * time.Second)},
				{Phase: types.StartupPhaseImagePull, StartTime: time.Now().Add(1 * time.Second), EndTime: time.Now().Add(10 * time.Second)},
				{Phase: types.StartupPhaseContainerCreate, StartTime: time.Now().Add(10 * time.Second), EndTime: time.Now().Add(11 * time.Second)},
			},
			expectedPhaseCount: 3,
		},
		{
			name: "container restarted once",
			containerStatuses: []collector.ContainerStatus{
				{Name: "main", RestartCount: 1},
			},
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseScheduling, StartTime: time.Now(), EndTime: time.Now().Add(1 * time.Second)},
				{Phase: types.StartupPhaseImagePull, StartTime: time.Now().Add(1 * time.Second), EndTime: time.Now().Add(10 * time.Second)},
				{Phase: types.StartupPhaseContainerCreate, StartTime: time.Now().Add(10 * time.Second), EndTime: time.Now().Add(11 * time.Second)},
				{Phase: types.StartupPhaseAppStart, StartTime: time.Now().Add(11 * time.Second), EndTime: time.Now().Add(15 * time.Second)},
				// Restart happens here
				{Phase: types.StartupPhaseContainerCreate, StartTime: time.Now().Add(20 * time.Second), EndTime: time.Now().Add(21 * time.Second)},
				{Phase: types.StartupPhaseAppStart, StartTime: time.Now().Add(21 * time.Second), EndTime: time.Now().Add(25 * time.Second)},
			},
			expectedPhaseCount: 2, // After consolidation: only phases after last restart (container create, app start)
		},
	}

	analyzer := NewAnalyzer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test consolidateDuplicatePhases
			consolidated := analyzer.consolidateDuplicatePhases(tt.phases)
			assert.Len(t, consolidated, tt.expectedPhaseCount, "Phase count after consolidation")

			// Verify we keep the most recent phases after restart
			if tt.containerStatuses[0].RestartCount > 0 {
				// Should have only one ContainerCreate phase (the latest)
				containerCreateCount := 0
				for _, phase := range consolidated {
					if phase.Phase == types.StartupPhaseContainerCreate {
						containerCreateCount++
					}
				}
				assert.Equal(t, 1, containerCreateCount, "Should have exactly one ContainerCreate phase after consolidation")
			}
		})
	}
}

func TestIdentifyBottlenecks(t *testing.T) {
	tests := []struct {
		name               string
		phases             []types.PhaseInfo
		totalTime          time.Duration
		expectedBottleneck types.StartupPhase
		noBottleneck       bool
	}{
		{
			name: "image pull bottleneck",
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseScheduling, Duration: 1 * time.Second},
				{Phase: types.StartupPhaseImagePull, Duration: 45 * time.Second},
				{Phase: types.StartupPhaseContainerCreate, Duration: 2 * time.Second},
				{Phase: types.StartupPhaseAppStart, Duration: 2 * time.Second},
			},
			totalTime:          50 * time.Second,
			expectedBottleneck: types.StartupPhaseImagePull,
		},
		{
			name: "init containers bottleneck",
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseScheduling, Duration: 1 * time.Second},
				{Phase: types.StartupPhaseImagePull, Duration: 5 * time.Second},
				{Phase: types.StartupPhaseInitContainers, Duration: 60 * time.Second},
				{Phase: types.StartupPhaseAppStart, Duration: 4 * time.Second},
			},
			totalTime:          70 * time.Second,
			expectedBottleneck: types.StartupPhaseInitContainers,
		},
		{
			name: "no significant bottleneck",
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseScheduling, Duration: 5 * time.Second},
				{Phase: types.StartupPhaseImagePull, Duration: 5 * time.Second},
				{Phase: types.StartupPhaseContainerCreate, Duration: 5 * time.Second},
				{Phase: types.StartupPhaseAppStart, Duration: 5 * time.Second},
			},
			totalTime:    20 * time.Second,
			noBottleneck: true,
		},
		{
			name: "zero total time",
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseScheduling, Duration: 0},
			},
			totalTime:    0,
			noBottleneck: true,
		},
		{
			name: "single phase fast startup no bottleneck",
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseImagePull, Duration: 5 * time.Second},
			},
			totalTime:    5 * time.Second,
			noBottleneck: true, // Should not show bottleneck for fast single-phase startup
		},
		{
			name: "single phase slow startup shows bottleneck",
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseImagePull, Duration: 25 * time.Second},
			},
			totalTime:          25 * time.Second,
			expectedBottleneck: types.StartupPhaseImagePull, // Should show bottleneck for slow single-phase startup
		},
	}

	analyzer := NewAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bottlenecks := analyzer.identifyBottlenecks(tt.phases, tt.totalTime)

			if tt.noBottleneck {
				assert.Empty(t, bottlenecks)
			} else {
				require.NotEmpty(t, bottlenecks)
				found := false
				for _, bottleneck := range bottlenecks {
					if bottleneck.Phase == tt.expectedBottleneck {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected bottleneck in phase %s not found", tt.expectedBottleneck)
			}
		})
	}
}

// TestGenerateRecommendationsBasicData tests recommendation generation using only basic pod data
// without ImageMetadata or ResourceWaiting information. This ensures the system can still
// provide useful recommendations even with minimal data available.
func TestGenerateRecommendationsBasicData(t *testing.T) {
	tests := []struct {
		name                   string
		podInfo                *collector.PodInfo
		profile                *types.PodStartupProfile
		expectedRecommendation string
		noRecommendations      bool
	}{
		{
			name: "image pull recommendation",
			podInfo: &collector.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
				Images:    []string{"nginx:latest"},
			},
			profile: &types.PodStartupProfile{
				Phases: []types.PhaseInfo{
					{
						Phase:    types.StartupPhaseImagePull,
						Duration: 60 * time.Second,
						Details:  "Pulled image nginx:latest",
					},
				},
				Bottlenecks: []types.Bottleneck{
					{
						Phase:      types.StartupPhaseImagePull,
						Duration:   60 * time.Second,
						Percentage: 75,
					},
				},
			},
			expectedRecommendation: "Optimize Image Pull Time",
		},
		{
			name: "init container recommendation",
			podInfo: &collector.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
			},
			profile: &types.PodStartupProfile{
				Phases: []types.PhaseInfo{
					{
						Phase:    types.StartupPhaseInitContainers,
						Duration: 45 * time.Second,
						Details:  "slow-init-container",
					},
				},
				Bottlenecks: []types.Bottleneck{
					{
						Phase:       types.StartupPhaseInitContainers,
						Duration:    45 * time.Second,
						Percentage:  60,
						Description: "slow-init-container",
					},
				},
			},
			expectedRecommendation: "Optimize Init Container Logic",
		},
		{
			name: "application startup recommendation",
			podInfo: &collector.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
			},
			profile: &types.PodStartupProfile{
				Phases: []types.PhaseInfo{
					{
						Phase:    types.StartupPhaseAppStart,
						Duration: 45 * time.Second,
					},
				},
				Bottlenecks: []types.Bottleneck{
					{
						Phase:      types.StartupPhaseAppStart,
						Duration:   45 * time.Second,
						Percentage: 50,
					},
				},
			},
			expectedRecommendation: "Optimize Application Startup",
		},
		{
			name: "no bottlenecks no recommendations",
			podInfo: &collector.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
			},
			profile: &types.PodStartupProfile{
				Bottlenecks: []types.Bottleneck{},
			},
			noRecommendations: true,
		},
	}

	analyzer := NewAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recommendations := analyzer.generateRecommendations(tt.podInfo, tt.profile)

			if tt.noRecommendations {
				assert.Empty(t, recommendations)
			} else {
				require.NotEmpty(t, recommendations)
				found := false
				for _, rec := range recommendations {
					if rec.Title == tt.expectedRecommendation {
						found = true
						assert.NotEmpty(t, rec.Description)
						assert.NotEmpty(t, rec.Impact)
						break
					}
				}
				assert.True(t, found, "Expected recommendation '%s' not found", tt.expectedRecommendation)
			}
		})
	}
}

// TestGenerateRecommendationsWithMetadata tests recommendation generation with rich metadata
// including ImageMetadata (size, registry, pull time) and ResourceWaiting information.
// This tests recommendation when detailed data is available.
func TestGenerateRecommendationsWithMetadata(t *testing.T) {
	tests := []struct {
		name                    string
		podInfo                 *collector.PodInfo
		profile                 *types.PodStartupProfile
		expectedRecommendations []string
	}{
		{
			name: "large image detection",
			podInfo: &collector.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
				ImageMetadata: []types.ImageInfo{
					{
						Name:     "nginx:latest",
						Size:     600 * 1024 * 1024, // 600MB
						Registry: "docker.io",
						IsLocal:  false,
						PullTime: 45 * time.Second,
					},
				},
			},
			profile: &types.PodStartupProfile{
				ImageMetadata: []types.ImageInfo{
					{
						Name:     "nginx:latest",
						Size:     800 * 1024 * 1024, // 800MB
						Registry: "docker.io",
						IsLocal:  false,
						PullTime: 45 * time.Second,
					},
				},
				Phases: []types.PhaseInfo{
					{
						Phase:    types.StartupPhaseImagePull,
						Duration: 45 * time.Second,
						Details:  "Pulled image nginx:latest",
					},
				},
				Bottlenecks: []types.Bottleneck{
					{
						Phase:      types.StartupPhaseImagePull,
						Duration:   45 * time.Second,
						Percentage: 60,
					},
				},
			},
			expectedRecommendations: []string{
				"Large Image Detected",
				"Use Local Registry",
			},
		},
		{
			name: "local registry - no remote recommendation",
			podInfo: &collector.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
				ImageMetadata: []types.ImageInfo{
					{
						Name:     "localhost:5000/myapp:v1",
						Size:     100 * 1024 * 1024, // 100MB
						Registry: "localhost:5000",
						IsLocal:  true,
						PullTime: 2 * time.Second,
					},
				},
			},
			profile: &types.PodStartupProfile{
				Phases: []types.PhaseInfo{
					{
						Phase:    types.StartupPhaseImagePull,
						Duration: 2 * time.Second,
					},
				},
			},
			expectedRecommendations: []string{},
		},
		{
			name: "resource waiting detection",
			podInfo: &collector.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
				ResourceWaiting: &types.ResourceWaitInfo{
					Type:     "CPU",
					Reason:   "Insufficient CPU",
					Duration: 2 * time.Minute,
					Message:  "0/3 nodes available: InsufficientCPU",
				},
			},
			profile: &types.PodStartupProfile{
				ResourceWaiting: &types.ResourceWaitInfo{
					Type:     "CPU",
					Reason:   "Insufficient CPU",
					Duration: 2 * time.Minute,
					Message:  "0/3 nodes available: InsufficientCPU",
				},
			},
			expectedRecommendations: []string{
				"Insufficient CPU Resources",
			},
		},
		{
			name: "node selector constraint",
			podInfo: &collector.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
				ResourceWaiting: &types.ResourceWaitInfo{
					Type:     "NodeSelector",
					Reason:   "NodeSelectorMismatch",
					Duration: 30 * time.Second,
					Message:  "0/5 nodes available: NodeSelectorMismatch",
				},
			},
			profile: &types.PodStartupProfile{
				ResourceWaiting: &types.ResourceWaitInfo{
					Type:     "NodeSelector",
					Reason:   "NodeSelectorMismatch",
					Duration: 30 * time.Second,
					Message:  "0/5 nodes available: NodeSelectorMismatch",
				},
			},
			expectedRecommendations: []string{
				"Scheduling Constraints Too Restrictive",
			},
		},
		{
			name: "multiple issues",
			podInfo: &collector.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
				ImageMetadata: []types.ImageInfo{
					{
						Name:     "gcr.io/project/large-app:latest",
						Size:     800 * 1024 * 1024, // 800MB
						Registry: "gcr.io",
						IsLocal:  false,
						PullTime: 60 * time.Second,
					},
				},
				ResourceWaiting: &types.ResourceWaitInfo{
					Type:     "Memory",
					Reason:   "InsufficientMemory",
					Duration: 45 * time.Second,
					Message:  "0/3 nodes available: InsufficientMemory",
				},
			},
			profile: &types.PodStartupProfile{
				ImageMetadata: []types.ImageInfo{
					{
						Name:     "gcr.io/project/large-app:latest",
						Size:     800 * 1024 * 1024, // 800MB
						Registry: "gcr.io",
						IsLocal:  false,
						PullTime: 60 * time.Second,
					},
				},
				ResourceWaiting: &types.ResourceWaitInfo{
					Type:     "Memory",
					Reason:   "InsufficientMemory",
					Duration: 45 * time.Second,
					Message:  "0/3 nodes available: InsufficientMemory",
				},
				Phases: []types.PhaseInfo{
					{
						Phase:    types.StartupPhaseImagePull,
						Duration: 60 * time.Second,
						Details:  "Pulled image gcr.io/project/large-app:latest",
					},
				},
				Bottlenecks: []types.Bottleneck{
					{
						Phase:      types.StartupPhaseImagePull,
						Duration:   60 * time.Second,
						Percentage: 50,
					},
				},
			},
			expectedRecommendations: []string{
				"Large Image Detected",
				"Use Local Registry",
				"Insufficient Memory Resources",
			},
		},
	}

	analyzer := NewAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recommendations := analyzer.generateRecommendations(tt.podInfo, tt.profile)

			// Check that we got the expected number of recommendations
			assert.GreaterOrEqual(t, len(recommendations), len(tt.expectedRecommendations),
				"Expected at least %d recommendations, got %d",
				len(tt.expectedRecommendations), len(recommendations))

			// Check that all expected recommendations are present
			recTitles := make(map[string]bool)
			for _, rec := range recommendations {
				recTitles[rec.Title] = true
			}

			for _, expectedTitle := range tt.expectedRecommendations {
				assert.True(t, recTitles[expectedTitle],
					"Expected recommendation '%s' not found", expectedTitle)
			}

			// Verify priority ordering
			if len(recommendations) > 1 {
				for i := 1; i < len(recommendations); i++ {
					assert.LessOrEqual(t, recommendations[i-1].Priority, recommendations[i].Priority,
						"Recommendations should be sorted by priority")
				}
			}
		})
	}
}

func TestFindReadyTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		podInfo      *collector.PodInfo
		expectReady  bool
		expectedTime time.Time
	}{
		{
			name: "ready from condition",
			podInfo: &collector.PodInfo{
				Conditions: []collector.PodCondition{
					{
						Type:               "Ready",
						Status:             "True",
						LastTransitionTime: now.Add(-10 * time.Second),
					},
				},
			},
			expectReady:  true,
			expectedTime: now.Add(-10 * time.Second),
		},
		{
			name: "not ready",
			podInfo: &collector.PodInfo{
				Conditions: []collector.PodCondition{
					{
						Type:               "Ready",
						Status:             "False",
						LastTransitionTime: now.Add(-10 * time.Second),
					},
				},
			},
			expectReady: false,
		},
		{
			name: "no ready condition",
			podInfo: &collector.PodInfo{
				Conditions: []collector.PodCondition{
					{
						Type:               "PodScheduled",
						Status:             "True",
						LastTransitionTime: now.Add(-10 * time.Second),
					},
				},
			},
			expectReady: false,
		},
	}

	analyzer := NewAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readyTime := analyzer.findReadyTime(tt.podInfo)

			if tt.expectReady {
				assert.False(t, readyTime.IsZero())
				assert.Equal(t, tt.expectedTime.Unix(), readyTime.Unix())
			} else {
				assert.True(t, readyTime.IsZero())
			}
		})
	}
}
