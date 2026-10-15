package webadmin

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uvwt/agentdock/internal/buildinfo"
	agentconfig "github.com/uvwt/agentdock/internal/config"
	"github.com/uvwt/agentdock/internal/desktopruntime"
)

//go:embed assets/index.html assets/app.css assets/app.js
var assets embed.FS

const sessionCookie = "agentdock_web_admin"

type server struct {
	runtimeRoot string
	listenAddr  string
	secret      string
	actions     sync.Mutex
}

type errorResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("agentdock web-admin", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runtimeRoot := flags.String("runtime-root", desktopruntime.DefaultRuntimeRoot(), "AgentDock 桌面运行目录")
	listenAddr := flags.String("listen", "127.0.0.1:18765", "仅限回环地址的管理页面监听地址")
	openBrowser := flags.Bool("open", false, "启动后用默认浏览器打开管理页面")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("用法：agentdock web-admin [--runtime-root <目录>] [--listen 127.0.0.1:18765] [--open]")
	}
	if strings.TrimSpace(*runtimeRoot) == "" {
		return errors.New("runtime-root 不能为空")
	}
	address, err := normalizeLoopbackAddress(*listenAddr)
	if err != nil {
		return err
	}
	secret, err := randomSecret()
	if err != nil {
		return fmt.Errorf("生成管理会话失败: %w", err)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("启动 Web 管理面板失败: %w", err)
	}
	defer listener.Close()
	s := &server{runtimeRoot: filepath.Clean(*runtimeRoot), listenAddr: listener.Addr().String(), secret: secret}
	baseURL := "http://" + listener.Addr().String()
	fmt.Fprintln(stdout, "AgentDock Web 管理面板已启动（仅本机可访问）")
	fmt.Fprintln(stdout, "管理地址:", baseURL+"/")
	fmt.Fprintln(stdout, "关闭此窗口会停止管理页面，但不会停止 AgentDock Core。")
	if *openBrowser {
		if err := OpenURL(baseURL + "/"); err != nil {
			fmt.Fprintln(stderr, "自动打开浏览器失败，请复制管理地址手动打开:", err)
		}
	}
	httpServer := &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	done := make(chan error, 1)
	go func() { done <- httpServer.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

func normalizeLoopbackAddress(raw string) (string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("管理监听地址无效: %w", err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", errors.New("管理端口必须是整数")
	}
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "127.0.0.1", "localhost":
		return net.JoinHostPort("127.0.0.1", port), nil
	case "::1":
		return net.JoinHostPort("::1", port), nil
	default:
		return "", errors.New("Web 管理面板只允许监听 127.0.0.1、::1 或 localhost")
	}
}

func randomSecret() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/assets/app.css", s.handleAsset("assets/app.css", "text/css; charset=utf-8"))
	mux.HandleFunc("/assets/app.js", s.handleAsset("assets/app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/api/snapshot", s.requireSession(s.handleSnapshot))
	mux.HandleFunc("/api/credentials", s.requireSession(s.handleCredentials))
	mux.HandleFunc("/api/service", s.requireMutation(s.handleService))
	mux.HandleFunc("/api/tunnel", s.requireMutation(s.handleTunnel))
	mux.HandleFunc("/api/tunnel/action", s.requireMutation(s.handleTunnelAction))
	mux.HandleFunc("/api/config", s.requireMutation(s.handleConfig))
	mux.HandleFunc("/api/public-test", s.requireMutation(s.handlePublicTest))
	mux.HandleFunc("/api/nexus/pair", s.requireMutation(s.handleNexusPair))
	mux.HandleFunc("/api/update/check", s.requireMutation(s.handleUpdateCheck))
	mux.HandleFunc("/api/update/run", s.requireMutation(s.handleUpdateRun))
	return s.securityHeaders(mux)
}

func (s *server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if !s.validSession(r) {
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: s.secret, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	s.serveAsset(w, "assets/index.html", "text/html; charset=utf-8")
}

func (s *server) handleAsset(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !s.validSession(r) {
			http.Error(w, "未授权", http.StatusUnauthorized)
			return
		}
		s.serveAsset(w, name, contentType)
	}
}

func (s *server) serveAsset(w http.ResponseWriter, name, contentType string) {
	data, err := assets.ReadFile(name)
	if err != nil {
		http.Error(w, "资源不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(data)
}

func (s *server) validSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	return err == nil && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.secret)) == 1
}

func (s *server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.validSession(r) {
			writeError(w, http.StatusUnauthorized, "管理会话已失效，请重新打开 http://127.0.0.1:18765/。")
			return
		}
		next(w, r)
	}
}

func (s *server) requireMutation(next http.HandlerFunc) http.HandlerFunc {
	return s.requireSession(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-AgentDock-Admin") != "1" {
			writeError(w, http.StatusMethodNotAllowed, "此操作只接受管理页面发起的 POST 请求。")
			return
		}
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		expected := "http://" + s.listenAddr
		if origin != expected {
			writeError(w, http.StatusForbidden, "请求来源不是本机管理页面。")
			return
		}
		next(w, r)
	})
}

func (s *server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET 请求。")
		return
	}
	runtimeInfo, err := desktopruntime.ReadWebAdminRuntime(r.Context(), s.runtimeRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	serviceStatus, serviceErr := desktopruntime.ServiceStatusForRuntime(r.Context(), s.runtimeRoot)
	tunnelStatus, tunnelErr := desktopruntime.TunnelStatusForRuntime(r.Context(), s.runtimeRoot)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "version": buildinfo.Version, "runtime": runtimeInfo,
		"service": serviceStatus, "tunnel": tunnelStatus,
		"service_error": errorText(serviceErr), "tunnel_error": errorText(tunnelErr),
	})
}

func (s *server) handleCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET 请求。")
		return
	}
	credentials, err := desktopruntime.ReadWebAdminCredentials(s.runtimeRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "credentials": credentials})
}

func (s *server) handleService(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Action string `json:"action"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	s.actions.Lock()
	defer s.actions.Unlock()
	if err := desktopruntime.ServiceActionForRuntime(r.Context(), s.runtimeRoot, request.Action); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "服务操作已完成"})
}

func (s *server) handleTunnel(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Mode      string `json:"mode"`
		ServerURL string `json:"server_url"`
		Token     string `json:"token"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	s.actions.Lock()
	defer s.actions.Unlock()
	if request.Mode == "quick" || request.Mode == "named" {
		output, installErr := s.runCore(r.Context(), "component", "install", "cloudflared", "--runtime-root", s.runtimeRoot, "--json")
		if installErr != nil {
			writeError(w, http.StatusBadGateway, "安装固定隧道组件失败: "+commandError(output, installErr))
			return
		}
	}
	tokenFile := ""
	if strings.TrimSpace(request.Token) != "" {
		file, err := os.CreateTemp("", "agentdock-tunnel-token-*.txt")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		tokenFile = file.Name()
		defer os.Remove(tokenFile)
		if err := file.Chmod(0o600); err == nil {
			_, err = file.WriteString(strings.TrimSpace(request.Token))
		}
		closeErr := file.Close()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if closeErr != nil {
			writeError(w, http.StatusInternalServerError, closeErr.Error())
			return
		}
	}
	err := desktopruntime.ConfigureTunnelForRuntime(r.Context(), desktopruntime.TunnelConfigureRequest{
		RuntimeRoot: s.runtimeRoot, Mode: request.Mode, ServerURL: request.ServerURL, TokenFile: tokenFile,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "公网访问配置已应用"})
}

func (s *server) handleTunnelAction(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Action string `json:"action"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	s.actions.Lock()
	defer s.actions.Unlock()
	if err := desktopruntime.TunnelActionForRuntime(r.Context(), s.runtimeRoot, request.Action); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Tunnel 操作已完成"})
}

func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Port          int    `json:"port"`
		LogLevel      string `json:"log_level"`
		MCPAppsMode   string `json:"mcp_apps_mode"`
		BrowserMode   string `json:"browser_mode"`
		BrowserCDPURL string `json:"browser_cdp_url"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	current, err := desktopruntime.ReadWebAdminRuntime(r.Context(), s.runtimeRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	mode, err := agentconfig.ParseMCPAppsMode(request.MCPAppsMode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	update := desktopruntime.ConfigUpdateRequest{
		RuntimeRoot: s.runtimeRoot, Port: request.Port, LogLevel: request.LogLevel, MCPAppsMode: mode,
		OAuthAccessTokenTTL: current.Settings.OAuthAccessTokenTTL,
		ACPEnabled:          current.Settings.ACPEnabled, ACPProfiles: current.Settings.ACPProfiles,
		ACPDefaultProfile: current.Settings.ACPDefaultProfile,
	}
	switch request.BrowserMode {
	case "off":
	case "isolated":
		update.BrowserEnabled = true
	case "reuse":
		update.BrowserEnabled = true
		update.BrowserReuseExistingCDP = true
	case "cdp":
		update.BrowserEnabled = true
		update.BrowserCDPURL = strings.TrimSpace(request.BrowserCDPURL)
	default:
		writeError(w, http.StatusBadRequest, "浏览器模式无效")
		return
	}
	s.actions.Lock()
	defer s.actions.Unlock()
	if err := desktopruntime.ApplyConfigUpdate(r.Context(), update); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "设置已保存，Core 已重新启动"})
}

func (s *server) handlePublicTest(w http.ResponseWriter, r *http.Request) {
	runtimeInfo, err := desktopruntime.ReadWebAdminRuntime(r.Context(), s.runtimeRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runtimeInfo.PublicMCPURL == "" {
		writeError(w, http.StatusBadRequest, "当前没有可测试的公网地址")
		return
	}
	healthURL := strings.TrimSuffix(runtimeInfo.PublicMCPURL, "/mcp") + "/healthz"
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	response, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, "公网健康检查返回 "+response.Status)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "公网访问正常", "status": response.StatusCode})
}

func (s *server) handleNexusPair(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Endpoint string `json:"endpoint"`
		Code     string `json:"code"`
		Name     string `json:"name"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	args := []string{"nexus", "pair", "--endpoint", strings.TrimSpace(request.Endpoint), "--code", strings.TrimSpace(request.Code)}
	if strings.TrimSpace(request.Name) != "" {
		args = append(args, "--name", strings.TrimSpace(request.Name))
	}
	output, err := s.runCore(r.Context(), args...)
	if err != nil {
		writeError(w, http.StatusBadRequest, commandError(output, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Nexus 配对完成，请重启 Core"})
}

func (s *server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	output, err := s.runCore(r.Context(), "update", "--check")
	if err != nil {
		writeError(w, http.StatusBadGateway, commandError(output, err))
		return
	}
	var result any
	if json.Unmarshal(output, &result) != nil {
		result = strings.TrimSpace(string(output))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
}

func (s *server) handleUpdateRun(w http.ResponseWriter, r *http.Request) {
	s.actions.Lock()
	defer s.actions.Unlock()
	output, err := s.runCore(r.Context(), "update")
	if err != nil {
		writeError(w, http.StatusBadGateway, commandError(output, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "更新完成", "output": strings.TrimSpace(string(output))})
}

func (s *server) runCore(ctx context.Context, args ...string) ([]byte, error) {
	manifest, err := desktopruntime.Load(filepath.Join(s.runtimeRoot, "runtime.json"))
	if err != nil {
		return nil, fmt.Errorf("读取 AgentDock 运行清单失败: %w", err)
	}
	binary := manifest.AgentDockBinary
	command := exec.CommandContext(ctx, binary, args...)
	return command.CombinedOutput()
}

func commandError(output []byte, err error) string {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return err.Error()
	}
	return message
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "请求数据无效: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{OK: false, Message: message})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
