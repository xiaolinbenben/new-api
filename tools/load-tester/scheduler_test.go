package main

import "testing"

func TestSmoothWeightedSchedulerDistribution(t *testing.T) {
	t.Parallel()

	scheduler, err := newSmoothWeightedScheduler([]scenarioConfig{
		{ID: "a", Enabled: true, Weight: 5},
		{ID: "b", Enabled: true, Weight: 3},
		{ID: "c", Enabled: true, Weight: 1},
	})
	if err != nil {
		t.Fatalf("newSmoothWeightedScheduler() error = %v", err)
	}

	counts := map[int]int{}
	for i := 0; i < 90; i++ {
		index, ok := scheduler.NextIndex()
		if !ok {
			t.Fatalf("scheduler returned no item at iteration %d", i)
		}
		counts[index]++
	}

	if counts[0] != 50 || counts[1] != 30 || counts[2] != 10 {
		t.Fatalf("unexpected distribution: %#v", counts)
	}
}
