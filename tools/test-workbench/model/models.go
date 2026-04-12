package model

import "time"

type ProjectModel struct {
	ID          string    `gorm:"type:varchar(64);primaryKey"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text;not null;default:''"`
	IsDefault   bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (ProjectModel) TableName() string {
	return "tool_projects"
}

type EnvironmentModel struct {
	ID                   string    `gorm:"type:varchar(64);primaryKey"`
	ProjectID            string    `gorm:"type:varchar(64);index;not null"`
	Name                 string    `gorm:"type:varchar(255);not null"`
	TargetType           string    `gorm:"type:varchar(64);not null"`
	ExternalBaseURL      string    `gorm:"type:text;not null;default:''"`
	DefaultHeadersJSON   string    `gorm:"type:text;not null;default:'{}'"`
	InsecureSkipVerify   bool      `gorm:"not null;default:false"`
	MockBindHost         string    `gorm:"type:varchar(255);not null;default:'0.0.0.0'"`
	MockPort             int       `gorm:"not null"`
	MockRequireAuth      bool      `gorm:"not null;default:false"`
	MockAuthToken        string    `gorm:"type:varchar(255);not null;default:''"`
	AutoStart            bool      `gorm:"not null;default:false"`
	DefaultMockProfileID string    `gorm:"type:varchar(64);not null;default:''"`
	DefaultRunProfileID  string    `gorm:"type:varchar(64);not null;default:''"`
	CreatedAt            time.Time `gorm:"not null"`
	UpdatedAt            time.Time `gorm:"not null"`
}

func (EnvironmentModel) TableName() string {
	return "tool_environments"
}

type MockProfileModel struct {
	ID         string    `gorm:"type:varchar(64);primaryKey"`
	ProjectID  string    `gorm:"type:varchar(64);index;not null"`
	Name       string    `gorm:"type:varchar(255);not null"`
	ConfigJSON string    `gorm:"type:text;not null"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (MockProfileModel) TableName() string {
	return "tool_mock_profiles"
}

type RunProfileModel struct {
	ID         string    `gorm:"type:varchar(64);primaryKey"`
	ProjectID  string    `gorm:"type:varchar(64);index;not null"`
	Name       string    `gorm:"type:varchar(255);not null"`
	ConfigJSON string    `gorm:"type:text;not null"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (RunProfileModel) TableName() string {
	return "tool_run_profiles"
}

type ScenarioModel struct {
	ID         string    `gorm:"type:varchar(64);primaryKey"`
	ProjectID  string    `gorm:"type:varchar(64);index;not null"`
	Name       string    `gorm:"type:varchar(255);not null"`
	Enabled    bool      `gorm:"not null;default:true"`
	Weight     int       `gorm:"not null;default:1"`
	Mode       string    `gorm:"type:varchar(64);not null"`
	Preset     string    `gorm:"type:varchar(128);not null;default:''"`
	ConfigJSON string    `gorm:"type:text;not null"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (ScenarioModel) TableName() string {
	return "tool_scenarios"
}

type RunModel struct {
	ID            string     `gorm:"type:varchar(64);primaryKey"`
	ProjectID     string     `gorm:"type:varchar(64);index;not null"`
	EnvironmentID string     `gorm:"type:varchar(64);index;not null"`
	RunProfileID  string     `gorm:"type:varchar(64);index;not null"`
	Status        string     `gorm:"type:varchar(64);index;not null"`
	ConfigJSON    string     `gorm:"type:text;not null"`
	SummaryJSON   string     `gorm:"type:text;not null"`
	TotalRequests int64      `gorm:"not null;default:0"`
	Successes     int64      `gorm:"not null;default:0"`
	Errors        int64      `gorm:"not null;default:0"`
	Timeouts      int64      `gorm:"not null;default:0"`
	P95MS         float64    `gorm:"not null;default:0"`
	StartedAt     time.Time  `gorm:"index;not null"`
	FinishedAt    *time.Time `gorm:"index"`
	CreatedAt     time.Time  `gorm:"not null"`
	UpdatedAt     time.Time  `gorm:"not null"`
}

func (RunModel) TableName() string {
	return "tool_runs"
}

type RunSampleModel struct {
	ID         string    `gorm:"type:varchar(96);primaryKey"`
	RunID      string    `gorm:"type:varchar(64);index;not null"`
	Kind       string    `gorm:"type:varchar(32);index;not null"`
	SampleJSON string    `gorm:"type:text;not null"`
	CreatedAt  time.Time `gorm:"index;not null"`
}

func (RunSampleModel) TableName() string {
	return "tool_run_samples"
}

type RunMetricModel struct {
	ID         string    `gorm:"type:varchar(96);primaryKey"`
	RunID      string    `gorm:"type:varchar(64);index;not null"`
	MetricType string    `gorm:"type:varchar(32);index;not null"`
	ScenarioID string    `gorm:"type:varchar(64);index;not null;default:''"`
	MetricJSON string    `gorm:"type:text;not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (RunMetricModel) TableName() string {
	return "tool_run_metrics"
}
