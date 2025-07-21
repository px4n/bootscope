package collector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestCollectPodInfo(t *testing.T) {
	tests := []struct {
		name          string
		podName       string
		namespace     string
		pod           *corev1.Pod
		events        *corev1.EventList
		expectError   bool
		expectedFound bool
	}{
		{
			name:      "successful collection",
			podName:   "test-pod",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					UID:       "test-uid",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			events: &corev1.EventList{
				Items: []corev1.Event{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "event1",
						},
						InvolvedObject: corev1.ObjectReference{
							UID: "test-uid",
						},
						Type:    "Normal",
						Reason:  "Scheduled",
						Message: "Successfully assigned",
					},
				},
			},
			expectError:   false,
			expectedFound: true,
		},
		{
			name:          "pod not found",
			podName:       "missing-pod",
			namespace:     "default",
			pod:           nil,
			events:        &corev1.EventList{},
			expectError:   true,
			expectedFound: false,
		},
		{
			name:      "different namespace",
			podName:   "test-pod",
			namespace: "other-namespace",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			events:        &corev1.EventList{},
			expectError:   true,
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			var objects []runtime.Object
			if tt.pod != nil {
				objects = append(objects, tt.pod)
			}
			client := fake.NewSimpleClientset(objects...)

			// Mock events
			if tt.events != nil {
				client.PrependReactor("list", "events", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, tt.events, nil
				})
			}

			collector := NewCollectorWithConfig(client, DefaultCollectorConfig())
			ctx := context.Background()

			podInfo, err := collector.CollectPodInfo(ctx, tt.namespace, tt.podName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, podInfo)
				assert.Equal(t, tt.podName, podInfo.Name)
				assert.Equal(t, tt.namespace, podInfo.Namespace)
				// Verify UID tracking for restart detection
				if tt.pod != nil && tt.pod.UID != "" {
					assert.Equal(t, string(tt.pod.UID), podInfo.UID)
				}
				if tt.events != nil {
					assert.Len(t, podInfo.Events, len(tt.events.Items))
				}
			}
		})
	}
}

func TestCollectPodEvents(t *testing.T) {
	now := time.Now()
	eventTime := metav1.MicroTime{Time: now.Add(-30 * time.Second)}

	tests := []struct {
		name           string
		namespace      string
		podName        string
		events         *corev1.EventList
		expectedCount  int
		expectedReason string
	}{
		{
			name:      "events with EventTime",
			namespace: "default",
			podName:   "test-pod",
			events: &corev1.EventList{
				Items: []corev1.Event{
					{
						EventTime: eventTime,
						Type:      "Normal",
						Reason:    "Scheduled",
						Message:   "Successfully assigned",
						InvolvedObject: corev1.ObjectReference{
							Name:      "test-pod",
							Namespace: "default",
						},
					},
				},
			},
			expectedCount:  1,
			expectedReason: "Scheduled",
		},
		{
			name:      "events with LastTimestamp",
			namespace: "default",
			podName:   "test-pod",
			events: &corev1.EventList{
				Items: []corev1.Event{
					{
						LastTimestamp: metav1.Time{Time: now.Add(-20 * time.Second)},
						Type:          "Normal",
						Reason:        "Pulled",
						Message:       "Successfully pulled image",
						InvolvedObject: corev1.ObjectReference{
							Name:      "test-pod",
							Namespace: "default",
						},
					},
				},
			},
			expectedCount:  1,
			expectedReason: "Pulled",
		},
		{
			name:      "multiple events with timestamps",
			namespace: "default",
			podName:   "test-pod",
			events: &corev1.EventList{
				Items: []corev1.Event{
					{
						EventTime: metav1.MicroTime{Time: now.Add(-10 * time.Second)},
						Reason:    "Started",
						InvolvedObject: corev1.ObjectReference{
							Name:      "test-pod",
							Namespace: "default",
						},
					},
					{
						EventTime: metav1.MicroTime{Time: now.Add(-30 * time.Second)},
						Reason:    "Scheduled",
						InvolvedObject: corev1.ObjectReference{
							Name:      "test-pod",
							Namespace: "default",
						},
					},
					{
						EventTime: metav1.MicroTime{Time: now.Add(-20 * time.Second)},
						Reason:    "Pulled",
						InvolvedObject: corev1.ObjectReference{
							Name:      "test-pod",
							Namespace: "default",
						},
					},
				},
			},
			expectedCount:  3,
			expectedReason: "Started", // Events are returned in order they appear, not sorted
		},
		{
			name:      "single pod event",
			namespace: "default",
			podName:   "test-pod",
			events: &corev1.EventList{
				Items: []corev1.Event{
					{
						Reason: "Started",
						InvolvedObject: corev1.ObjectReference{
							Name:      "test-pod",
							Namespace: "default",
						},
						EventTime: metav1.MicroTime{Time: now.Add(-5 * time.Second)},
					},
				},
			},
			expectedCount:  1,
			expectedReason: "Started",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", "events", func(action ktesting.Action) (bool, runtime.Object, error) {
				return true, tt.events, nil
			})

			collector := NewCollectorWithConfig(client, DefaultCollectorConfig())
			ctx := context.Background()

			events, err := collector.collectPodEvents(ctx, tt.namespace, tt.podName)

			assert.NoError(t, err)
			assert.Len(t, events, tt.expectedCount)
			if tt.expectedCount > 0 {
				assert.Equal(t, tt.expectedReason, events[0].Reason)
			}
		})
	}
}

func TestExtractImages(t *testing.T) {
	tests := []struct {
		name           string
		pod            *corev1.Pod
		expectedImages []string
	}{
		{
			name: "single container",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "nginx:latest",
						},
					},
				},
			},
			expectedImages: []string{"nginx:latest"},
		},
		{
			name: "multiple containers",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "nginx:latest",
						},
						{
							Image: "redis:alpine",
						},
					},
				},
			},
			expectedImages: []string{"nginx:latest", "redis:alpine"},
		},
		{
			name: "with init containers",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Image: "busybox:latest",
						},
					},
					Containers: []corev1.Container{
						{
							Image: "nginx:latest",
						},
					},
				},
			},
			expectedImages: []string{"busybox:latest", "nginx:latest"},
		},
	}

	collector := NewCollectorWithConfig(nil, DefaultCollectorConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			images := collector.extractImages(tt.pod)
			assert.Len(t, images, len(tt.expectedImages))

			// Check all expected images are present (order doesn't matter with map)
			for _, expected := range tt.expectedImages {
				assert.Contains(t, images, expected)
			}
		})
	}
}

func TestConvertContainerStatuses(t *testing.T) {
	now := time.Now()

	statuses := []corev1.ContainerStatus{
		{
			Name:  "nginx",
			Ready: true,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Time{Time: now.Add(-20 * time.Second)},
				},
			},
			RestartCount: 0,
		},
		{
			Name:  "failed-container",
			Ready: false,
			State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					StartedAt:  metav1.Time{Time: now.Add(-30 * time.Second)},
					FinishedAt: metav1.Time{Time: now.Add(-10 * time.Second)},
					ExitCode:   1,
					Reason:     "Error",
				},
			},
			RestartCount: 2,
		},
		{
			Name:  "waiting-container",
			Ready: false,
			State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{
					Reason:  "ImagePullBackOff",
					Message: "Back-off pulling image",
				},
			},
			RestartCount: 0,
		},
	}

	collector := NewCollectorWithConfig(nil, DefaultCollectorConfig())
	converted := collector.convertContainerStatuses(statuses)

	assert.Len(t, converted, 3)

	// Check running container
	assert.Equal(t, "nginx", converted[0].Name)
	assert.True(t, converted[0].Ready)
	assert.Equal(t, "Running", converted[0].State)
	assert.NotNil(t, converted[0].StartedAt)
	assert.Equal(t, int32(0), converted[0].RestartCount)

	// Check terminated container
	assert.Equal(t, "failed-container", converted[1].Name)
	assert.False(t, converted[1].Ready)
	assert.Equal(t, "Terminated", converted[1].State)
	assert.NotNil(t, converted[1].StartedAt)
	assert.NotNil(t, converted[1].FinishedAt)
	assert.Equal(t, int32(2), converted[1].RestartCount)

	// Check waiting container
	assert.Equal(t, "waiting-container", converted[2].Name)
	assert.False(t, converted[2].Ready)
	assert.Equal(t, "Waiting", converted[2].State)
	assert.Equal(t, int32(0), converted[2].RestartCount)
}

func TestWatchPod(t *testing.T) {
	tests := []struct {
		name        string
		podName     string
		namespace   string
		timeout     time.Duration
		pod         *corev1.Pod
		expectError bool
	}{
		{
			name:      "watch pod becomes ready",
			podName:   "test-pod",
			namespace: "default",
			timeout:   5 * time.Second,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "pod not found",
			podName:     "missing-pod",
			namespace:   "default",
			timeout:     1 * time.Second,
			pod:         nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []runtime.Object
			if tt.pod != nil {
				objects = append(objects, tt.pod)
			}
			client := fake.NewSimpleClientset(objects...)

			collector := NewCollectorWithConfig(client, DefaultCollectorConfig())
			ctx := context.Background()

			podInfo, err := collector.WatchPod(ctx, tt.namespace, tt.podName, tt.timeout)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, podInfo)
				assert.Equal(t, tt.podName, podInfo.Name)
			}
		})
	}
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name     string
		podInfo  *PodInfo
		expected bool
	}{
		{
			name: "pod is ready",
			podInfo: &PodInfo{
				Phase: "Running",
				Conditions: []PodCondition{
					{
						Type:   "Ready",
						Status: "True",
					},
				},
			},
			expected: true,
		},
		{
			name: "pod not ready - condition false",
			podInfo: &PodInfo{
				Phase: "Running",
				Conditions: []PodCondition{
					{
						Type:   "Ready",
						Status: "False",
					},
				},
			},
			expected: false,
		},
		{
			name: "pod not ready - no ready condition",
			podInfo: &PodInfo{
				Phase: "Pending",
				Conditions: []PodCondition{
					{
						Type:   "PodScheduled",
						Status: "True",
					},
				},
			},
			expected: false,
		},
		{
			name: "pod failed",
			podInfo: &PodInfo{
				Phase: "Failed",
			},
			expected: false,
		},
		{
			name: "pod succeeded without ready condition",
			podInfo: &PodInfo{
				Phase: "Succeeded",
			},
			expected: false, // isPodReady only checks Ready condition, not phase
		},
	}

	collector := NewCollectorWithConfig(nil, DefaultCollectorConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collector.isPodReady(tt.podInfo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCollectImageMetadata(t *testing.T) {
	tests := []struct {
		name            string
		pod             *corev1.Pod
		events          []Event
		expectedImages  int
		checkLargeImage bool
		checkLocalReg   bool
		checkRemoteReg  bool
	}{
		{
			name: "detect large image from pull time",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Image: "nginx:latest"},
					},
				},
			},
			events: []Event{
				{
					Reason:    "Pulling",
					Message:   "Pulling image \"nginx:latest\"",
					Timestamp: time.Now().Add(-60 * time.Second),
				},
				{
					Reason:    "Pulled",
					Message:   "Successfully pulled image \"nginx:latest\"",
					Timestamp: time.Now().Add(-10 * time.Second),
				},
			},
			expectedImages:  1,
			checkLargeImage: true, // 50s pull time = 500MB
			checkRemoteReg:  true, // docker.io
		},
		{
			name: "detect local registry",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Image: "localhost:5000/myapp:v1"},
					},
				},
			},
			events:         []Event{},
			expectedImages: 1,
			checkLocalReg:  true,
		},
		{
			name: "detect cluster-local registry",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Image: "registry.cluster.local/myapp:v1"},
					},
				},
			},
			events:         []Event{},
			expectedImages: 1,
			checkLocalReg:  true,
		},
		{
			name: "multiple images with different registries",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Image: "nginx:latest"},
						{Image: "localhost:5000/redis:alpine"},
					},
				},
			},
			events:         []Event{},
			expectedImages: 2,
		},
	}

	collector := NewCollectorWithConfig(nil, DefaultCollectorConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := collector.collectImageMetadata(tt.pod, tt.events)

			assert.Len(t, metadata, tt.expectedImages)

			if tt.expectedImages > 0 {
				imgInfo := metadata[0]

				if tt.checkLargeImage {
					assert.GreaterOrEqual(t, imgInfo.Size, int64(500*1024*1024), "Expected large image")
				}

				if tt.checkLocalReg {
					assert.True(t, imgInfo.IsLocal, "Expected local registry")
				}

				if tt.checkRemoteReg {
					assert.False(t, imgInfo.IsLocal, "Expected remote registry")
					assert.NotEmpty(t, imgInfo.Registry)
				}
			}
		})
	}
}

func TestParseImageRegistry(t *testing.T) {
	tests := []struct {
		name          string
		image         string
		expectedReg   string
		expectedLocal bool
	}{
		{
			name:          "docker hub short form",
			image:         "nginx:latest",
			expectedReg:   "docker.io",
			expectedLocal: false,
		},
		{
			name:          "docker hub with library",
			image:         "library/nginx:latest",
			expectedReg:   "docker.io",
			expectedLocal: false,
		},
		{
			name:          "localhost registry",
			image:         "localhost:5000/myapp:v1",
			expectedReg:   "localhost:5000",
			expectedLocal: true,
		},
		{
			name:          "cluster local registry",
			image:         "registry.cluster.local/myapp:v1",
			expectedReg:   "registry.cluster.local",
			expectedLocal: true,
		},
		{
			name:          "private IP registry",
			image:         "192.168.1.100:5000/myapp:v1",
			expectedReg:   "192.168.1.100:5000",
			expectedLocal: true,
		},
		{
			name:          "gcr.io registry",
			image:         "gcr.io/project/image:tag",
			expectedReg:   "gcr.io",
			expectedLocal: false,
		},
	}

	collector := NewCollectorWithConfig(nil, DefaultCollectorConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, isLocal := collector.parseImageRegistry(tt.image)
			assert.Equal(t, tt.expectedReg, registry)
			assert.Equal(t, tt.expectedLocal, isLocal)
		})
	}
}

func TestDetectResourceWaiting(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		pod           *corev1.Pod
		events        []Event
		expectWaiting bool
		expectedType  string
		minDuration   time.Duration
	}{
		{
			name: "insufficient CPU",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: now.Add(-2 * time.Minute)},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			events: []Event{
				{
					Reason:    "FailedScheduling",
					Message:   "0/3 nodes available: InsufficientCPU",
					Timestamp: now.Add(-2 * time.Minute),
				},
				{
					Reason:    "Scheduled",
					Message:   "Successfully assigned to node",
					Timestamp: now.Add(-30 * time.Second),
				},
			},
			expectWaiting: true,
			expectedType:  "CPU",
			minDuration:   90 * time.Second,
		},
		{
			name: "node selector mismatch",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: now.Add(-1 * time.Minute)},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:               corev1.PodScheduled,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: metav1.Time{Time: now.Add(-1 * time.Minute)},
							Message:            "0/5 nodes available: NodeSelectorMismatch",
						},
					},
				},
			},
			events:        []Event{},
			expectWaiting: true,
			expectedType:  "NodeSelector",
			minDuration:   1 * time.Minute,
		},
		{
			name: "no resource waiting",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			events: []Event{
				{
					Reason:    "Scheduled",
					Message:   "Successfully assigned to node",
					Timestamp: now.Add(-10 * time.Second),
				},
			},
			expectWaiting: false,
		},
	}

	collector := NewCollectorWithConfig(nil, DefaultCollectorConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waitInfo := collector.detectResourceWaiting(tt.pod, tt.events)

			if tt.expectWaiting {
				require.NotNil(t, waitInfo)
				assert.Equal(t, tt.expectedType, waitInfo.Type)
				assert.GreaterOrEqual(t, waitInfo.Duration, tt.minDuration)
				assert.NotEmpty(t, waitInfo.Message)
			} else {
				assert.Nil(t, waitInfo)
			}
		})
	}
}
