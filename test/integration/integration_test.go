//go:build integration
// +build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/px4n/bootscope/pkg/analyzer"
	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/config"
	"github.com/px4n/bootscope/pkg/types"
)

// TestFullAnalysisPipeline tests the complete flow from pod collection to analysis
func TestFullAnalysisPipeline(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func() (*fake.Clientset, *v1.Pod, *v1.EventList)
		validateFunc func(t *testing.T, profile *types.PodStartupProfile)
	}{
		{
			name: "successful pod startup with all phases",
			setupFunc: func() (*fake.Clientset, *v1.Pod, *v1.EventList) {
				now := time.Now()
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-pod",
						Namespace:         "default",
						CreationTimestamp: metav1.Time{Time: now.Add(-60 * time.Second)},
					},
					Spec: v1.PodSpec{
						NodeName: "worker-1",
						InitContainers: []v1.Container{
							{Name: "init-db", Image: "postgres:init"},
						},
						Containers: []v1.Container{
							{Name: "main", Image: "nginx:latest"},
						},
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
						Conditions: []v1.PodCondition{
							{
								Type:               v1.PodScheduled,
								Status:             v1.ConditionTrue,
								LastTransitionTime: metav1.Time{Time: now.Add(-59 * time.Second)},
							},
							{
								Type:               v1.PodInitialized,
								Status:             v1.ConditionTrue,
								LastTransitionTime: metav1.Time{Time: now.Add(-20 * time.Second)},
							},
							{
								Type:               v1.ContainersReady,
								Status:             v1.ConditionTrue,
								LastTransitionTime: metav1.Time{Time: now},
							},
							{
								Type:               v1.PodReady,
								Status:             v1.ConditionTrue,
								LastTransitionTime: metav1.Time{Time: now},
							},
						},
						InitContainerStatuses: []v1.ContainerStatus{
							{
								Name: "init-db",
								State: v1.ContainerState{
									Terminated: &v1.ContainerStateTerminated{
										StartedAt:  metav1.Time{Time: now.Add(-40 * time.Second)},
										FinishedAt: metav1.Time{Time: now.Add(-20 * time.Second)},
									},
								},
							},
						},
						ContainerStatuses: []v1.ContainerStatus{
							{
								Name:  "main",
								Ready: true,
								State: v1.ContainerState{
									Running: &v1.ContainerStateRunning{
										StartedAt: metav1.Time{Time: now.Add(-15 * time.Second)},
									},
								},
							},
						},
					},
				}

				events := &v1.EventList{
					Items: []v1.Event{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "event-1",
							},
							InvolvedObject: v1.ObjectReference{
								Name:      "test-pod",
								Namespace: "default",
							},
							Reason:         "Scheduled",
							Message:        "Successfully assigned default/test-pod to worker-1",
							FirstTimestamp: metav1.Time{Time: now.Add(-59 * time.Second)},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "event-2",
							},
							InvolvedObject: v1.ObjectReference{
								Name:      "test-pod",
								Namespace: "default",
							},
							Reason:         "Pulling",
							Message:        "Pulling image \"nginx:latest\"",
							FirstTimestamp: metav1.Time{Time: now.Add(-55 * time.Second)},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "event-3",
							},
							InvolvedObject: v1.ObjectReference{
								Name:      "test-pod",
								Namespace: "default",
							},
							Reason:         "Pulled",
							Message:        "Successfully pulled image \"nginx:latest\" in 15s",
							FirstTimestamp: metav1.Time{Time: now.Add(-40 * time.Second)},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "event-4",
							},
							InvolvedObject: v1.ObjectReference{
								Name:      "test-pod",
								Namespace: "default",
							},
							Reason:         "Created",
							Message:        "Created container main",
							FirstTimestamp: metav1.Time{Time: now.Add(-17 * time.Second)},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "event-5",
							},
							InvolvedObject: v1.ObjectReference{
								Name:      "test-pod",
								Namespace: "default",
							},
							Reason:         "Started",
							Message:        "Started container main",
							FirstTimestamp: metav1.Time{Time: now.Add(-15 * time.Second)},
						},
					},
				}

				client := fake.NewSimpleClientset(pod)
				return client, pod, events
			},
			validateFunc: func(t *testing.T, profile *types.PodStartupProfile) {
				require.NotNil(t, profile)
				assert.Equal(t, "test-pod", profile.PodName)
				assert.Equal(t, "default", profile.Namespace)
				assert.Equal(t, "Running", profile.Status)

				// Should have detected all phases
				phaseMap := make(map[types.StartupPhase]bool)
				for _, phase := range profile.Phases {
					phaseMap[phase.Phase] = true
				}

				assert.True(t, phaseMap[types.StartupPhaseScheduling], "Should detect scheduling phase")
				assert.True(t, phaseMap[types.StartupPhaseImagePull], "Should detect image pull phase")
				assert.True(t, phaseMap[types.StartupPhaseContainerCreate], "Should detect container creation")
				assert.True(t, phaseMap[types.StartupPhaseInitContainers], "Should detect init containers")
				assert.True(t, phaseMap[types.StartupPhaseAppStart], "Should detect app start")

				// Total time should be ~60s
				assert.InDelta(t, 60*time.Second, profile.TotalTime, float64(5*time.Second))

				// Should identify image pull as a bottleneck (15s out of 60s = 25%)
				hasImagePullBottleneck := false
				for _, bottleneck := range profile.Bottlenecks {
					if bottleneck.Phase == types.StartupPhaseImagePull {
						hasImagePullBottleneck = true
					}
				}
				assert.True(t, hasImagePullBottleneck, "Should identify image pull as bottleneck")

				// Should have recommendations
				assert.NotEmpty(t, profile.Recommendations)
			},
		},
		{
			name: "pod stuck in image pull",
			setupFunc: func() (*fake.Clientset, *v1.Pod, *v1.EventList) {
				now := time.Now()
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "stuck-pod",
						Namespace:         "default",
						CreationTimestamp: metav1.Time{Time: now.Add(-300 * time.Second)}, // 5 minutes ago
					},
					Spec: v1.PodSpec{
						NodeName: "worker-2",
						Containers: []v1.Container{
							{Name: "app", Image: "company/large-app:v2"},
						},
					},
					Status: v1.PodStatus{
						Phase: v1.PodPending,
						Conditions: []v1.PodCondition{
							{
								Type:               v1.PodScheduled,
								Status:             v1.ConditionTrue,
								LastTransitionTime: metav1.Time{Time: now.Add(-295 * time.Second)},
							},
						},
						ContainerStatuses: []v1.ContainerStatus{
							{
								Name: "app",
								State: v1.ContainerState{
									Waiting: &v1.ContainerStateWaiting{
										Reason:  "ImagePullBackOff",
										Message: "Back-off pulling image",
									},
								},
							},
						},
					},
				}

				events := &v1.EventList{
					Items: []v1.Event{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "event-1",
							},
							InvolvedObject: v1.ObjectReference{
								Name:      "stuck-pod",
								Namespace: "default",
							},
							Reason:         "Scheduled",
							Message:        "Successfully assigned default/stuck-pod to worker-2",
							FirstTimestamp: metav1.Time{Time: now.Add(-295 * time.Second)},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "event-2",
							},
							InvolvedObject: v1.ObjectReference{
								Name:      "stuck-pod",
								Namespace: "default",
							},
							Reason:         "Pulling",
							Message:        "Pulling image \"company/large-app:v2\"",
							FirstTimestamp: metav1.Time{Time: now.Add(-290 * time.Second)},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "event-3",
							},
							InvolvedObject: v1.ObjectReference{
								Name:      "stuck-pod",
								Namespace: "default",
							},
							Reason:         "Failed",
							Message:        "Failed to pull image: timeout",
							FirstTimestamp: metav1.Time{Time: now.Add(-60 * time.Second)},
						},
					},
				}

				client := fake.NewSimpleClientset(pod)
				return client, pod, events
			},
			validateFunc: func(t *testing.T, profile *types.PodStartupProfile) {
				require.NotNil(t, profile)
				assert.Equal(t, "Pending", profile.Status)

				// Should show it's stuck in image pull
				assert.True(t, profile.TotalTime > 4*time.Minute)

				// Should have critical bottleneck
				hasCriticalBottleneck := false
				for _, bottleneck := range profile.Bottlenecks {
					if bottleneck.Severity == "critical" {
						hasCriticalBottleneck = true
					}
				}
				assert.True(t, hasCriticalBottleneck)

				// Should recommend optimizing image pull
				hasImagePullRec := false
				for _, rec := range profile.Recommendations {
					if strings.Contains(rec.Title, "Image Pull") || strings.Contains(rec.Description, "Image pull") ||
						strings.Contains(rec.Description, "registry") {
						hasImagePullRec = true
					}
				}
				assert.True(t, hasImagePullRec)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			client, pod, events := tt.setupFunc()

			// Mock events response
			client.PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, events, nil
			})

			// Create collector and analyzer
			cfg := config.DefaultConfig()
			collectorConfig := &collector.Config{
				WatchPollInterval:   cfg.GetWatchPollInterval(),
				NetworkSpeedAverage: cfg.GetNetworkSpeedAverage(),
				NetworkSpeedFast:    cfg.GetNetworkSpeedFast(),
				FastPullThreshold:   cfg.GetFastPullThreshold(),
			}

			coll := collector.NewCollectorWithConfig(client, collectorConfig)
			anal := analyzer.NewAnalyzerWithConfig(cfg)

			// Collect pod info
			ctx := context.Background()
			podInfo, err := coll.CollectPodInfo(ctx, pod.Namespace, pod.Name)
			require.NoError(t, err)

			// Analyze
			profile, err := anal.AnalyzePod(podInfo)
			require.NoError(t, err)

			// Debug: print what we got
			t.Logf("Pod info events: %d", len(podInfo.Events))
			for _, event := range podInfo.Events {
				t.Logf("  Event: %s, Message: %q at %s", event.Reason, event.Message, event.Timestamp)
			}
			t.Logf("Init containers: %d", len(podInfo.InitContainers))
			t.Logf("Pod conditions: %d", len(podInfo.Conditions))
			for _, cond := range podInfo.Conditions {
				t.Logf("  Condition: %s = %s at %s", cond.Type, cond.Status, cond.LastTransitionTime)
			}
			t.Logf("Profile phases: %d", len(profile.Phases))
			for _, phase := range profile.Phases {
				t.Logf("  Phase: %s, Duration: %s, Details: %s", phase.Phase, phase.Duration, phase.Details)
			}

			// Validate
			tt.validateFunc(t, profile)
		})
	}
}
