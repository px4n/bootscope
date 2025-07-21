package output

import (
	"math"
	"sort"
	"time"
)

// Stats represents statistical calculations for a set of durations
type Stats struct {
	Count  int
	Min    time.Duration
	Max    time.Duration
	Avg    time.Duration
	Median time.Duration
	P95    time.Duration
	P99    time.Duration
	StdDev time.Duration
}

// CalculateStats computes statistics for a slice of durations
func CalculateStats(durations []time.Duration) Stats {
	if len(durations) == 0 {
		return Stats{}
	}

	// Sort durations for percentile calculations
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	stats := Stats{
		Count: len(durations),
		Min:   sorted[0],
		Max:   sorted[len(sorted)-1],
	}

	// Calculate average
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	stats.Avg = sum / time.Duration(len(durations))

	// Calculate median
	if len(sorted)%2 == 0 {
		stats.Median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	} else {
		stats.Median = sorted[len(sorted)/2]
	}

	// Calculate percentiles
	stats.P95 = percentile(sorted, 95)
	stats.P99 = percentile(sorted, 99)

	// Calculate standard deviation
	var variance float64
	avgNanos := float64(stats.Avg.Nanoseconds())
	for _, d := range durations {
		diff := float64(d.Nanoseconds()) - avgNanos
		variance += diff * diff
	}
	variance /= float64(len(durations))
	stats.StdDev = time.Duration(math.Sqrt(variance))

	return stats
}

// percentile calculates the nth percentile of sorted durations
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	index := (p / 100) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[lower]
	}

	weight := index - float64(lower)
	return time.Duration(float64(sorted[lower])*(1-weight) + float64(sorted[upper])*weight)
}
