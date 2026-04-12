package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	appruntime "github.com/QuantumNous/new-api/tools/test-workbench/app/runtime"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		if err := runServe(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "测试工作台启动失败: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(2)
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	adminHost := fs.String("admin-host", "127.0.0.1", "管理端监听地址")
	adminPort := fs.Int("admin-port", 18880, "管理端监听端口")
	dataDir := fs.String("data-dir", "./tools/test-workbench/data", "数据目录")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := appruntime.New(appruntime.Config{
		AdminHost:    *adminHost,
		AdminPort:    *adminPort,
		DataDir:      filepath.Clean(*dataDir),
		DatabasePath: filepath.Join(filepath.Clean(*dataDir), "workbench.db"),
	}, uiFiles)
	if err != nil {
		return err
	}
	return app.Run(ctx)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "用法:")
	fmt.Fprintln(os.Stderr, "  go run ./tools/test-workbench serve [--admin-host 127.0.0.1] [--admin-port 18880] [--data-dir ./tools/test-workbench/data]")
}
