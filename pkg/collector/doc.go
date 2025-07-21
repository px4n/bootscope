// Package collector implements Kubernetes data collection for pod startup analysis.
//
// This package is responsible for gathering all the raw data needed to analyze
// pod startup times. It interfaces with the Kubernetes API to collect pod state,
// events, and metadata.
//
// # Overview
//
// The collector retrieves:
//   - Pod status and conditions
//   - Container states and restart counts
//   - Kubernetes events related to the pod
//   - Image metadata (size, registry, pull times)
//   - Resource waiting information
//
// # Usage
//
// Create a collector with a Kubernetes client:
//
//	clientset := kubernetes.NewForConfigOrDie(config)
//	collector := collector.NewCollectorWithConfig(clientset, &collector.Config{
//	    WatchPollInterval: 2 * time.Second,
//	    NetworkSpeedAverage: 25 * 1024 * 1024, // 25 MB/s
//	})
//
// Collect pod information:
//
//	podInfo, err := collector.CollectPodInfo(ctx, "namespace", "pod-name")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Watch a pod until it's ready:
//
//	podInfo, err := collector.WatchPod(ctx, "namespace", "pod-name", 5*time.Minute)
//
// # Configuration
//
// The collector can be configured with network speeds, registry patterns, and
// polling intervals to improve accuracy of image size estimation and local
// registry detection.
package collector
