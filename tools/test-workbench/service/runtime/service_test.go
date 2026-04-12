package runtime

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
	"github.com/QuantumNous/new-api/tools/test-workbench/model"
	"github.com/QuantumNous/new-api/tools/test-workbench/service/loadtest"
	"github.com/QuantumNous/new-api/tools/test-workbench/service/mock"
	"github.com/QuantumNous/new-api/tools/test-workbench/service/project"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newRuntimeService(t *testing.T) (*project.Service, *Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "runtime.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.ProjectModel{},
		&model.EnvironmentModel{},
		&model.MockProfileModel{},
		&model.RunProfileModel{},
		&model.ScenarioModel{},
		&model.RunModel{},
		&model.RunSampleModel{},
		&model.RunMetricModel{},
	))

	projects := project.New(db)
	require.NoError(t, projects.Bootstrap(context.Background()))
	mocks := mock.NewService()
	loads := loadtest.NewService(projects)
	return projects, New(projects, mocks, loads)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestRuntimeStartsMockAndRun(t *testing.T) {
	t.Parallel()

	projects, runtime := newRuntimeService(t)
	projectList, err := projects.ListProjects(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, projectList)
	projectID := projectList[0].ID

	environments, err := projects.ListEnvironments(context.Background(), projectID)
	require.NoError(t, err)
	require.Len(t, environments, 1)
	env := environments[0]
	env.MockPort = freePort(t)
	updatedEnv, err := projects.UpdateEnvironment(context.Background(), env.ID, env)
	require.NoError(t, err)

	runProfiles, err := projects.ListRunProfiles(context.Background(), projectID)
	require.NoError(t, err)
	require.NotEmpty(t, runProfiles)
	runProfile := runProfiles[0]
	config := domain.DefaultRunProfileConfig()
	config.TotalConcurrency = 1
	config.DurationSec = 1
	config.MaxRequests = 1
	runProfile.Config = config
	runProfile, err = projects.UpdateRunProfile(context.Background(), runProfile.ID, runProfile)
	require.NoError(t, err)

	scenarios, err := projects.ListScenarios(context.Background(), projectID)
	require.NoError(t, err)
	for _, scenario := range scenarios {
		cfg := scenario.Config
		cfg.Enabled = false
		scenario.Config = cfg
		_, err = projects.UpdateScenario(context.Background(), scenario.ID, scenario)
		require.NoError(t, err)
	}

	_, err = projects.CreateScenario(context.Background(), projectID, domain.Scenario{
		Name: "Models",
		Config: domain.ScenarioConfig{
			ID:               "models-single",
			Name:             "Models",
			Enabled:          true,
			Preset:           "raw_http",
			Mode:             "single",
			Weight:           1,
			Method:           "GET",
			Path:             "/v1/models",
			Headers:          map[string]string{},
			Body:             "",
			ExpectedStatuses: []int{200},
		},
	})
	require.NoError(t, err)

	mockRuntime, err := runtime.StartMockListener(context.Background(), updatedEnv.ID, updatedEnv.DefaultMockProfile)
	require.NoError(t, err)
	require.True(t, mockRuntime.Healthy)

	record, err := runtime.StartRun(context.Background(), projectID, updatedEnv.ID, runProfile.ID)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(runtime.ListActiveRuns()) == 0
	}, 8*time.Second, 100*time.Millisecond)

	saved, err := projects.GetRun(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", saved.Status)
	require.GreaterOrEqual(t, saved.Summary.TotalRequests, int64(1))
	require.Equal(t, int64(0), saved.Summary.Errors)
}
