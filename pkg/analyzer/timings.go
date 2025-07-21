package analyzer

import (
	"sort"

	"github.com/px4n/bootscope/pkg/types"
)

// fixPhaseTimings ensures phases are properly ordered and don't overlap
func (a *Analyzer) fixPhaseTimings(phases []types.PhaseInfo) []types.PhaseInfo {
	if len(phases) == 0 {
		return phases
	}

	// Sort phases by start time
	sort.Slice(phases, func(i, j int) bool {
		return phases[i].StartTime.Before(phases[j].StartTime)
	})

	// Fix overlapping phases
	for i := 1; i < len(phases); i++ {
		// If current phase starts before previous phase ends, adjust it
		if phases[i].StartTime.Before(phases[i-1].EndTime) {
			phases[i].StartTime = phases[i-1].EndTime
			phases[i].Duration = phases[i].EndTime.Sub(phases[i].StartTime)

			// If duration becomes negative, set to zero
			if phases[i].Duration < 0 {
				phases[i].Duration = 0
			}
		}
	}

	return phases
}

// consolidateDuplicatePhases merges duplicate phases (e.g., multiple image pulls)
func (a *Analyzer) consolidateDuplicatePhases(phases []types.PhaseInfo) []types.PhaseInfo {
	if len(phases) == 0 {
		return phases
	}

	// Sort phases by start time first
	sort.Slice(phases, func(i, j int) bool {
		return phases[i].StartTime.Before(phases[j].StartTime)
	})

	// Find if there are multiple container creation phases (indicating restarts)
	var containerCreations []types.PhaseInfo
	for _, phase := range phases {
		if phase.Phase == types.StartupPhaseContainerCreate {
			containerCreations = append(containerCreations, phase)
		}
	}

	// If we have multiple container creations, keep only phases after the last one
	if len(containerCreations) > 1 {
		lastRestartTime := containerCreations[len(containerCreations)-1].StartTime
		var recentPhases []types.PhaseInfo
		for _, phase := range phases {
			if phase.StartTime.After(lastRestartTime) || phase.StartTime.Equal(lastRestartTime) {
				recentPhases = append(recentPhases, phase)
			}
		}
		phases = recentPhases
	}

	// Now consolidate phases of the same type
	phaseMap := make(map[types.StartupPhase][]types.PhaseInfo)
	for _, phase := range phases {
		phaseMap[phase.Phase] = append(phaseMap[phase.Phase], phase)
	}

	var result []types.PhaseInfo
	for _, phaseList := range phaseMap {
		if len(phaseList) == 1 {
			result = append(result, phaseList[0])
		} else if len(phaseList) > 1 {
			// Use the most recent phase
			sort.Slice(phaseList, func(i, j int) bool {
				return phaseList[i].StartTime.After(phaseList[j].StartTime)
			})
			result = append(result, phaseList[0])
		}
	}

	// Sort result by start time
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.Before(result[j].StartTime)
	})

	return result
}
