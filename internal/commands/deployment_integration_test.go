package commands

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
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

func createTestDeployment(name, namespace string) *appsv1.Deployment {
	replicas := int32(3)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:      replicas,
			ReadyReplicas: replicas,
		},
	}
}

func createDeploymentPods(deploymentName, namespace string, count int) []*corev1.Pod {
	pods := make([]*corev1.Pod, count)
	now := metav1.Now()

	for i := 0; i < count; i++ {
		startTime := now.Add(-time.Duration(60-i*10) * time.Second)
		pods[i] = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-pod-%d", deploymentName, i),
				Namespace: namespace,
				Labels: map[string]string{
					"app": deploymentName,
				},
				CreationTimestamp: metav1.NewTime(startTime),
			},
			Spec: corev1.PodSpec{
				NodeName: fmt.Sprintf("node-%d", i),
				Containers: []corev1.Container{
					{
						Name:  "main",
						Image: "nginx:latest",
					},
				},
			},
			Status: corev1.PodStatus{
				Phase:     corev1.PodRunning,
				StartTime: &metav1.Time{Time: startTime},
				Conditions: []corev1.PodCondition{
					{
						Type:               corev1.PodScheduled,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.NewTime(startTime.Add(2 * time.Second)),
					},
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.NewTime(startTime.Add(30 * time.Second)),
					},
				},
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:         "main",
						Ready:        true,
						RestartCount: 0,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.NewTime(startTime.Add(25 * time.Second)),
							},
						},
					},
				},
			},
		}
	}

	return pods
}

func createDeploymentEvents(deploymentName, namespace string, podCount int) *corev1.EventList {
	events := []corev1.Event{}
	now := metav1.Now()

	for i := 0; i < podCount; i++ {
		podName := fmt.Sprintf("%s-pod-%d", deploymentName, i)
		startTime := now.Add(-time.Duration(60-i*10) * time.Second)

		events = append(events,
			corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("event-scheduled-%d", i),
					Namespace: namespace,
				},
				InvolvedObject: corev1.ObjectReference{
					Name:      podName,
					Namespace: namespace,
				},
				Reason:         "Scheduled",
				Message:        fmt.Sprintf("Successfully assigned %s/%s to node-%d", namespace, podName, i),
				FirstTimestamp: metav1.NewTime(startTime.Add(2 * time.Second)),
				Type:           "Normal",
			},
			corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("event-pulled-%d", i),
					Namespace: namespace,
				},
				InvolvedObject: corev1.ObjectReference{
					Name:      podName,
					Namespace: namespace,
				},
				Reason:         "Pulled",
				Message:        "Successfully pulled image nginx:latest",
				FirstTimestamp: metav1.NewTime(startTime.Add(15 * time.Second)),
				Type:           "Normal",
			},
			corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("event-started-%d", i),
					Namespace: namespace,
				},
				InvolvedObject: corev1.ObjectReference{
					Name:      podName,
					Namespace: namespace,
				},
				Reason:         "Started",
				Message:        "Started container main",
				FirstTimestamp: metav1.NewTime(startTime.Add(25 * time.Second)),
				Type:           "Normal",
			},
		)
	}

	return &corev1.EventList{Items: events}
}

func TestAnalyzeDeployment_Integration_InvalidNames(t *testing.T) {
	// Create a deployment and pods (won't be reached due to validation)
	deployment := createTestDeployment("test-deployment", "default")
	pods := createDeploymentPods("test-deployment", "default", 1)

	fakeClient := fake.NewSimpleClientset(deployment)
	for _, pod := range pods {
		fakeClient.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
	}

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	testCases := []struct {
		name           string
		namespace      string
		deploymentName string
		errMsg         string
	}{
		{
			name:           "invalid deployment name with uppercase",
			namespace:      "default",
			deploymentName: "MyDeployment",
			errMsg:         "invalid deployment name",
		},
		{
			name:           "invalid namespace with spaces",
			namespace:      "my namespace",
			deploymentName: "test-deployment",
			errMsg:         "invalid namespace name",
		},
		{
			name:           "empty deployment name",
			namespace:      "default",
			deploymentName: "",
			errMsg:         "deployment name cannot be empty",
		},
		{
			name:           "deployment name with special characters",
			namespace:      "default",
			deploymentName: "test_deployment!",
			errMsg:         "invalid deployment name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := AnalyzeDeployment(ctx, fakeClient, coll, anal, cfg, tc.namespace, tc.deploymentName)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestAnalyzeDeployment_Integration_WithEvents(t *testing.T) {
	deployment := createTestDeployment("test-deployment", "default")
	pods := createDeploymentPods("test-deployment", "default", 3)
	events := createDeploymentEvents("test-deployment", "default", 3)

	// Create fake client with deployment
	fakeClient := fake.NewSimpleClientset(deployment)

	// Add pods
	for _, pod := range pods {
		_, err := fakeClient.CoreV1().Pods(pod.Namespace).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Add events using reactor
	fakeClient.PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, events, nil
	})

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test deployment analysis with events
	output := testutil.CaptureStdout(func() {
		err := AnalyzeDeployment(ctx, fakeClient, coll, anal, cfg, "default", "test-deployment")
		require.NoError(t, err)
	})

	// Verify output includes event data
	assert.Contains(t, output, "Deployment Startup Analysis:")
	assert.Contains(t, output, "test-deployment")
	assert.Contains(t, output, "Pods analyzed: 3")
	assert.Contains(t, output, "Scheduling")
	// Note: ImagePull phase won't appear because test pods are created in a Running state
	// without container status history showing image pulls
	assert.Contains(t, output, "ApplicationStart")
}

func TestAnalyzeDeployment_Integration_Success(t *testing.T) {
	deployment := createTestDeployment("test-deployment", "default")
	pods := createDeploymentPods("test-deployment", "default", 3)

	// Create fake client with deployment
	fakeClient := fake.NewSimpleClientset(deployment)

	// Add pods
	for _, pod := range pods {
		_, err := fakeClient.CoreV1().Pods(pod.Namespace).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Add events using reactor
	fakeClient.PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		// Return empty events for simplicity
		return true, &corev1.EventList{}, nil
	})

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test deployment analysis
	output := testutil.CaptureStdout(func() {
		err := AnalyzeDeployment(ctx, fakeClient, coll, anal, cfg, "default", "test-deployment")
		require.NoError(t, err)
	})

	// Verify output
	assert.Contains(t, output, "Deployment Startup Analysis:")
	assert.Contains(t, output, "test-deployment")
	assert.Contains(t, output, "Pods analyzed: 3")
	assert.Contains(t, output, "Phase Statistics:")
}

func TestAnalyzeDeployment_Integration_NotFound(t *testing.T) {
	// Create fake client without deployment
	fakeClient := fake.NewSimpleClientset()

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test deployment not found
	err := AnalyzeDeployment(ctx, fakeClient, coll, anal, cfg, "default", "missing-deployment")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestAnalyzeDeployment_Integration_NoPods(t *testing.T) {
	// Create deployment without pods
	deployment := createTestDeployment("empty-deployment", "default")
	fakeClient := fake.NewSimpleClientset(deployment)

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test deployment with no pods - should return an error
	err := AnalyzeDeployment(ctx, fakeClient, coll, anal, cfg, "default", "empty-deployment")

	// Verify error is returned for deployment with no pods
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found for deployment empty-deployment")
}

func TestAnalyzeDeployment_Integration_MixedPodStates(t *testing.T) {
	deployment := createTestDeployment("mixed-deployment", "default")

	// Create pods in different states
	runningPod := createTestPod("mixed-deployment-pod-1", "default", corev1.PodRunning)
	runningPod.Labels = map[string]string{"app": "mixed-deployment"}

	pendingPod := createTestPod("mixed-deployment-pod-2", "default", corev1.PodPending)
	pendingPod.Labels = map[string]string{"app": "mixed-deployment"}

	failedPod := createTestPod("mixed-deployment-pod-3", "default", corev1.PodFailed)
	failedPod.Labels = map[string]string{"app": "mixed-deployment"}

	fakeClient := fake.NewSimpleClientset(deployment, runningPod, pendingPod, failedPod)

	// Add events
	fakeClient.PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &corev1.EventList{}, nil
	})

	coll := collector.NewCollectorWithConfig(fakeClient, collector.DefaultCollectorConfig())
	anal := analyzer.NewAnalyzer()
	cfg := config.DefaultConfig()

	ctx := context.Background()

	// Test deployment with mixed pod states
	output := testutil.CaptureStdout(func() {
		err := AnalyzeDeployment(ctx, fakeClient, coll, anal, cfg, "default", "mixed-deployment")
		require.NoError(t, err)
	})

	// Verify output shows multiple pods
	assert.Contains(t, output, "Deployment Startup Analysis:")
	assert.Contains(t, output, "Pods analyzed: 3")
}
