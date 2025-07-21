package commands

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
	k8stesting "k8s.io/client-go/testing"

	"github.com/px4n/bootscope/internal/testutil"
	"github.com/px4n/bootscope/pkg/analyzer"
	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/config"
)

func createTestPod(name, namespace string, phase corev1.PodPhase) *corev1.Pod {
	now := metav1.Now()
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(now.Add(-60 * time.Second)),
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "nginx:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase:     phase,
			StartTime: &now,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodScheduled,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(now.Add(-58 * time.Second)),
				},
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(now.Add(-30 * time.Second)),
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "main",
					Ready:        true,
					RestartCount: 0,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: metav1.NewTime(now.Add(-30 * time.Second)),
						},
					},
				},
			},
		},
	}
}

func createTestEvents(podName, namespace string) *corev1.EventList {
	now := metav1.Now()
	return &corev1.EventList{
		Items: []corev1.Event{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event1",
					Namespace: namespace,
				},
				InvolvedObject: corev1.ObjectReference{
					Name:      podName,
					Namespace: namespace,
				},
				Reason:         "Scheduled",
				Message:        "Successfully assigned test-namespace/test-pod to test-node",
				FirstTimestamp: metav1.NewTime(now.Add(-58 * time.Second)),
				Type:           "Normal",
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event2",
					Namespace: namespace,
				},
				InvolvedObject: corev1.ObjectReference{
					Name:      podName,
					Namespace: namespace,
				},
				Reason:         "Pulled",
				Message:        "Successfully pulled image nginx:latest",
				FirstTimestamp: metav1.NewTime(now.Add(-40 * time.Second)),
				Type:           "Normal",
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event3",
					Namespace: namespace,
				},
				InvolvedObject: corev1.ObjectReference{
					Name:      podName,
					Namespace: namespace,
				},
				Reason:         "Started",
				Message:        "Started container main",
				FirstTimestamp: metav1.NewTime(now.Add(-30 * time.Second)),
				Type:           "Normal",
			},
		},
	}
}

func TestAnalyzePod_Integration_SimpleMode(t *testing.T) {
	// Create fake client with test data
	pod := createTestPod("test-pod", "default", corev1.PodRunning)
	events := createTestEvents("test-pod", "default")
	fakeClient := fake.NewSimpleClientset(pod)

	// Add events using the reactor
	fakeClient.PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, events, nil
	})

	// Create real collector and analyzer with fake client
	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test simple mode
	output := testutil.CaptureStdout(func() {
		err := AnalyzePod(ctx, coll, anal, cfg, "default", "test-pod", false, "2m", "simple", true, false)
		require.NoError(t, err)
	})

	// Verify output
	assert.Contains(t, output, "Your pod started in")
	assert.Contains(t, output, "Finding a node:")
	assert.Contains(t, output, "Total time saved if fixed:")
}

func TestAnalyzePod_Integration_TextMode(t *testing.T) {
	// Create fake client with test data
	pod := createTestPod("test-pod", "default", corev1.PodRunning)
	events := createTestEvents("test-pod", "default")
	fakeClient := fake.NewSimpleClientset(pod)

	fakeClient.PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, events, nil
	})

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test text mode
	output := testutil.CaptureStdout(func() {
		err := AnalyzePod(ctx, coll, anal, cfg, "default", "test-pod", false, "2m", "text", false, false)
		require.NoError(t, err)
	})

	// Verify output
	assert.Contains(t, output, "Pod Startup Profile:")
	assert.Contains(t, output, "default/test-pod")
	assert.Contains(t, output, "Total Time:")
	assert.Contains(t, output, "Status: Running")
	assert.Contains(t, output, "Phase Breakdown:")
}

func TestAnalyzePod_Integration_JSONMode(t *testing.T) {
	// Create fake client with test data
	pod := createTestPod("test-pod", "default", corev1.PodRunning)
	events := createTestEvents("test-pod", "default")
	fakeClient := fake.NewSimpleClientset(pod)

	fakeClient.PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, events, nil
	})

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test JSON mode
	output := testutil.CaptureStdout(func() {
		err := AnalyzePod(ctx, coll, anal, cfg, "default", "test-pod", false, "2m", "json", false, false)
		require.NoError(t, err)
	})

	// Verify JSON output
	assert.Contains(t, output, "\"podName\":")
	assert.Contains(t, output, "\"namespace\":")
	assert.Contains(t, output, "\"status\":")
	assert.Contains(t, output, "\"phases\":")
}

func TestAnalyzePod_Integration_DebugMode(t *testing.T) {
	// Create fake client with test data
	pod := createTestPod("test-pod", "default", corev1.PodRunning)
	events := createTestEvents("test-pod", "default")
	fakeClient := fake.NewSimpleClientset(pod)

	fakeClient.PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, events, nil
	})

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test debug mode
	output := testutil.CaptureStdout(func() {
		err := AnalyzePod(ctx, coll, anal, cfg, "default", "test-pod", false, "2m", "text", false, true)
		require.NoError(t, err)
	})

	// Verify debug output
	assert.Contains(t, output, "Pod Startup Profile:")
	assert.Contains(t, output, "DEBUG INFORMATION")
	assert.Contains(t, output, "Events (sorted by time):")
	assert.Contains(t, output, "Conditions:")
}

func TestAnalyzePod_Integration_PodNotFound(t *testing.T) {
	// Create fake client without any pods
	fakeClient := fake.NewSimpleClientset()

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test pod not found
	err := AnalyzePod(ctx, coll, anal, cfg, "default", "missing-pod", false, "2m", "text", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestAnalyzePod_Integration_InvalidOutputFormat(t *testing.T) {
	// Create fake client with test data
	pod := createTestPod("test-pod", "default", corev1.PodRunning)
	fakeClient := fake.NewSimpleClientset(pod)

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// The function defaults to text output for unrecognized formats
	err := AnalyzePod(ctx, coll, anal, cfg, "default", "test-pod", false, "2m", "invalid", false, false)
	assert.NoError(t, err, "Invalid output format should not produce an error, it should default to text output")
}

func TestAnalyzePod_Integration_InvalidNames(t *testing.T) {
	// Create a valid pod for testing (won't be reached due to validation)
	pod := createTestPod("test-pod", "default", corev1.PodRunning)
	fakeClient := fake.NewSimpleClientset(pod)

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	testCases := []struct {
		name      string
		namespace string
		podName   string
		errMsg    string
	}{
		{
			name:      "invalid pod name with uppercase",
			namespace: "default",
			podName:   "MyPod",
			errMsg:    "invalid pod name",
		},
		{
			name:      "invalid namespace with spaces",
			namespace: "my namespace",
			podName:   "test-pod",
			errMsg:    "invalid namespace name",
		},
		{
			name:      "empty pod name",
			namespace: "default",
			podName:   "",
			errMsg:    "pod name cannot be empty",
		},
		{
			name:      "empty namespace",
			namespace: "",
			podName:   "test-pod",
			errMsg:    "namespace name cannot be empty",
		},
		{
			name:      "pod name with special characters",
			namespace: "default",
			podName:   "test_pod!",
			errMsg:    "invalid pod name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := AnalyzePod(ctx, coll, anal, cfg, tc.namespace, tc.podName, false, "", "text", false, false)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestAnalyzePod_Integration_InvalidTimeout(t *testing.T) {
	// Create a valid pod for testing (won't be reached due to validation)
	pod := createTestPod("test-pod", "default", corev1.PodRunning)
	fakeClient := fake.NewSimpleClientset(pod)

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	testCases := []struct {
		name    string
		timeout string
		errMsg  string
	}{
		{
			name:    "timeout too short",
			timeout: "500ms",
			errMsg:  "timeout too short",
		},
		{
			name:    "timeout too long",
			timeout: "2h",
			errMsg:  "timeout too long",
		},
		{
			name:    "invalid timeout format",
			timeout: "invalid",
			errMsg:  "invalid timeout",
		},
		{
			name:    "negative timeout",
			timeout: "-5s",
			errMsg:  "timeout too short",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := AnalyzePod(ctx, coll, anal, cfg, "default", "test-pod", true, tc.timeout, "text", false, false)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestAnalyzePod_Integration_PendingPod(t *testing.T) {
	// Create a pending pod
	pod := createTestPod("pending-pod", "default", corev1.PodPending)
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:    corev1.PodScheduled,
			Status:  corev1.ConditionFalse,
			Reason:  "Unschedulable",
			Message: "0/3 nodes are available: insufficient cpu",
		},
	}

	fakeClient := fake.NewSimpleClientset(pod)

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test pending pod
	output := testutil.CaptureStdout(func() {
		err := AnalyzePod(ctx, coll, anal, cfg, "default", "pending-pod", false, "2m", "text", false, false)
		require.NoError(t, err)
	})

	// Verify output shows pending status
	assert.Contains(t, output, "Status: Pending")
}

func TestAnalyzePod_Integration_FailedPod(t *testing.T) {
	// Create a failed pod
	pod := createTestPod("failed-pod", "default", corev1.PodFailed)
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			Name: "main",
			State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 1,
					Reason:   "Error",
					Message:  "Container failed to start",
				},
			},
		},
	}

	events := &corev1.EventList{
		Items: []corev1.Event{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event1",
					Namespace: "default",
				},
				InvolvedObject: corev1.ObjectReference{
					Name:      "failed-pod",
					Namespace: "default",
				},
				Reason:  "Failed",
				Message: "Error: Container failed to start",
				Type:    "Warning",
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(pod)
	fakeClient.PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, events, nil
	})

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test failed pod
	output := testutil.CaptureStdout(func() {
		err := AnalyzePod(ctx, coll, anal, cfg, "default", "failed-pod", false, "2m", "text", false, false)
		require.NoError(t, err)
	})

	// Verify output shows failed status
	assert.Contains(t, output, "Status: Failed")
}
