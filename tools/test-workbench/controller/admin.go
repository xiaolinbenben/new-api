package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
	projectsvc "github.com/QuantumNous/new-api/tools/test-workbench/service/project"
	runtimesvc "github.com/QuantumNous/new-api/tools/test-workbench/service/runtime"
)

type AdminController struct {
	projects *projectsvc.Service
	runtime  *runtimesvc.Service
}

func NewAdminController(projects *projectsvc.Service, runtime *runtimesvc.Service) *AdminController {
	return &AdminController{projects: projects, runtime: runtime}
}

func (c *AdminController) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects", c.handleProjects)
	mux.HandleFunc("POST /api/v1/projects", c.handleProjects)
	mux.HandleFunc("GET /api/v1/projects/{id}", c.handleProjectItem)
	mux.HandleFunc("PUT /api/v1/projects/{id}", c.handleProjectItem)
	mux.HandleFunc("DELETE /api/v1/projects/{id}", c.handleProjectItem)

	mux.HandleFunc("GET /api/v1/projects/{id}/environments", c.handleProjectEnvironments)
	mux.HandleFunc("POST /api/v1/projects/{id}/environments", c.handleProjectEnvironments)
	mux.HandleFunc("GET /api/v1/projects/{id}/mock-profiles", c.handleProjectMockProfiles)
	mux.HandleFunc("POST /api/v1/projects/{id}/mock-profiles", c.handleProjectMockProfiles)
	mux.HandleFunc("GET /api/v1/projects/{id}/run-profiles", c.handleProjectRunProfiles)
	mux.HandleFunc("POST /api/v1/projects/{id}/run-profiles", c.handleProjectRunProfiles)
	mux.HandleFunc("GET /api/v1/projects/{id}/scenarios", c.handleProjectScenarios)
	mux.HandleFunc("POST /api/v1/projects/{id}/scenarios", c.handleProjectScenarios)

	mux.HandleFunc("GET /api/v1/environments/{id}", c.handleEnvironmentItem)
	mux.HandleFunc("PUT /api/v1/environments/{id}", c.handleEnvironmentItem)
	mux.HandleFunc("DELETE /api/v1/environments/{id}", c.handleEnvironmentItem)
	mux.HandleFunc("GET /api/v1/mock-profiles/{id}", c.handleMockProfileItem)
	mux.HandleFunc("PUT /api/v1/mock-profiles/{id}", c.handleMockProfileItem)
	mux.HandleFunc("DELETE /api/v1/mock-profiles/{id}", c.handleMockProfileItem)
	mux.HandleFunc("GET /api/v1/run-profiles/{id}", c.handleRunProfileItem)
	mux.HandleFunc("PUT /api/v1/run-profiles/{id}", c.handleRunProfileItem)
	mux.HandleFunc("DELETE /api/v1/run-profiles/{id}", c.handleRunProfileItem)
	mux.HandleFunc("GET /api/v1/scenarios/{id}", c.handleScenarioItem)
	mux.HandleFunc("PUT /api/v1/scenarios/{id}", c.handleScenarioItem)
	mux.HandleFunc("DELETE /api/v1/scenarios/{id}", c.handleScenarioItem)

	mux.HandleFunc("GET /api/v1/runs", c.handleRuns)
	mux.HandleFunc("POST /api/v1/runs", c.handleRuns)
	mux.HandleFunc("GET /api/v1/runs/{id}", c.handleRunItem)
	mux.HandleFunc("GET /api/v1/runs/{id}/summary", c.handleRunSummary)
	mux.HandleFunc("GET /api/v1/runs/{id}/scenarios", c.handleRunScenarios)
	mux.HandleFunc("GET /api/v1/runs/{id}/samples", c.handleRunSamples)
	mux.HandleFunc("POST /api/v1/runs/{id}/stop", c.handleRunStop)

	mux.HandleFunc("GET /api/v1/runtime/mock-listeners", c.handleRuntimeMockListeners)
	mux.HandleFunc("POST /api/v1/runtime/mock-listeners", c.handleRuntimeMockListeners)
	mux.HandleFunc("POST /api/v1/runtime/mock-listeners/{environment_id}/stop", c.handleRuntimeMockStop)
	mux.HandleFunc("GET /api/v1/runtime/mock-listeners/{environment_id}/routes", c.handleRuntimeMockRoutes)
	mux.HandleFunc("GET /api/v1/runtime/mock-listeners/{environment_id}/events", c.handleRuntimeMockEvents)
	mux.HandleFunc("GET /api/v1/runtime/load-runs", c.handleRuntimeLoadRuns)
}

func (c *AdminController) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := c.projects.ListProjects(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var input domain.Project
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.CreateProject(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleProjectItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		item, err := c.projects.GetProject(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPut:
		var input domain.Project
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.UpdateProject(r.Context(), id, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := c.projects.DeleteProject(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleProjectEnvironments(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		items, err := c.projects.ListEnvironments(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var input domain.Environment
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.CreateEnvironment(r.Context(), projectID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleProjectMockProfiles(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		items, err := c.projects.ListMockProfiles(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var input domain.MockProfile
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.CreateMockProfile(r.Context(), projectID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleProjectRunProfiles(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		items, err := c.projects.ListRunProfiles(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var input domain.RunProfile
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.CreateRunProfile(r.Context(), projectID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleProjectScenarios(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		items, err := c.projects.ListScenarios(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var input domain.Scenario
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.CreateScenario(r.Context(), projectID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleEnvironmentItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		item, err := c.projects.GetEnvironment(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPut:
		var input domain.Environment
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.UpdateEnvironment(r.Context(), id, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := c.projects.DeleteEnvironment(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleMockProfileItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		item, err := c.projects.GetMockProfile(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPut:
		var input domain.MockProfile
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.UpdateMockProfile(r.Context(), id, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := c.projects.DeleteMockProfile(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleRunProfileItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		item, err := c.projects.GetRunProfile(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPut:
		var input domain.RunProfile
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.UpdateRunProfile(r.Context(), id, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := c.projects.DeleteRunProfile(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleScenarioItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		items, err := c.projects.ListProjects(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, project := range items {
			scenarios, err := c.projects.ListScenarios(r.Context(), project.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			for _, scenario := range scenarios {
				if scenario.ID == id {
					writeJSON(w, http.StatusOK, scenario)
					return
				}
			}
		}
		writeError(w, http.StatusNotFound, errors.New("scenario not found"))
	case http.MethodPut:
		var input domain.Scenario
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.projects.UpdateScenario(r.Context(), id, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := c.projects.DeleteScenario(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type createRunRequest struct {
	ProjectID     string `json:"project_id"`
	EnvironmentID string `json:"environment_id"`
	RunProfileID  string `json:"run_profile_id"`
}

func (c *AdminController) handleRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projectID := r.URL.Query().Get("project_id")
		items, err := c.projects.ListRuns(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		active := c.runtime.ListActiveRuns()
		activeMap := make(map[string]domain.LoadRunRuntime, len(active))
		for _, item := range active {
			activeMap[item.RunID] = item
		}
		for i := range items {
			if runtime, ok := activeMap[items[i].ID]; ok {
				items[i].Status = runtime.Status
				items[i].TotalRequests = runtime.Summary.TotalRequests
				items[i].Successes = runtime.Summary.Successes
				items[i].Errors = runtime.Summary.Errors
				items[i].Timeouts = runtime.Summary.Timeouts
				items[i].P95MS = runtime.Summary.P95MS
			}
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var req createRunRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		record, err := c.runtime.StartRun(r.Context(), req.ProjectID, req.EnvironmentID, req.RunProfileID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, record)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleRunItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	record, err := c.getRunRecord(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (c *AdminController) handleRunSummary(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if record, ok := c.runtime.ActiveRunRecord(id); ok {
		writeJSON(w, http.StatusOK, record.Summary)
		return
	}
	item, err := c.projects.GetRunSummary(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (c *AdminController) handleRunScenarios(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if record, ok := c.runtime.ActiveRunRecord(id); ok {
		writeJSON(w, http.StatusOK, record.Scenarios)
		return
	}
	items, err := c.projects.GetRunScenarios(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (c *AdminController) handleRunSamples(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if record, ok := c.runtime.ActiveRunRecord(id); ok {
		writeJSON(w, http.StatusOK, record.Samples)
		return
	}
	items, err := c.projects.GetRunSamples(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (c *AdminController) handleRunStop(w http.ResponseWriter, r *http.Request) {
	if err := c.runtime.StopRun(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

type startMockRequest struct {
	EnvironmentID string `json:"environment_id"`
	MockProfileID string `json:"mock_profile_id"`
}

func (c *AdminController) handleRuntimeMockListeners(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, c.runtime.ListMockListeners())
	case http.MethodPost:
		var req startMockRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item, err := c.runtime.StartMockListener(r.Context(), req.EnvironmentID, req.MockProfileID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *AdminController) handleRuntimeMockStop(w http.ResponseWriter, r *http.Request) {
	if err := c.runtime.StopMockListener(r.Context(), r.PathValue("environment_id")); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

func (c *AdminController) handleRuntimeMockRoutes(w http.ResponseWriter, r *http.Request) {
	items, err := c.runtime.MockRoutes(r.PathValue("environment_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (c *AdminController) handleRuntimeMockEvents(w http.ResponseWriter, r *http.Request) {
	items, err := c.runtime.MockEvents(r.PathValue("environment_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (c *AdminController) handleRuntimeLoadRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, c.runtime.ListActiveRuns())
}

func (c *AdminController) getRunRecord(ctx context.Context, id string) (domain.RunRecord, error) {
	if record, ok := c.runtime.ActiveRunRecord(id); ok {
		return record, nil
	}
	return c.projects.GetRun(ctx, id)
}

func readJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}
	return common.DecodeJson(r.Body, target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	data, err := common.Marshal(payload)
	if err != nil {
		http.Error(w, "json marshal failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"message": err.Error()})
}
