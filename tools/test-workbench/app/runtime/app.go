package runtime

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/QuantumNous/new-api/tools/test-workbench/controller"
	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
	"github.com/QuantumNous/new-api/tools/test-workbench/model"
	"github.com/QuantumNous/new-api/tools/test-workbench/router"
	"github.com/QuantumNous/new-api/tools/test-workbench/service/loadtest"
	"github.com/QuantumNous/new-api/tools/test-workbench/service/mock"
	"github.com/QuantumNous/new-api/tools/test-workbench/service/project"
	runtimesvc "github.com/QuantumNous/new-api/tools/test-workbench/service/runtime"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Config struct {
	AdminHost    string
	AdminPort    int
	DataDir      string
	DatabasePath string
}

type App struct {
	cfg      Config
	server   *http.Server
	projects *project.Service
	mocks    *mock.Service
	loads    *loadtest.Service
	runtime  *runtimesvc.Service
}

func New(cfg Config, uiFS fs.FS) (*App, error) {
	if cfg.AdminHost == "" {
		cfg.AdminHost = domain.DefaultAdminHost
	}
	if cfg.AdminPort == 0 {
		cfg.AdminPort = domain.DefaultAdminPort
	}
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Clean("./tools/test-workbench/data")
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = filepath.Join(cfg.DataDir, "workbench.db")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(cfg.DatabasePath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(
		&model.ProjectModel{},
		&model.EnvironmentModel{},
		&model.MockProfileModel{},
		&model.RunProfileModel{},
		&model.ScenarioModel{},
		&model.RunModel{},
		&model.RunSampleModel{},
		&model.RunMetricModel{},
	); err != nil {
		return nil, err
	}

	projectService := project.New(db)
	if err := projectService.Bootstrap(context.Background()); err != nil {
		return nil, err
	}

	mockService := mock.NewService()
	loadService := loadtest.NewService(projectService)
	runtimeService := runtimesvc.New(projectService, mockService, loadService)
	adminController := controller.NewAdminController(projectService, runtimeService)
	handler := router.New(adminController, uiFS)

	return &App{
		cfg:      cfg,
		server:   &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.AdminHost, cfg.AdminPort), Handler: handler, ReadHeaderTimeout: 5 * time.Second},
		projects: projectService,
		mocks:    mockService,
		loads:    loadService,
		runtime:  runtimeService,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	if err := a.runtime.StartAutoMockListeners(ctx); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		err := a.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	fmt.Printf("\n测试工作台已启动\n")
	fmt.Printf("管理端界面: http://%s:%d\n\n", a.cfg.AdminHost, a.cfg.AdminPort)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.runtime.StopAll(shutdownCtx)
		return a.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
