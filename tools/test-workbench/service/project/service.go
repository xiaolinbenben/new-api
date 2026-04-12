package project

import (
	"context"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
	"github.com/QuantumNous/new-api/tools/test-workbench/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Bootstrap(ctx context.Context) error {
	if err := s.MarkStaleRunsAborted(ctx); err != nil {
		return err
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&model.ProjectModel{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.seedDefault(ctx)
}

func (s *Service) MarkStaleRunsAborted(ctx context.Context) error {
	var runs []model.RunModel
	if err := s.db.WithContext(ctx).
		Where("status IN ?", []string{"queued", "running"}).
		Find(&runs).Error; err != nil {
		return err
	}
	if len(runs) == 0 {
		return nil
	}
	now := time.Now()
	for _, item := range runs {
		summary := domain.RunSummary{RunID: item.ID, RunStatus: "aborted", StartedAt: item.StartedAt, FinishedAt: now}
		if strings.TrimSpace(item.SummaryJSON) != "" {
			_ = domain.UnmarshalString(item.SummaryJSON, &summary)
			summary.RunStatus = "aborted"
			summary.FinishedAt = now
		}
		summaryJSON, err := domain.MarshalToString(summary)
		if err != nil {
			return err
		}
		if err := s.db.WithContext(ctx).Model(&model.RunModel{}).
			Where("id = ?", item.ID).
			Updates(map[string]any{
				"status":       "aborted",
				"finished_at":  now,
				"summary_json": summaryJSON,
				"updated_at":   now,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListProjects(ctx context.Context) ([]domain.Project, error) {
	var items []model.ProjectModel
	if err := s.db.WithContext(ctx).Order("created_at asc").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Project, 0, len(items))
	for _, item := range items {
		out = append(out, toProject(item))
	}
	return out, nil
}

func (s *Service) GetProject(ctx context.Context, id string) (domain.Project, error) {
	var item model.ProjectModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.Project{}, err
	}
	return toProject(item), nil
}

func (s *Service) CreateProject(ctx context.Context, input domain.Project) (domain.Project, error) {
	now := time.Now()
	item := model.ProjectModel{
		ID:          uuid.NewString(),
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		IsDefault:   input.IsDefault,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if item.Name == "" {
		item.Name = "新项目"
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return domain.Project{}, err
	}
	return toProject(item), nil
}

func (s *Service) UpdateProject(ctx context.Context, id string, input domain.Project) (domain.Project, error) {
	var item model.ProjectModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.Project{}, err
	}
	item.Name = strings.TrimSpace(input.Name)
	item.Description = strings.TrimSpace(input.Description)
	item.IsDefault = input.IsDefault
	item.UpdatedAt = time.Now()
	if item.Name == "" {
		item.Name = "未命名项目"
	}
	if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
		return domain.Project{}, err
	}
	return toProject(item), nil
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var runIDs []string
		if err := tx.Model(&model.RunModel{}).Where("project_id = ?", id).Pluck("id", &runIDs).Error; err != nil {
			return err
		}
		if len(runIDs) > 0 {
			if err := tx.Where("run_id IN ?", runIDs).Delete(&model.RunSampleModel{}).Error; err != nil {
				return err
			}
			if err := tx.Where("run_id IN ?", runIDs).Delete(&model.RunMetricModel{}).Error; err != nil {
				return err
			}
		}
		for _, stmt := range []struct {
			query string
			value any
			model any
		}{
			{"project_id = ?", id, &model.RunModel{}},
			{"project_id = ?", id, &model.EnvironmentModel{}},
			{"project_id = ?", id, &model.MockProfileModel{}},
			{"project_id = ?", id, &model.RunProfileModel{}},
			{"project_id = ?", id, &model.ScenarioModel{}},
			{"id = ?", id, &model.ProjectModel{}},
		} {
			if err := tx.Where(stmt.query, stmt.value).Delete(stmt.model).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ListEnvironments(ctx context.Context, projectID string) ([]domain.Environment, error) {
	var items []model.EnvironmentModel
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Environment, 0, len(items))
	for _, item := range items {
		converted, err := toEnvironment(item)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func (s *Service) GetEnvironment(ctx context.Context, id string) (domain.Environment, error) {
	var item model.EnvironmentModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.Environment{}, err
	}
	return toEnvironment(item)
}

func (s *Service) CreateEnvironment(ctx context.Context, projectID string, input domain.Environment) (domain.Environment, error) {
	input.ProjectID = projectID
	if err := domain.ValidateEnvironment(&input); err != nil {
		return domain.Environment{}, err
	}
	headersJSON, err := domain.MarshalToString(input.DefaultHeaders)
	if err != nil {
		return domain.Environment{}, err
	}
	now := time.Now()
	item := model.EnvironmentModel{
		ID:                   uuid.NewString(),
		ProjectID:            projectID,
		Name:                 input.Name,
		TargetType:           input.TargetType,
		ExternalBaseURL:      input.ExternalBaseURL,
		DefaultHeadersJSON:   headersJSON,
		InsecureSkipVerify:   input.InsecureSkipVerify,
		MockBindHost:         input.MockBindHost,
		MockPort:             input.MockPort,
		MockRequireAuth:      input.MockRequireAuth,
		MockAuthToken:        input.MockAuthToken,
		AutoStart:            input.AutoStart,
		DefaultMockProfileID: input.DefaultMockProfile,
		DefaultRunProfileID:  input.DefaultRunProfile,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return domain.Environment{}, err
	}
	return toEnvironment(item)
}

func (s *Service) UpdateEnvironment(ctx context.Context, id string, input domain.Environment) (domain.Environment, error) {
	var item model.EnvironmentModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.Environment{}, err
	}
	input.ProjectID = item.ProjectID
	if err := domain.ValidateEnvironment(&input); err != nil {
		return domain.Environment{}, err
	}
	headersJSON, err := domain.MarshalToString(input.DefaultHeaders)
	if err != nil {
		return domain.Environment{}, err
	}
	item.Name = input.Name
	item.TargetType = input.TargetType
	item.ExternalBaseURL = input.ExternalBaseURL
	item.DefaultHeadersJSON = headersJSON
	item.InsecureSkipVerify = input.InsecureSkipVerify
	item.MockBindHost = input.MockBindHost
	item.MockPort = input.MockPort
	item.MockRequireAuth = input.MockRequireAuth
	item.MockAuthToken = input.MockAuthToken
	item.AutoStart = input.AutoStart
	item.DefaultMockProfileID = input.DefaultMockProfile
	item.DefaultRunProfileID = input.DefaultRunProfile
	item.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
		return domain.Environment{}, err
	}
	return toEnvironment(item)
}

func (s *Service) DeleteEnvironment(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.EnvironmentModel{}).Error
}

func (s *Service) ListMockProfiles(ctx context.Context, projectID string) ([]domain.MockProfile, error) {
	var items []model.MockProfileModel
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]domain.MockProfile, 0, len(items))
	for _, item := range items {
		converted, err := toMockProfile(item)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func (s *Service) GetMockProfile(ctx context.Context, id string) (domain.MockProfile, error) {
	var item model.MockProfileModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.MockProfile{}, err
	}
	return toMockProfile(item)
}

func (s *Service) CreateMockProfile(ctx context.Context, projectID string, input domain.MockProfile) (domain.MockProfile, error) {
	if err := domain.ValidateMockProfileConfig(&input.Config); err != nil {
		return domain.MockProfile{}, err
	}
	configJSON, err := domain.MarshalToString(input.Config)
	if err != nil {
		return domain.MockProfile{}, err
	}
	now := time.Now()
	item := model.MockProfileModel{
		ID:         uuid.NewString(),
		ProjectID:  projectID,
		Name:       firstNonEmpty(strings.TrimSpace(input.Name), "默认 Mock 配置"),
		ConfigJSON: configJSON,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return domain.MockProfile{}, err
	}
	return toMockProfile(item)
}

func (s *Service) UpdateMockProfile(ctx context.Context, id string, input domain.MockProfile) (domain.MockProfile, error) {
	var item model.MockProfileModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.MockProfile{}, err
	}
	if err := domain.ValidateMockProfileConfig(&input.Config); err != nil {
		return domain.MockProfile{}, err
	}
	configJSON, err := domain.MarshalToString(input.Config)
	if err != nil {
		return domain.MockProfile{}, err
	}
	item.Name = firstNonEmpty(strings.TrimSpace(input.Name), item.Name)
	item.ConfigJSON = configJSON
	item.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
		return domain.MockProfile{}, err
	}
	return toMockProfile(item)
}

func (s *Service) DeleteMockProfile(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.MockProfileModel{}).Error
}

func (s *Service) ListRunProfiles(ctx context.Context, projectID string) ([]domain.RunProfile, error) {
	var items []model.RunProfileModel
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]domain.RunProfile, 0, len(items))
	for _, item := range items {
		converted, err := toRunProfile(item)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func (s *Service) GetRunProfile(ctx context.Context, id string) (domain.RunProfile, error) {
	var item model.RunProfileModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.RunProfile{}, err
	}
	return toRunProfile(item)
}

func (s *Service) CreateRunProfile(ctx context.Context, projectID string, input domain.RunProfile) (domain.RunProfile, error) {
	if err := domain.ValidateRunProfileConfig(&input.Config); err != nil {
		return domain.RunProfile{}, err
	}
	configJSON, err := domain.MarshalToString(input.Config)
	if err != nil {
		return domain.RunProfile{}, err
	}
	now := time.Now()
	item := model.RunProfileModel{
		ID:         uuid.NewString(),
		ProjectID:  projectID,
		Name:       firstNonEmpty(strings.TrimSpace(input.Name), "默认压测配置"),
		ConfigJSON: configJSON,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return domain.RunProfile{}, err
	}
	return toRunProfile(item)
}

func (s *Service) UpdateRunProfile(ctx context.Context, id string, input domain.RunProfile) (domain.RunProfile, error) {
	var item model.RunProfileModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.RunProfile{}, err
	}
	if err := domain.ValidateRunProfileConfig(&input.Config); err != nil {
		return domain.RunProfile{}, err
	}
	configJSON, err := domain.MarshalToString(input.Config)
	if err != nil {
		return domain.RunProfile{}, err
	}
	item.Name = firstNonEmpty(strings.TrimSpace(input.Name), item.Name)
	item.ConfigJSON = configJSON
	item.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
		return domain.RunProfile{}, err
	}
	return toRunProfile(item)
}

func (s *Service) DeleteRunProfile(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.RunProfileModel{}).Error
}

func (s *Service) ListScenarios(ctx context.Context, projectID string) ([]domain.Scenario, error) {
	var items []model.ScenarioModel
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Scenario, 0, len(items))
	for _, item := range items {
		converted, err := toScenario(item)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func (s *Service) CreateScenario(ctx context.Context, projectID string, input domain.Scenario) (domain.Scenario, error) {
	if err := domain.ValidateScenarioConfig(&input.Config); err != nil {
		return domain.Scenario{}, err
	}
	configJSON, err := domain.MarshalToString(input.Config)
	if err != nil {
		return domain.Scenario{}, err
	}
	now := time.Now()
	item := model.ScenarioModel{
		ID:         firstNonEmpty(strings.TrimSpace(input.Config.ID), uuid.NewString()),
		ProjectID:  projectID,
		Name:       firstNonEmpty(strings.TrimSpace(input.Name), input.Config.Name),
		Enabled:    input.Config.Enabled,
		Weight:     input.Config.Weight,
		Mode:       input.Config.Mode,
		Preset:     input.Config.Preset,
		ConfigJSON: configJSON,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return domain.Scenario{}, err
	}
	return toScenario(item)
}

func (s *Service) UpdateScenario(ctx context.Context, id string, input domain.Scenario) (domain.Scenario, error) {
	var item model.ScenarioModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.Scenario{}, err
	}
	if input.Config.ID == "" {
		input.Config.ID = item.ID
	}
	if err := domain.ValidateScenarioConfig(&input.Config); err != nil {
		return domain.Scenario{}, err
	}
	configJSON, err := domain.MarshalToString(input.Config)
	if err != nil {
		return domain.Scenario{}, err
	}
	item.Name = firstNonEmpty(strings.TrimSpace(input.Name), input.Config.Name)
	item.Enabled = input.Config.Enabled
	item.Weight = input.Config.Weight
	item.Mode = input.Config.Mode
	item.Preset = input.Config.Preset
	item.ConfigJSON = configJSON
	item.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
		return domain.Scenario{}, err
	}
	return toScenario(item)
}

func (s *Service) DeleteScenario(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.ScenarioModel{}).Error
}

func (s *Service) CreateRunRecord(ctx context.Context, projectID string, environmentID string, runProfileID string, cfg domain.RunExecutionConfig) (domain.RunRecord, error) {
	cfgJSON, err := domain.MarshalToString(cfg)
	if err != nil {
		return domain.RunRecord{}, err
	}
	now := time.Now()
	summary := domain.RunSummary{
		RunStatus:   "running",
		StartedAt:   now,
		StatusCodes: map[string]int64{},
		ErrorKinds:  map[string]int64{},
	}
	summaryJSON, err := domain.MarshalToString(summary)
	if err != nil {
		return domain.RunRecord{}, err
	}
	item := model.RunModel{
		ID:            uuid.NewString(),
		ProjectID:     projectID,
		EnvironmentID: environmentID,
		RunProfileID:  runProfileID,
		Status:        "running",
		ConfigJSON:    cfgJSON,
		SummaryJSON:   summaryJSON,
		StartedAt:     now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return domain.RunRecord{}, err
	}
	return s.GetRun(ctx, item.ID)
}

func (s *Service) FinalizeRun(ctx context.Context, record domain.RunRecord) error {
	cfgJSON, err := domain.MarshalToString(record.Config)
	if err != nil {
		return err
	}
	summaryJSON, err := domain.MarshalToString(record.Summary)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		runUpdate := map[string]any{
			"status":         record.Status,
			"config_json":    cfgJSON,
			"summary_json":   summaryJSON,
			"total_requests": record.Summary.TotalRequests,
			"successes":      record.Summary.Successes,
			"errors":         record.Summary.Errors,
			"timeouts":       record.Summary.Timeouts,
			"p95_ms":         record.Summary.P95MS,
			"started_at":     record.StartedAt,
			"updated_at":     time.Now(),
		}
		if record.FinishedAt != nil {
			runUpdate["finished_at"] = record.FinishedAt
		}
		if err := tx.Model(&model.RunModel{}).Where("id = ?", record.ID).Updates(runUpdate).Error; err != nil {
			return err
		}
		if err := tx.Where("run_id = ?", record.ID).Delete(&model.RunMetricModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("run_id = ?", record.ID).Delete(&model.RunSampleModel{}).Error; err != nil {
			return err
		}

		now := time.Now()
		summaryMetricJSON, err := domain.MarshalToString(record.Summary)
		if err != nil {
			return err
		}
		metrics := []model.RunMetricModel{{
			ID:         uuid.NewString(),
			RunID:      record.ID,
			MetricType: "summary",
			ScenarioID: "",
			MetricJSON: summaryMetricJSON,
			CreatedAt:  now,
		}}
		for _, scenario := range record.Scenarios {
			payload, err := domain.MarshalToString(scenario)
			if err != nil {
				return err
			}
			metrics = append(metrics, model.RunMetricModel{
				ID:         uuid.NewString(),
				RunID:      record.ID,
				MetricType: "scenario",
				ScenarioID: scenario.ScenarioID,
				MetricJSON: payload,
				CreatedAt:  now,
			})
		}
		if err := tx.Create(&metrics).Error; err != nil {
			return err
		}

		sampleRows := make([]model.RunSampleModel, 0, len(record.Samples.Requests)+len(record.Samples.Errors))
		for _, item := range record.Samples.Requests {
			data, err := domain.MarshalToString(item)
			if err != nil {
				return err
			}
			sampleRows = append(sampleRows, model.RunSampleModel{
				ID:         item.ID + "-req",
				RunID:      record.ID,
				Kind:       "request",
				SampleJSON: data,
				CreatedAt:  item.Timestamp,
			})
		}
		for _, item := range record.Samples.Errors {
			data, err := domain.MarshalToString(item)
			if err != nil {
				return err
			}
			sampleRows = append(sampleRows, model.RunSampleModel{
				ID:         item.ID + "-err",
				RunID:      record.ID,
				Kind:       "error",
				SampleJSON: data,
				CreatedAt:  item.Timestamp,
			})
		}
		if len(sampleRows) > 0 {
			if err := tx.Create(&sampleRows).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ListRuns(ctx context.Context, projectID string) ([]domain.RunListItem, error) {
	query := s.db.WithContext(ctx).Model(&model.RunModel{})
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	var items []model.RunModel
	if err := query.Order("started_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]domain.RunListItem, 0, len(items))
	for _, item := range items {
		out = append(out, domain.RunListItem{
			ID:            item.ID,
			ProjectID:     item.ProjectID,
			EnvironmentID: item.EnvironmentID,
			RunProfileID:  item.RunProfileID,
			Status:        item.Status,
			StartedAt:     item.StartedAt,
			FinishedAt:    item.FinishedAt,
			TotalRequests: item.TotalRequests,
			Successes:     item.Successes,
			Errors:        item.Errors,
			Timeouts:      item.Timeouts,
			P95MS:         item.P95MS,
		})
	}
	return out, nil
}

func (s *Service) GetRun(ctx context.Context, id string) (domain.RunRecord, error) {
	var item model.RunModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.RunRecord{}, err
	}
	record := domain.RunRecord{
		ID:            item.ID,
		ProjectID:     item.ProjectID,
		EnvironmentID: item.EnvironmentID,
		RunProfileID:  item.RunProfileID,
		Status:        item.Status,
		StartedAt:     item.StartedAt,
		FinishedAt:    item.FinishedAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
	if err := domain.UnmarshalString(item.ConfigJSON, &record.Config); err != nil {
		return domain.RunRecord{}, err
	}
	if err := domain.UnmarshalString(item.SummaryJSON, &record.Summary); err != nil {
		return domain.RunRecord{}, err
	}
	scenarios, err := s.GetRunScenarios(ctx, id)
	if err != nil {
		return domain.RunRecord{}, err
	}
	record.Scenarios = scenarios
	samples, err := s.GetRunSamples(ctx, id)
	if err != nil {
		return domain.RunRecord{}, err
	}
	record.Samples = samples
	return record, nil
}

func (s *Service) GetRunSummary(ctx context.Context, id string) (domain.RunSummary, error) {
	var item model.RunModel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return domain.RunSummary{}, err
	}
	var summary domain.RunSummary
	if err := domain.UnmarshalString(item.SummaryJSON, &summary); err != nil {
		return domain.RunSummary{}, err
	}
	return summary, nil
}

func (s *Service) GetRunScenarios(ctx context.Context, id string) ([]domain.ScenarioStatsItem, error) {
	var rows []model.RunMetricModel
	if err := s.db.WithContext(ctx).
		Where("run_id = ? AND metric_type = ?", id, "scenario").
		Order("created_at asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.ScenarioStatsItem, 0, len(rows))
	for _, row := range rows {
		var item domain.ScenarioStatsItem
		if err := domain.UnmarshalString(row.MetricJSON, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) GetRunSamples(ctx context.Context, id string) (domain.SamplesResponse, error) {
	var rows []model.RunSampleModel
	if err := s.db.WithContext(ctx).
		Where("run_id = ?", id).
		Order("created_at desc").
		Find(&rows).Error; err != nil {
		return domain.SamplesResponse{}, err
	}
	resp := domain.SamplesResponse{}
	for _, row := range rows {
		var item domain.SampleRecord
		if err := domain.UnmarshalString(row.SampleJSON, &item); err != nil {
			return domain.SamplesResponse{}, err
		}
		if row.Kind == "error" {
			resp.Errors = append(resp.Errors, item)
		} else {
			resp.Requests = append(resp.Requests, item)
		}
	}
	return resp, nil
}

func (s *Service) seedDefault(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		projectID := uuid.NewString()
		mockProfileID := uuid.NewString()
		runProfileID := uuid.NewString()
		envID := uuid.NewString()

		mockCfgJSON, err := domain.MarshalToString(domain.DefaultMockProfileConfig())
		if err != nil {
			return err
		}
		runCfgJSON, err := domain.MarshalToString(domain.DefaultRunProfileConfig())
		if err != nil {
			return err
		}
		headersJSON, err := domain.MarshalToString(domain.DefaultEnvironment().DefaultHeaders)
		if err != nil {
			return err
		}

		records := []any{
			&model.ProjectModel{
				ID:          projectID,
				Name:        "默认项目",
				Description: "由测试工作台初始化并管理",
				IsDefault:   true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			&model.MockProfileModel{
				ID:         mockProfileID,
				ProjectID:  projectID,
				Name:       "默认 Mock 配置",
				ConfigJSON: mockCfgJSON,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
			&model.RunProfileModel{
				ID:         runProfileID,
				ProjectID:  projectID,
				Name:       "默认压测配置",
				ConfigJSON: runCfgJSON,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
			&model.EnvironmentModel{
				ID:                   envID,
				ProjectID:            projectID,
				Name:                 domain.DefaultEnvironment().Name,
				TargetType:           domain.DefaultEnvironment().TargetType,
				ExternalBaseURL:      "",
				DefaultHeadersJSON:   headersJSON,
				InsecureSkipVerify:   false,
				MockBindHost:         domain.DefaultEnvironment().MockBindHost,
				MockPort:             domain.DefaultEnvironment().MockPort,
				MockRequireAuth:      domain.DefaultEnvironment().MockRequireAuth,
				MockAuthToken:        domain.DefaultEnvironment().MockAuthToken,
				AutoStart:            true,
				DefaultMockProfileID: mockProfileID,
				DefaultRunProfileID:  runProfileID,
				CreatedAt:            now,
				UpdatedAt:            now,
			},
		}
		for _, record := range records {
			if err := tx.Create(record).Error; err != nil {
				return err
			}
		}
		for _, scenario := range domain.DefaultScenarioConfigs() {
			if err := domain.ValidateScenarioConfig(&scenario); err != nil {
				return err
			}
			configJSON, err := domain.MarshalToString(scenario)
			if err != nil {
				return err
			}
			row := model.ScenarioModel{
				ID:         scenario.ID,
				ProjectID:  projectID,
				Name:       scenario.Name,
				Enabled:    scenario.Enabled,
				Weight:     scenario.Weight,
				Mode:       scenario.Mode,
				Preset:     scenario.Preset,
				ConfigJSON: configJSON,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func toProject(item model.ProjectModel) domain.Project {
	return domain.Project{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		IsDefault:   item.IsDefault,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toEnvironment(item model.EnvironmentModel) (domain.Environment, error) {
	out := domain.Environment{
		ID:                 item.ID,
		ProjectID:          item.ProjectID,
		Name:               item.Name,
		TargetType:         item.TargetType,
		ExternalBaseURL:    item.ExternalBaseURL,
		InsecureSkipVerify: item.InsecureSkipVerify,
		MockBindHost:       item.MockBindHost,
		MockPort:           item.MockPort,
		MockRequireAuth:    item.MockRequireAuth,
		MockAuthToken:      item.MockAuthToken,
		AutoStart:          item.AutoStart,
		DefaultMockProfile: item.DefaultMockProfileID,
		DefaultRunProfile:  item.DefaultRunProfileID,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
	if err := domain.UnmarshalString(item.DefaultHeadersJSON, &out.DefaultHeaders); err != nil {
		return domain.Environment{}, err
	}
	return out, nil
}

func toMockProfile(item model.MockProfileModel) (domain.MockProfile, error) {
	out := domain.MockProfile{
		ID:        item.ID,
		ProjectID: item.ProjectID,
		Name:      item.Name,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
	if err := domain.UnmarshalString(item.ConfigJSON, &out.Config); err != nil {
		return domain.MockProfile{}, err
	}
	return out, nil
}

func toRunProfile(item model.RunProfileModel) (domain.RunProfile, error) {
	out := domain.RunProfile{
		ID:        item.ID,
		ProjectID: item.ProjectID,
		Name:      item.Name,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
	if err := domain.UnmarshalString(item.ConfigJSON, &out.Config); err != nil {
		return domain.RunProfile{}, err
	}
	return out, nil
}

func toScenario(item model.ScenarioModel) (domain.Scenario, error) {
	out := domain.Scenario{
		ID:        item.ID,
		ProjectID: item.ProjectID,
		Name:      item.Name,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
	if err := domain.UnmarshalString(item.ConfigJSON, &out.Config); err != nil {
		return domain.Scenario{}, err
	}
	return out, nil
}

func firstNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
