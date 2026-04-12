package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

type historyStore struct {
	dir string
	mu  sync.Mutex
}

func newHistoryStore(dir string) *historyStore {
	return &historyStore{dir: dir}
}

func (s *historyStore) save(record runRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := common.Marshal(record)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, record.RunID+".json"), data, 0o644)
}

func (s *historyStore) list() (historyListResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return historyListResponse{}, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return historyListResponse{}, err
	}
	items := make([]historyItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		record, err := s.readLocked(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		items = append(items, historyItem{
			RunID:         record.RunID,
			RunStatus:     record.RunStatus,
			StartedAt:     record.StartedAt,
			FinishedAt:    record.FinishedAt,
			TotalRequests: record.Summary.TotalRequests,
			Successes:     record.Summary.Successes,
			Errors:        record.Summary.Errors,
			Timeouts:      record.Summary.Timeouts,
			P95MS:         record.Summary.P95MS,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].StartedAt.After(items[j].StartedAt)
	})
	return historyListResponse{Items: items}, nil
}

func (s *historyStore) get(runID string) (runRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked(runID)
}

func (s *historyStore) readLocked(runID string) (runRecord, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, runID+".json"))
	if err != nil {
		return runRecord{}, err
	}
	var record runRecord
	if err := common.Unmarshal(data, &record); err != nil {
		return runRecord{}, err
	}
	return record, nil
}
