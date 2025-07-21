package analyzer

import (
	"fmt"
	"testing"
	"time"

	"github.com/px4n/bootscope/pkg/collector"
	"github.com/px4n/bootscope/pkg/types"
)

// BenchmarkAnalyzePod benchmarks the pod analysis with different scenarios
func BenchmarkAnalyzePod(b *testing.B) {
	scenarios := []struct {
		name      string
		setupFunc func() *collector.PodInfo
	}{
		{
			name: "simple_pod",
			setupFunc: func() *collector.PodInfo {
				now := time.Now()
				return &collector.PodInfo{
					Name:              "bench-pod",
					Namespace:         "default",
					CreationTimestamp: now.Add(-30 * time.Second),
					NodeName:          "worker-1",
					Phase:             "Running",
					Conditions: []collector.PodCondition{
						{
							Type:               "Ready",
							Status:             "True",
							LastTransitionTime: now,
						},
					},
					Events: []collector.Event{
						{Reason: "Scheduled", Timestamp: now.Add(-29 * time.Second)},
						{Reason: "Pulling", Timestamp: now.Add(-25 * time.Second)},
						{Reason: "Pulled", Timestamp: now.Add(-10 * time.Second)},
						{Reason: "Created", Timestamp: now.Add(-5 * time.Second)},
						{Reason: "Started", Timestamp: now.Add(-3 * time.Second)},
					},
				}
			},
		},
		{
			name: "complex_pod_many_events",
			setupFunc: func() *collector.PodInfo {
				now := time.Now()
				info := &collector.PodInfo{
					Name:              "complex-pod",
					Namespace:         "default",
					CreationTimestamp: now.Add(-300 * time.Second),
					NodeName:          "worker-1",
					Phase:             "Running",
				}

				// Add 100 events to simulate a pod with many retries/issues
				for i := 0; i < 100; i++ {
					info.Events = append(info.Events, collector.Event{
						Reason:    "Pulling",
						Timestamp: now.Add(time.Duration(-300+i) * time.Second),
						Message:   "Pulling image attempt " + string(rune(i)),
					})
				}

				// Add final success events
				info.Events = append(info.Events,
					collector.Event{Reason: "Pulled", Timestamp: now.Add(-30 * time.Second)},
					collector.Event{Reason: "Created", Timestamp: now.Add(-20 * time.Second)},
					collector.Event{Reason: "Started", Timestamp: now.Add(-15 * time.Second)},
				)

				info.Conditions = []collector.PodCondition{
					{Type: "Ready", Status: "True", LastTransitionTime: now},
				}

				return info
			},
		},
		{
			name: "pod_with_many_init_containers",
			setupFunc: func() *collector.PodInfo {
				now := time.Now()
				info := &collector.PodInfo{
					Name:              "init-heavy-pod",
					Namespace:         "default",
					CreationTimestamp: now.Add(-600 * time.Second),
					Phase:             "Running",
				}

				// Add 20 init containers
				for i := 0; i < 20; i++ {
					started := now.Add(time.Duration(-600+i*20) * time.Second)
					finished := now.Add(time.Duration(-600+i*20+15) * time.Second)
					info.InitContainers = append(info.InitContainers, collector.ContainerStatus{
						Name:       fmt.Sprintf("init-%d", i),
						State:      "Terminated",
						StartedAt:  &started,
						FinishedAt: &finished,
					})
				}

				started := now.Add(-100 * time.Second)
				info.ContainerStatuses = []collector.ContainerStatus{
					{
						Name:      "main",
						State:     "Running",
						Ready:     true,
						StartedAt: &started,
					},
				}

				info.Conditions = []collector.PodCondition{
					{Type: "Ready", Status: "True", LastTransitionTime: now.Add(-50 * time.Second)},
				}

				return info
			},
		},
		{
			name: "large_deployment_pod",
			setupFunc: func() *collector.PodInfo {
				// Simulate analyzing one pod from a large deployment
				// with lots of metadata and annotations
				now := time.Now()
				info := &collector.PodInfo{
					Name:              "app-deployment-7d4f689c5b-x2n4k",
					Namespace:         "production",
					CreationTimestamp: now.Add(-120 * time.Second),
					NodeName:          "worker-node-us-east-1a-i-0a1b2c3d4e5f67890",
					Phase:             "Running",
				}

				// Add realistic events for production pod
				events := []struct {
					reason string
					offset time.Duration
					msg    string
				}{
					{"TriggeredScaleUp", -119 * time.Second, "pod triggered scale-up"},
					{"Scheduled", -115 * time.Second, "Successfully assigned to node"},
					{"Pulling", -110 * time.Second, "Pulling image \"company.registry.io/app:v2.3.4\""},
					{"Pulled", -70 * time.Second, "Successfully pulled image in 40s (1.2GB)"},
					{"Created", -65 * time.Second, "Created container app"},
					{"Started", -63 * time.Second, "Started container app"},
					{"Readiness", -30 * time.Second, "Readiness probe failed"},
					{"Readiness", -25 * time.Second, "Readiness probe failed"},
					{"Readiness", -20 * time.Second, "Readiness probe passed"},
				}

				for _, e := range events {
					info.Events = append(info.Events, collector.Event{
						Reason:    e.reason,
						Timestamp: now.Add(e.offset),
						Message:   e.msg,
					})
				}

				// Add container with resource info
				started := now.Add(-63 * time.Second)
				info.ContainerStatuses = []collector.ContainerStatus{
					{
						Name:      "app",
						State:     "Running",
						Ready:     true,
						StartedAt: &started,
					},
				}

				// Add images and image metadata
				info.Images = []string{"company.registry.io/app:v2.3.4"}
				info.ImageMetadata = []types.ImageInfo{
					{
						Name:     "company.registry.io/app:v2.3.4",
						Size:     1200 * 1024 * 1024, // 1.2GB
						PullTime: 40 * time.Second,
					},
				}

				info.Conditions = []collector.PodCondition{
					{Type: "PodScheduled", Status: "True", LastTransitionTime: now.Add(-115 * time.Second)},
					{Type: "Initialized", Status: "True", LastTransitionTime: now.Add(-65 * time.Second)},
					{Type: "ContainersReady", Status: "True", LastTransitionTime: now.Add(-20 * time.Second)},
					{Type: "Ready", Status: "True", LastTransitionTime: now.Add(-20 * time.Second)},
				}

				return info
			},
		},
	}

	// Create analyzer once
	analyzer := NewAnalyzer()

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			podInfo := scenario.setupFunc()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				profile, err := analyzer.AnalyzePod(podInfo)
				if err != nil {
					b.Fatal(err)
				}
				if profile == nil {
					b.Fatal("profile should not be nil")
				}
			}
		})
	}
}

// BenchmarkPhaseDetection benchmarks individual phase detection functions
func BenchmarkPhaseDetection(b *testing.B) {
	analyzer := NewAnalyzer()
	now := time.Now()

	// Create a pod info with many events
	podInfo := &collector.PodInfo{
		CreationTimestamp: now.Add(-300 * time.Second),
		Events:            make([]collector.Event, 0, 200),
	}

	// Add many events
	for i := 0; i < 200; i++ {
		podInfo.Events = append(podInfo.Events, collector.Event{
			Reason:    "SomeEvent",
			Timestamp: now.Add(time.Duration(-300+i) * time.Second),
			Message:   fmt.Sprintf("Event message %d", i),
		})
	}

	b.Run("detectPhases", func(b *testing.B) {
		// Add specific events to ensure all phase detection paths are exercised
		podInfo.Events = append(podInfo.Events,
			collector.Event{Reason: "Scheduled", Message: "Successfully assigned to node", Timestamp: now.Add(-250 * time.Second)},
			collector.Event{Reason: "Pulling", Message: "Pulling image \"nginx:latest\"", Timestamp: now.Add(-200 * time.Second)},
			collector.Event{Reason: "Pulled", Message: "Successfully pulled image \"nginx:latest\"", Timestamp: now.Add(-150 * time.Second)},
			collector.Event{Reason: "Created", Message: "Created container", Timestamp: now.Add(-100 * time.Second)},
			collector.Event{Reason: "Started", Message: "Started container", Timestamp: now.Add(-90 * time.Second)},
		)

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = analyzer.detectPhases(podInfo)
		}
	})

	b.Run("fixPhaseTimings", func(b *testing.B) {
		// Create phases that need timing fixes
		phases := analyzer.detectPhases(podInfo)

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Make a copy to avoid modifying the original
			phasesCopy := make([]types.PhaseInfo, len(phases))
			copy(phasesCopy, phases)
			analyzer.fixPhaseTimings(phasesCopy)
		}
	})
}

// BenchmarkRecommendationGeneration benchmarks recommendation generation
func BenchmarkRecommendationGeneration(b *testing.B) {
	analyzer := NewAnalyzer()

	scenarios := []struct {
		name   string
		phases []types.PhaseInfo
	}{
		{
			name: "no_issues",
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseScheduling, Duration: 100 * time.Millisecond},
				{Phase: types.StartupPhaseImagePull, Duration: 5 * time.Second},
				{Phase: types.StartupPhaseAppStart, Duration: 2 * time.Second},
			},
		},
		{
			name: "many_issues",
			phases: []types.PhaseInfo{
				{Phase: types.StartupPhaseScheduling, Duration: 30 * time.Second},     // Slow scheduling
				{Phase: types.StartupPhaseImagePull, Duration: 120 * time.Second},     // Very slow pull
				{Phase: types.StartupPhaseInitContainers, Duration: 60 * time.Second}, // Slow init
				{Phase: types.StartupPhaseAppStart, Duration: 45 * time.Second},       // Slow start
			},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			podInfo := &collector.PodInfo{
				Images: []string{"app:latest"},
				ImageMetadata: []types.ImageInfo{
					{Name: "app:latest", Size: 2 * 1024 * 1024 * 1024}, // 2GB image
				},
			}

			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				profile := &types.PodStartupProfile{
					Phases:        scenario.phases,
					ImageMetadata: podInfo.ImageMetadata,
				}
				_ = analyzer.generateRecommendations(podInfo, profile)
			}
		})
	}
}
