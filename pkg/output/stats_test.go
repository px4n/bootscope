package output

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateStats(t *testing.T) {
	tests := []struct {
		name      string
		durations []time.Duration
		want      Stats
		scenario  string
	}{
		{
			name: "typical pod startup times",
			durations: []time.Duration{
				5 * time.Second,  // Fast pod
				8 * time.Second,  // Normal pod
				12 * time.Second, // Slower pod
				15 * time.Second, // Slow pod
				45 * time.Second, // Very slow pod (outlier)
			},
			want: Stats{
				Min:    5 * time.Second,
				Max:    45 * time.Second,
				Avg:    17 * time.Second,
				Median: 12 * time.Second,
				P95:    38999999999 * time.Nanosecond, // ~39s
				P99:    43800000000 * time.Nanosecond, // ~43.8s
				StdDev: 14408330923 * time.Nanosecond, // ~14.4s
			},
			scenario: "Mixed deployment with one outlier pod taking much longer",
		},
		{
			name: "consistent fast pods",
			durations: []time.Duration{
				2 * time.Second,
				2100 * time.Millisecond,
				2200 * time.Millisecond,
				2300 * time.Millisecond,
				2400 * time.Millisecond,
			},
			want: Stats{
				Min:    2 * time.Second,
				Max:    2400 * time.Millisecond,
				Avg:    2200 * time.Millisecond,
				Median: 2200 * time.Millisecond,
				P95:    2380 * time.Millisecond,
				P99:    2396 * time.Millisecond,
				StdDev: 141421356 * time.Nanosecond, // ~141ms
			},
			scenario: "Well-optimized deployment with consistent startup times",
		},
		{
			name: "single pod",
			durations: []time.Duration{
				30 * time.Second,
			},
			want: Stats{
				Min:    30 * time.Second,
				Max:    30 * time.Second,
				Avg:    30 * time.Second,
				Median: 30 * time.Second,
				P95:    30 * time.Second,
				P99:    30 * time.Second,
				StdDev: 0,
			},
			scenario: "Single pod deployment or testing scenario",
		},
		{
			name:      "empty deployment",
			durations: []time.Duration{},
			want: Stats{
				Min:    0,
				Max:    0,
				Avg:    0,
				Median: 0,
				P95:    0,
				P99:    0,
				StdDev: 0,
			},
			scenario: "No pods found or all pods filtered out",
		},
		{
			name: "large deployment with various startup times",
			durations: func() []time.Duration {
				// Simulate 100 pods with varying startup times
				durations := make([]time.Duration, 100)
				for i := 0; i < 80; i++ {
					// 80% of pods start between 5-10 seconds
					durations[i] = time.Duration(5+i%5) * time.Second
				}
				for i := 80; i < 95; i++ {
					// 15% of pods start between 15-25 seconds
					durations[i] = time.Duration(15+(i-80)) * time.Second
				}
				for i := 95; i < 100; i++ {
					// 5% outliers taking 60+ seconds
					durations[i] = time.Duration(60+(i-95)*10) * time.Second
				}
				return durations
			}(),
			want: Stats{
				Min:    5 * time.Second,
				Max:    100 * time.Second,
				Avg:    12900 * time.Millisecond, // ~12.9s
				Median: 8 * time.Second,
				P95:    30549999999 * time.Nanosecond, // ~30.55s
				P99:    90099999999 * time.Nanosecond, // ~90.1s
				StdDev: 16727 * time.Millisecond,      // ~16.73s based on the distribution
			},
			scenario: "Large production deployment with some problematic pods",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateStats(tt.durations)

			// For exact values
			assert.Equal(t, tt.want.Min, got.Min, "Min should match - %s", tt.scenario)
			assert.Equal(t, tt.want.Max, got.Max, "Max should match - %s", tt.scenario)
			assert.Equal(t, tt.want.Median, got.Median, "Median should match - %s", tt.scenario)

			// For calculated values, allow small tolerance due to floating point
			if len(tt.durations) > 0 {
				assert.InDelta(t, tt.want.Avg, got.Avg, float64(time.Millisecond),
					"Average should be within 1ms - %s", tt.scenario)
				assert.InDelta(t, tt.want.StdDev, got.StdDev, float64(time.Millisecond),
					"StdDev should be within 1ms - %s", tt.scenario)
			}

			// P95 and P99 might vary slightly based on implementation
			assert.InDelta(t, tt.want.P95, got.P95, float64(100*time.Microsecond), "P95 should match - %s", tt.scenario)
			assert.InDelta(t, tt.want.P99, got.P99, float64(100*time.Microsecond), "P99 should match - %s", tt.scenario)
		})
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name     string
		values   []time.Duration
		p        float64
		want     time.Duration
		scenario string
	}{
		{
			name: "P50 median of odd count",
			values: []time.Duration{
				1 * time.Second,
				2 * time.Second,
				3 * time.Second,
				4 * time.Second,
				5 * time.Second,
			},
			p:        50,
			want:     3 * time.Second,
			scenario: "Should return middle value for odd count",
		},
		{
			name: "P50 median of even count",
			values: []time.Duration{
				1 * time.Second,
				2 * time.Second,
				3 * time.Second,
				4 * time.Second,
			},
			p:        50,
			want:     2500 * time.Millisecond, // Average of 2s and 3s
			scenario: "Should interpolate between middle values for even count",
		},
		{
			name: "P95 of real pod startup times",
			values: []time.Duration{
				5 * time.Second, // Fast pods
				6 * time.Second,
				7 * time.Second,
				8 * time.Second,
				9 * time.Second, // Normal pods
				10 * time.Second,
				12 * time.Second,
				15 * time.Second,
				20 * time.Second, // Slower pods
				60 * time.Second, // Outlier
			},
			p:        95,
			want:     41999999999 * time.Nanosecond, // ~42s - interpolated between 20s and 60s
			scenario: "P95 should be interpolated correctly for 10 values",
		},
		{
			name: "P99 edge case",
			values: []time.Duration{
				1 * time.Second,
			},
			p:        99,
			want:     1 * time.Second,
			scenario: "Single value should return itself for any percentile",
		},
		{
			name:     "empty slice",
			values:   []time.Duration{},
			p:        50,
			want:     0,
			scenario: "Empty data should return 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.values, tt.p)
			assert.Equal(t, tt.want, got, tt.scenario)
		})
	}
}

func TestStatsForDeploymentAnalysis(t *testing.T) {
	// Fake real-world scenarios: analyzing a deployment with varying pod performance
	podStartupTimes := []time.Duration{
		// Node 1: Fast SSD, good CPU
		3 * time.Second,
		3500 * time.Millisecond,
		4 * time.Second,

		// Node 2: Slower disk
		8 * time.Second,
		9 * time.Second,
		10 * time.Second,

		// Node 3: Resource contention
		15 * time.Second,
		18 * time.Second,
		20 * time.Second,

		// Node 4: Major issues
		45 * time.Second,
		50 * time.Second,
		120 * time.Second, // Image pull timeout
	}

	stats := CalculateStats(podStartupTimes)

	// Verify the stats provide meaningful insights
	assert.Less(t, stats.Min, 5*time.Second, "Fastest pods should start under 5s")
	assert.Greater(t, stats.Max, 100*time.Second, "Slowest pod indicates serious issues")
	assert.Less(t, stats.Median, stats.Avg, "Median < Average indicates outliers skewing the data")
	assert.Less(t, stats.P95, stats.Max, "P95 should exclude extreme outliers")
	assert.Greater(t, stats.StdDev, 20*time.Second, "High StdDev indicates inconsistent performance")

	// These stats would trigger warnings in the real tool:
	varianceRatio := float64(stats.Max) / float64(stats.Min)
	assert.Greater(t, varianceRatio, 20.0, "High variance ratio should trigger investigation")
}

func BenchmarkCalculateStats(b *testing.B) {
	// Benchmark with realistic data sizes
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		durations := make([]time.Duration, size)
		for i := range durations {
			// Simulate varied startup times between 1s and 60s
			durations[i] = time.Duration(1+i%60) * time.Second
		}

		b.Run(fmt.Sprintf("pods_%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = CalculateStats(durations)
			}
		})
	}
}
