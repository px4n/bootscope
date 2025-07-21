// Package analyzer implements pod startup analysis and recommendation generation.
//
// This package takes raw pod data collected by the collector package and produces
// a comprehensive analysis of the pod's startup sequence, identifying bottlenecks
// and generating actionable recommendations.
//
// # Overview
//
// The analyzer performs several key functions:
//   - Detects and times distinct startup phases
//   - Identifies performance bottlenecks
//   - Generates optimization recommendations
//   - Handles pod restarts and phase consolidation
//   - Calculates accurate timing for various pod states
//
// # Usage
//
// Create an analyzer and analyze a pod:
//
//	analyzer := analyzer.NewAnalyzerWithConfig(config)
//	profile, err := analyzer.AnalyzePod(podInfo)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access analysis results
//	fmt.Printf("Total startup time: %s\n", profile.TotalTime)
//	for _, bottleneck := range profile.Bottlenecks {
//	    fmt.Printf("Bottleneck: %s took %s (%.0f%%)\n",
//	        bottleneck.Phase, bottleneck.Duration, bottleneck.Percentage)
//	}
//
// # Phase Detection
//
// The analyzer uses Kubernetes events and pod conditions to detect phases:
//   - Scheduling: From pod creation to node assignment
//   - Image Pull: Detected from Pulling/Pulled events
//   - Container Creation: From image pulled to container created
//   - Init Containers: Time spent in init containers
//   - Application Start: From container start to ready condition
//
// # Recommendations
//
// The analyzer generates recommendations based on:
//   - Image size and pull times
//   - Init container duration
//   - Application startup time
//   - Resource waiting patterns
//   - Registry location (local vs remote)
package analyzer
