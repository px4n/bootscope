// Package output provides formatting utilities for displaying analysis results.
//
// This package contains functions for formatting durations, percentages, timestamps,
// and calculating statistics for deployment analysis. It handles the presentation
// logic for bootscope's output.
//
// # Formatting Functions
//
// Duration formatting:
//
//	d := 90500 * time.Millisecond
//	fmt.Println(output.FormatDuration(d))  // "1m30.5s"
//
// Percentage formatting:
//
//	fmt.Println(output.FormatPercentage(45.678))  // "46%"
//
// # Statistics
//
// The package provides statistical analysis for deployment data:
//
//	times := []time.Duration{10*time.Second, 20*time.Second, 30*time.Second}
//	stats := output.CalculateStats(times)
//	fmt.Printf("Average: %s\n", output.FormatDuration(stats.Avg))
//	fmt.Printf("P95: %s\n", output.FormatDuration(stats.P95))
//
// # Display Helpers
//
// The package includes helpers for terminal output:
//   - Status icons (✓, ⚠, ✗, etc.)
//   - Color selection based on duration thresholds
//   - Progress indicators
package output
