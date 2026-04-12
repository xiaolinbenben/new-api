package runtime

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
	"github.com/QuantumNous/new-api/tools/test-workbench/service/loadtest"
	"github.com/QuantumNous/new-api/tools/test-workbench/service/mock"
	"github.com/QuantumNous/new-api/tools/test-workbench/service/project"
)

type Service struct {
	projects *project.Service
	mocks    *mock.Service
	runs     *loadtest.Service
}

func New(projects *project.Service, mocks *mock.Service, runs *loadtest.Service) *Service {
	return &Service{projects: projects, mocks: mocks, runs: runs}
}

func (s *Service) StartAutoMockListeners(ctx context.Context) error {
	projects, err := s.projects.ListProjects(ctx)
	if err != nil {
		return err
	}
	for _, item := range projects {
		environments, err := s.projects.ListEnvironments(ctx, item.ID)
		if err != nil {
			return err
		}
		for _, env := range environments {
			if env.TargetType != domain.TargetTypeInternalMock || !env.AutoStart {
				continue
			}
			if _, err := s.StartMockListener(ctx, env.ID, env.DefaultMockProfile); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) StartMockListener(ctx context.Context, environmentID string, profileID string) (domain.MockListenerRuntime, error) {
	env, err := s.projects.GetEnvironment(ctx, environmentID)
	if err != nil {
		return domain.MockListenerRuntime{}, err
	}
	profile, err := s.resolveMockProfile(ctx, env.ProjectID, chooseID(profileID, env.DefaultMockProfile))
	if err != nil {
		return domain.MockListenerRuntime{}, err
	}
	return s.mocks.Start(ctx, env, profile)
}

func (s *Service) StopMockListener(ctx context.Context, environmentID string) error {
	return s.mocks.Stop(ctx, environmentID)
}

func (s *Service) ListMockListeners() []domain.MockListenerRuntime {
	return s.mocks.List()
}

func (s *Service) MockRoutes(environmentID string) ([]domain.RouteStatsItem, error) {
	return s.mocks.Routes(environmentID)
}

func (s *Service) MockEvents(environmentID string) (domain.EventsResponse, error) {
	return s.mocks.Events(environmentID)
}

func (s *Service) StartRun(ctx context.Context, projectID string, environmentID string, runProfileID string) (domain.RunRecord, error) {
	env, err := s.projects.GetEnvironment(ctx, environmentID)
	if err != nil {
		return domain.RunRecord{}, err
	}
	profile, err := s.resolveRunProfile(ctx, projectID, chooseID(runProfileID, env.DefaultRunProfile))
	if err != nil {
		return domain.RunRecord{}, err
	}
	scenarios, err := s.projects.ListScenarios(ctx, projectID)
	if err != nil {
		return domain.RunRecord{}, err
	}
	scenarioConfigs := make([]domain.ScenarioConfig, 0, len(scenarios))
	for _, item := range scenarios {
		scenarioConfigs = append(scenarioConfigs, item.Config)
	}
	cfg := domain.RunExecutionConfig{
		Target: domain.RuntimeLoadTarget{
			Headers:            domain.CloneStringMap(env.DefaultHeaders),
			InsecureSkipVerify: env.InsecureSkipVerify,
		},
		Run:       profile.Config,
		Scenarios: domain.CloneScenarioConfigs(scenarioConfigs),
	}
	switch env.TargetType {
	case domain.TargetTypeInternalMock:
		runtime, ok := s.mocks.Runtime(environmentID)
		if !ok || !runtime.Healthy {
			runtime, err = s.StartMockListener(ctx, environmentID, env.DefaultMockProfile)
			if err != nil {
				return domain.RunRecord{}, err
			}
		}
		cfg.Target.BaseURL = runtime.LocalBaseURL
	case domain.TargetTypeExternalHTTP:
		cfg.Target.BaseURL = env.ExternalBaseURL
	default:
		return domain.RunRecord{}, fmt.Errorf("unsupported target type: %s", env.TargetType)
	}
	return s.runs.Start(ctx, loadtest.StartInput{
		ProjectID:     projectID,
		EnvironmentID: environmentID,
		RunProfileID:  profile.ID,
		Config:        cfg,
	})
}

func (s *Service) StopRun(ctx context.Context, runID string) error {
	return s.runs.Stop(ctx, runID)
}

func (s *Service) ListActiveRuns() []domain.LoadRunRuntime {
	return s.runs.List()
}

func (s *Service) ActiveRun(runID string) (domain.LoadRunRuntime, bool) {
	return s.runs.Active(runID)
}

func (s *Service) ActiveRunRecord(runID string) (domain.RunRecord, bool) {
	record, err := s.runs.Get(runID)
	if err != nil {
		return domain.RunRecord{}, false
	}
	return record, true
}

func (s *Service) StopAll(ctx context.Context) error {
	if err := s.runs.StopAll(ctx); err != nil {
		return err
	}
	return s.mocks.StopAll(ctx)
}

func (s *Service) resolveMockProfile(ctx context.Context, projectID string, profileID string) (domain.MockProfile, error) {
	if profileID != "" {
		return s.projects.GetMockProfile(ctx, profileID)
	}
	items, err := s.projects.ListMockProfiles(ctx, projectID)
	if err != nil {
		return domain.MockProfile{}, err
	}
	if len(items) == 0 {
		return domain.MockProfile{}, fmt.Errorf("no mock profile found")
	}
	return items[0], nil
}

func (s *Service) resolveRunProfile(ctx context.Context, projectID string, runProfileID string) (domain.RunProfile, error) {
	if runProfileID != "" {
		return s.projects.GetRunProfile(ctx, runProfileID)
	}
	items, err := s.projects.ListRunProfiles(ctx, projectID)
	if err != nil {
		return domain.RunProfile{}, err
	}
	if len(items) == 0 {
		return domain.RunProfile{}, fmt.Errorf("no run profile found")
	}
	return items[0], nil
}

func chooseID(primary string, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
