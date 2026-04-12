package project

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
	"github.com/QuantumNous/new-api/tools/test-workbench/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestProjectService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "workbench.db")), &gorm.Config{})
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
	return New(db)
}

func TestBootstrapSeedsDefaultProject(t *testing.T) {
	t.Parallel()

	service := newTestProjectService(t)
	err := service.Bootstrap(context.Background())
	require.NoError(t, err)

	projects, err := service.ListProjects(context.Background())
	require.NoError(t, err)
	require.Len(t, projects, 1)
	require.Equal(t, "默认项目", projects[0].Name)

	environments, err := service.ListEnvironments(context.Background(), projects[0].ID)
	require.NoError(t, err)
	require.Len(t, environments, 1)
	require.Equal(t, domain.TargetTypeInternalMock, environments[0].TargetType)

	scenarios, err := service.ListScenarios(context.Background(), projects[0].ID)
	require.NoError(t, err)
	require.NotEmpty(t, scenarios)
}

func TestMarkStaleRunsAborted(t *testing.T) {
	t.Parallel()

	service := newTestProjectService(t)
	err := service.Bootstrap(context.Background())
	require.NoError(t, err)

	record, err := service.CreateRunRecord(context.Background(), "project", "environment", "profile", domain.RunExecutionConfig{
		Target: domain.RuntimeLoadTarget{BaseURL: "http://127.0.0.1:9999", Headers: map[string]string{}},
		Run:    domain.DefaultRunProfileConfig(),
	})
	require.NoError(t, err)

	require.NoError(t, service.MarkStaleRunsAborted(context.Background()))
	updated, err := service.GetRun(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, "aborted", updated.Status)
	require.Equal(t, "aborted", updated.Summary.RunStatus)
	require.WithinDuration(t, time.Now(), *updated.FinishedAt, 2*time.Second)
}
