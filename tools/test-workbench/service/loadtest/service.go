package loadtest

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
)

type RunStore interface {
	CreateRunRecord(ctx context.Context, projectID string, environmentID string, runProfileID string, cfg domain.RunExecutionConfig) (domain.RunRecord, error)
	FinalizeRun(ctx context.Context, record domain.RunRecord) error
}

type StartInput struct {
	ProjectID     string
	EnvironmentID string
	RunProfileID  string
	Config        domain.RunExecutionConfig
}

type Service struct {
	store RunStore

	mu              sync.RWMutex
	activeByID      map[string]*activeRun
	activeByProject map[string]*activeRun
}

type activeRun struct {
	projectID     string
	environmentID string
	runProfileID  string
	targetBaseURL string
	runner        *runner
}

func NewService(store RunStore) *Service {
	return &Service{
		store:           store,
		activeByID:      make(map[string]*activeRun),
		activeByProject: make(map[string]*activeRun),
	}
}

func (s *Service) Start(ctx context.Context, input StartInput) (domain.RunRecord, error) {
	s.mu.Lock()
	if _, ok := s.activeByProject[input.ProjectID]; ok {
		s.mu.Unlock()
		return domain.RunRecord{}, fmt.Errorf("project already has an active run")
	}
	s.mu.Unlock()

	record, err := s.store.CreateRunRecord(ctx, input.ProjectID, input.EnvironmentID, input.RunProfileID, input.Config)
	if err != nil {
		return domain.RunRecord{}, err
	}
	run, err := newRunner(record.ID, input.Config)
	if err != nil {
		return domain.RunRecord{}, err
	}
	active := &activeRun{
		projectID:     input.ProjectID,
		environmentID: input.EnvironmentID,
		runProfileID:  input.RunProfileID,
		targetBaseURL: input.Config.Target.BaseURL,
		runner:        run,
	}

	s.mu.Lock()
	s.activeByID[record.ID] = active
	s.activeByProject[input.ProjectID] = active
	s.mu.Unlock()

	run.start(context.Background())

	go func() {
		<-run.Done()
		finalRecord := run.buildRecord(input.ProjectID, input.EnvironmentID, input.RunProfileID)
		_ = s.store.FinalizeRun(context.Background(), finalRecord)

		s.mu.Lock()
		delete(s.activeByID, record.ID)
		if current, ok := s.activeByProject[input.ProjectID]; ok && current == active {
			delete(s.activeByProject, input.ProjectID)
		}
		s.mu.Unlock()
	}()

	return s.Get(record.ID)
}

func (s *Service) Stop(ctx context.Context, runID string) error {
	s.mu.RLock()
	active, ok := s.activeByID[runID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("run not found")
	}
	active.runner.stop()
	return nil
}

func (s *Service) StopAll(ctx context.Context) error {
	s.mu.RLock()
	items := make([]*activeRun, 0, len(s.activeByID))
	for _, item := range s.activeByID {
		items = append(items, item)
	}
	s.mu.RUnlock()
	for _, item := range items {
		item.runner.stop()
	}
	return nil
}

func (s *Service) Get(runID string) (domain.RunRecord, error) {
	s.mu.RLock()
	active, ok := s.activeByID[runID]
	s.mu.RUnlock()
	if !ok {
		return domain.RunRecord{}, fmt.Errorf("active run not found")
	}
	return active.runner.buildRecord(active.projectID, active.environmentID, active.runProfileID), nil
}

func (s *Service) Active(runID string) (domain.LoadRunRuntime, bool) {
	s.mu.RLock()
	active, ok := s.activeByID[runID]
	s.mu.RUnlock()
	if !ok {
		return domain.LoadRunRuntime{}, false
	}
	return active.runtime(), true
}

func (s *Service) List() []domain.LoadRunRuntime {
	s.mu.RLock()
	items := make([]domain.LoadRunRuntime, 0, len(s.activeByID))
	for _, item := range s.activeByID {
		items = append(items, item.runtime())
	}
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].RunID < items[j].RunID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})
	return items
}

func (a *activeRun) runtime() domain.LoadRunRuntime {
	return domain.LoadRunRuntime{
		RunID:         a.runner.id,
		ProjectID:     a.projectID,
		EnvironmentID: a.environmentID,
		RunProfileID:  a.runProfileID,
		Status:        a.runner.status(),
		StartedAt:     a.runner.startedAt,
		TargetBaseURL: a.targetBaseURL,
		Summary:       a.runner.summary(),
		Samples:       a.runner.samplesSnapshot(),
	}
}
