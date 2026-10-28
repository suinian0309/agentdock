//go:build windows

// agentdock-service is the unattended Windows distribution. It deliberately
// bypasses desktop manifests, login tasks, tray, Web admin and Nexus pairing.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/uvwt/agentdock/internal/app"
	"github.com/uvwt/agentdock/internal/config"
	"github.com/uvwt/agentdock/internal/fs/atomicfile"
	"github.com/uvwt/agentdock/internal/fs/securepath"
	"github.com/uvwt/agentdock/internal/httpx"
	"github.com/uvwt/agentdock/internal/logfile"
	"github.com/uvwt/agentdock/internal/mcp"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const coreName = "AgentDockHeadless"
const tunnelName = "AgentDockHeadlessTunnel"

type settings struct {
	Port        int    `json:"port"`
	PublicURL   string `json:"public_url"`
	AuthToken   string `json:"auth_token"`
	TunnelToken string `json:"tunnel_token"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "agentdock-service:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("用法：agentdock-service <install|run|status|uninstall|connection> --root <独立安装目录> [--source-json <一次性配置>]")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	root := f.String("root", "", "独立安装目录")
	source := f.String("source-json", "", "一次性配置 JSON 路径，不要在命令行传递凭据")
	role := f.String("role", "core", "core 或 tunnel")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 || *root == "" || !filepath.IsAbs(*root) {
		return errors.New("必须指定绝对路径 --root，且不能有多余参数")
	}
	*root = filepath.Clean(*root)
	switch args[0] {
	case "install":
		return install(*root, *source)
	case "run":
		if *role != "core" && *role != "tunnel" {
			return errors.New("role 必须是 core 或 tunnel")
		}
		yes, err := svc.IsWindowsService()
		if err != nil {
			return err
		}
		if !yes {
			return errors.New("run 仅供 Windows 服务管理器使用")
		}
		name := coreName
		if *role == "tunnel" {
			name = tunnelName
		}
		return svc.Run(name, &handler{root: *root, role: *role})
	case "connection":
		c, err := load(*root)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"mcp_url": c.PublicURL, "bearer_token": c.AuthToken})
	case "status", "uninstall":
		return manage(args[0], *root)
	default:
		return errors.New("未知命令")
	}
}

func validate(c *settings) error {
	if c.Port == 0 {
		c.Port = 8765
	}
	if c.Port < 1024 || c.Port > 65535 {
		return errors.New("port 必须在1024至65535之间")
	}
	u, err := url.Parse(c.PublicURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "/mcp" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("public_url 必须为 https://固定域名/mcp")
	}
	if len(c.AuthToken) < 32 || strings.ContainsAny(c.AuthToken, "\r\n") {
		return errors.New("MCP Token 至少32字符且不能换行")
	}
	if strings.TrimSpace(c.TunnelToken) == "" || strings.ContainsAny(c.TunnelToken, "\r\n") {
		return errors.New("必须提供有效的单行 Tunnel Token")
	}
	return nil
}

func crypt(data []byte, protect bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("配置不能为空")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	var err error
	if protect {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_LOCAL_MACHINE|windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
func load(root string) (settings, error) {
	var c settings
	b, err := os.ReadFile(filepath.Join(root, "config.dpapi"))
	if err != nil {
		return c, err
	}
	b, err = crypt(b, false)
	if err != nil {
		return c, errors.New("配置无法解密，请确认在原机器运行并使用正确权限")
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, errors.New("配置格式不正确")
	}
	return c, validate(&c)
}

func install(root, source string) (err error) {
	if source == "" {
		return errors.New("请使用 --source-json 指定一次性配置文件")
	}
	if _, e := os.Stat(filepath.Join(root, "config.dpapi")); !errors.Is(e, os.ErrNotExist) {
		return errors.New("此目录已有配置或无法访问，拒绝覆盖")
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	if !strings.EqualFold(filepath.Dir(exe), root) {
		return errors.New("请从目标安装目录内运行 agentdock-service.exe")
	}
	if _, e = os.Stat(filepath.Join(root, "cloudflared.exe")); e != nil {
		return errors.New("安装目录缺少 cloudflared.exe")
	}
	manager, e := mgr.Connect()
	if e != nil {
		return e
	}
	defer manager.Disconnect()
	// Inspect all names first; never replace an unrelated or desktop service.
	for _, name := range []string{coreName, tunnelName} {
		s, e := manager.OpenService(name)
		if e == nil {
			s.Close()
			return fmt.Errorf("服务 %s 已存在，拒绝覆盖", name)
		}
		if !errors.Is(e, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return e
		}
	}
	raw, e := os.ReadFile(source)
	if e != nil {
		return e
	}
	var c settings
	decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(string(raw), "\ufeff")))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&c); e != nil {
		return errors.New("一次性配置 JSON 格式不正确")
	}
	if c.AuthToken == "" {
		b := make([]byte, 32)
		if _, e = rand.Read(b); e != nil {
			return e
		}
		c.AuthToken = hex.EncodeToString(b)
	}
	if e = validate(&c); e != nil {
		return e
	}
	if e = securepath.EnsurePrivate(root); e != nil {
		return e
	}
	for _, d := range []string{"state", "work", "logs"} {
		if e = os.MkdirAll(filepath.Join(root, d), 0700); e != nil {
			return e
		}
	}
	raw, e = json.Marshal(c)
	if e != nil {
		return e
	}
	encrypted, e := crypt(raw, true)
	if e != nil {
		return e
	}
	if e = atomicfile.Write(filepath.Join(root, "config.dpapi"), encrypted, 0600); e != nil {
		return e
	}
	var created []*mgr.Service
	defer func() {
		if err != nil {
			for i := len(created) - 1; i >= 0; i-- {
				_, _ = created[i].Control(svc.Stop)
				_ = created[i].Delete()
			}
			_ = os.Remove(filepath.Join(root, "config.dpapi"))
		}
		for _, s := range created {
			s.Close()
		}
	}()
	for _, item := range []struct{ name, role, description string }{{coreName, "core", "运维中枢 AgentDock 执行端"}, {tunnelName, "tunnel", "运维中枢 Cloudflare 固定域名隧道"}} {
		s, e := manager.CreateService(item.name, exe, mgr.Config{DisplayName: item.description, Description: item.description, StartType: mgr.StartAutomatic, DelayedAutoStart: true, ServiceStartName: "LocalSystem"}, "run", "--root", root, "--role", item.role)
		if e != nil {
			return e
		}
		created = append(created, s)
		if e = s.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 10 * time.Second}, {Type: mgr.ServiceRestart, Delay: 30 * time.Second}, {Type: mgr.ServiceRestart, Delay: 60 * time.Second}}, 86400); e != nil {
			return e
		}
		if e = s.SetRecoveryActionsOnNonCrashFailures(true); e != nil {
			return e
		}
	}
	for _, s := range created {
		if e = s.Start(); e != nil {
			return e
		}
	}
	fmt.Fprintln(os.Stdout, "两个后台服务已注册并收到启动请求。使用 status 查看状态；使用 connection 查看登记到运维中枢的地址与凭据。未执行公网连接测试。")
	return nil
}

func manage(action, root string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	for _, name := range []string{tunnelName, coreName} {
		s, err := m.OpenService(name)
		if err != nil {
			return err
		}
		c, err := s.Config()
		if err != nil {
			s.Close()
			return err
		}
		argv, err := windows.DecomposeCommandLine(c.BinaryPathName)
		if err != nil || len(argv) != 6 || !strings.EqualFold(argv[0], filepath.Join(root, "agentdock-service.exe")) || argv[1] != "run" || argv[2] != "--root" || !strings.EqualFold(argv[3], root) || argv[4] != "--role" {
			s.Close()
			return errors.New("服务不属于指定安装目录，拒绝操作")
		}
		if action == "status" {
			state, e := s.Query()
			s.Close()
			if e != nil {
				return e
			}
			fmt.Fprintf(os.Stdout, "%s: %d (4=运行，1=停止)\n", name, state.State)
			continue
		}
		state, err := s.Query()
		if err != nil {
			s.Close()
			return err
		}
		if state.State != svc.Stopped {
			if _, err = s.Control(svc.Stop); err != nil {
				s.Close()
				return err
			}
			deadline := time.Now().Add(35 * time.Second)
			for time.Now().Before(deadline) {
				state, err = s.Query()
				if err != nil || state.State == svc.Stopped {
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
			if err != nil || state.State != svc.Stopped {
				s.Close()
				return errors.New("服务未能停止，保留注册和文件")
			}
		}
		err = s.Delete()
		s.Close()
		if err != nil {
			return err
		}
	}
	if action == "uninstall" {
		fmt.Fprintln(os.Stdout, "服务已卸载；程序、配置和日志保留，请按需要手动归档或移除。")
	}
	return nil
}

type handler struct{ root, role string }

func (h *handler) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending, WaitHint: 30000}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.serve(ctx) }()
	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	changes <- current
	for {
		select {
		case err := <-done:
			if err != nil {
				return true, 1
			}
			return false, 0
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				changes <- current
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending, WaitHint: 30000}
				cancel()
				select {
				case <-done:
					return false, 0
				case <-time.After(25 * time.Second):
					return true, 2
				}
			}
		}
	}
}

func (h *handler) serve(ctx context.Context) error {
	logs, err := logfile.Open(filepath.Join(h.root, "logs", h.role+".log"), 2<<20, 3)
	if err != nil {
		return err
	}
	defer logs.Close()
	c, err := load(h.root)
	if err != nil {
		fmt.Fprintln(logs, err)
		return err
	}
	if h.role == "tunnel" {
		// Only this child receives the tunnel token. It never appears in argv/files.
		cmd := exec.CommandContext(ctx, filepath.Join(h.root, "cloudflared.exe"), "tunnel", "--no-autoupdate", "run")
		cmd.Dir = h.root
		cmd.Env = append(os.Environ(), "TUNNEL_TOKEN="+c.TunnelToken)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Stdout = logs
		cmd.Stderr = logs
		err = cmd.Run()
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			return errors.New("cloudflared 意外退出")
		}
		return err
	}
	// Construct explicit settings rather than inheriting a desktop user's paths,
	// OAuth secrets, ACP profiles, Nexus identity or runtime manifests.
	cfg := config.Config{AgentDockHome: filepath.Join(h.root, "state"), AgentDockDefaultDir: filepath.Join(h.root, "work"), Host: "127.0.0.1", Port: c.Port, AuthToken: c.AuthToken, LogLevel: "warn", MCPAppsMode: config.MCPAppsModeOff}
	if err = cfg.Normalize(); err != nil {
		return err
	}
	if err = cfg.ValidateAuth(); err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	runtime, err := app.NewRuntime(cfg)
	if err != nil {
		fmt.Fprintln(logs, "Core 初始化失败")
		return err
	}
	defer runtime.Close()
	err = httpx.Serve(ctx, mcp.NewServer(runtime, cfg), runtime, cfg)
	if err != nil {
		fmt.Fprintln(logs, "Core 退出:", err)
	}
	return err
}
