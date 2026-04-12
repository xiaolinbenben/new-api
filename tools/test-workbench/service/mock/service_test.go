package mock

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
	"github.com/stretchr/testify/require"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestMockServiceServesModels(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	service := NewService()
	env := domain.DefaultEnvironment()
	env.ID = "env-1"
	env.ProjectID = "project-1"
	env.MockPort = port
	profile := domain.MockProfile{
		ID:        "mock-1",
		ProjectID: "project-1",
		Name:      "Default",
		Config:    domain.DefaultMockProfileConfig(),
	}

	runtime, err := service.Start(context.Background(), env, profile)
	require.NoError(t, err)
	require.True(t, runtime.Healthy)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(runtime.LocalBaseURL + "/v1/models")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.NoError(t, service.Stop(context.Background(), env.ID))
}
