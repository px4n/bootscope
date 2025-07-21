// Package types provides the core data structures for the BootScope pod startup profiler.
//
// This package contains all the public types that are exposed to consumers of the
// bootscope library. These types are designed to be stable and backward-compatible.
//
// # Main Types
//
// The primary type is PodStartupProfile, which contains a complete analysis of a
// pod's startup sequence:
//
//	profile := &types.PodStartupProfile{
//	    PodName:   "my-pod",
//	    Namespace: "default",
//	    TotalTime: 45 * time.Second,
//	    Phases: []types.PhaseInfo{
//	        {
//	            Phase:    types.StartupPhaseScheduling,
//	            Duration: 2 * time.Second,
//	        },
//	        {
//	            Phase:    types.StartupPhaseImagePull,
//	            Duration: 30 * time.Second,
//	        },
//	    },
//	}
//
// # Startup Phases
//
// The pod startup process is divided into distinct phases:
//
//   - Scheduling: Finding a suitable node
//   - ImagePull: Downloading container images
//   - ContainerCreation: Creating containers
//   - InitContainers: Running init containers
//   - ApplicationStart: Starting the main application
//   - Ready: Pod becomes ready to serve traffic
//
// # Analysis Results
//
// The package provides types for analysis results:
//
//   - Bottleneck: Identifies phases that took significant time
//   - Recommendation: Provides actionable advice for improvements
//   - ContainerStatusInfo: Current state of containers
//   - ImageInfo: Metadata about container images
//   - ResourceWaitInfo: Information about resource waiting
package types
