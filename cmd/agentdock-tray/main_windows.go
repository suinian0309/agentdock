//go:build windows

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/uvwt/agentdock/internal/desktopruntime"
	"github.com/uvwt/agentdock/internal/webadmin"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "agentdock-tray:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	runtimeRoot := defaultRuntimeRoot()
	if len(args) > 0 && (strings.EqualFold(args[0], "--start-core") || strings.EqualFold(args[0], "--start-tunnel")) {
		if len(args) == 3 && strings.EqualFold(args[1], "--runtime-root") && strings.TrimSpace(args[2]) != "" {
			runtimeRoot = args[2]
		} else if len(args) != 1 {
			return fmt.Errorf("用法: agentdock-tray <--start-core|--start-tunnel> [--runtime-root <目录>]")
		}
		if strings.EqualFold(args[0], "--start-tunnel") {
			return desktopruntime.TunnelActionForRuntime(ctx, runtimeRoot, "start")
		}
		return desktopruntime.ServiceActionForRuntime(ctx, runtimeRoot, "start")
	}
	if len(args) > 0 && strings.EqualFold(args[0], "--background") {
		return webadmin.Run(ctx, []string{"--runtime-root", runtimeRoot, "--listen", "127.0.0.1:18765"}, os.Stdout, os.Stderr)
	}
	if len(args) != 0 {
		return fmt.Errorf("不支持的参数: %s", strings.Join(args, " "))
	}

	const target = "http://127.0.0.1:18765/"
	if !webReady(target) {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		command := exec.Command(executable, "--background")
		if err := command.Start(); err != nil {
			return fmt.Errorf("启动 Web 管理服务: %w", err)
		}
		_ = command.Process.Release()
		for attempt := 0; attempt < 40 && !webReady(target); attempt++ {
			time.Sleep(100 * time.Millisecond)
		}
		if !webReady(target) {
			return fmt.Errorf("Web 管理服务未能在 %s 启动", target)
		}
	}
	return webadmin.OpenURL(target)
}

func defaultRuntimeRoot() string {
	root := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if root == "" {
		root = filepath.Dir(filepath.Dir(filepath.Dir(os.Args[0])))
	}
	return filepath.Join(root, "AgentDock")
}

func webReady(target string) bool {
	client := &http.Client{Timeout: 250 * time.Millisecond}
	response, err := client.Get(target)
	if err != nil {
		return false
	}
	_ = response.Body.Close()
	return response.StatusCode == http.StatusOK
}
