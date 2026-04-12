package main

import (
	"fmt"
	"sync"
)

type scheduledScenario struct {
	Index         int
	ScenarioID    string
	Weight        int
	CurrentWeight int
}

type smoothWeightedScheduler struct {
	mu          sync.Mutex
	totalWeight int
	items       []scheduledScenario
}

func newSmoothWeightedScheduler(scenarios []scenarioConfig) (*smoothWeightedScheduler, error) {
	items := make([]scheduledScenario, 0, len(scenarios))
	total := 0
	for index, scenario := range scenarios {
		if !scenario.Enabled {
			continue
		}
		if scenario.Weight <= 0 {
			return nil, fmt.Errorf("scenario %q has invalid weight %d", scenario.ID, scenario.Weight)
		}
		total += scenario.Weight
		items = append(items, scheduledScenario{
			Index:      index,
			ScenarioID: scenario.ID,
			Weight:     scenario.Weight,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no enabled scenarios")
	}
	return &smoothWeightedScheduler{items: items, totalWeight: total}, nil
}

func (s *smoothWeightedScheduler) NextIndex() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 || s.totalWeight <= 0 {
		return 0, false
	}
	best := 0
	for i := range s.items {
		s.items[i].CurrentWeight += s.items[i].Weight
		if s.items[i].CurrentWeight > s.items[best].CurrentWeight {
			best = i
		}
	}
	s.items[best].CurrentWeight -= s.totalWeight
	return s.items[best].Index, true
}
