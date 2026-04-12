package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	defaultControlPort  = 18090
	defaultWorkerConfig = "config.json"
	defaultDataDir      = "./data"
	defaultAdminPass    = "admin"
)

type bootstrapConfig struct {
	ControlPort   int
	DataDir       string
	AdminPassword string
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "control":
		if err := runControl(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "control server failed: %v\n", err)
			os.Exit(1)
		}
	case "worker":
		if err := runWorker(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "worker server failed: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(2)
	}
}

func runControl(args []string) error {
	fs := flag.NewFlagSet("control", flag.ContinueOnError)
	port := fs.Int("port", intEnv("LOAD_TESTER_CONTROL_PORT", defaultControlPort), "control server port")
	dataDir := fs.String("data-dir", stringEnv("LOAD_TESTER_DATA_DIR", defaultDataDir), "data directory")
	adminPassword := fs.String("admin-password", stringEnv("LOAD_TESTER_ADMIN_PASSWORD", defaultAdminPass), "control admin password")
	if err := fs.Parse(args); err != nil {
		return err
	}

	bootstrap := bootstrapConfig{
		ControlPort:   *port,
		DataDir:       filepath.Clean(*dataDir),
		AdminPassword: strings.TrimSpace(*adminPassword),
	}
	if bootstrap.AdminPassword == "" {
		bootstrap.AdminPassword = defaultAdminPass
	}

	configPath := filepath.Join(bootstrap.DataDir, defaultWorkerConfig)
	store, err := loadConfigStore(configPath)
	if err != nil {
		return err
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	bridge := newProcessManager(execPath, bootstrap.DataDir, configPath, store)
	server := newControlServer(bootstrap, store, bridge)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.run(ctx)
}

func runWorker(args []string) error {
	fs := flag.NewFlagSet("worker", flag.ContinueOnError)
	configPath := fs.String("config", "", "worker config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*configPath) == "" {
		return fmt.Errorf("--config is required")
	}

	store, err := loadConfigStore(filepath.Clean(*configPath))
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := newWorkerApp(filepath.Clean(*configPath), store)
	return app.run(ctx)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  load-tester control [--port 18090] [--data-dir ./data] [--admin-password admin]")
	fmt.Fprintln(os.Stderr, "  load-tester worker --config ./data/config.json")
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed int
	_, err := fmt.Sscanf(value, "%d", &parsed)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func stringEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
