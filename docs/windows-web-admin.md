# Windows Web 管理面板

Web 管理面板用于无法运行原生 Windows 控制面板的环境。Web 版的 Core 和
后台管理程序均为 Go 静态程序，不依赖 Windows App Runtime、Microsoft Store、
Node.js 或 .NET，适合精简版 Windows。

## 安装和打开

双击 `AgentDock-Web-Setup-amd64.exe` 安装。安装器会一次性完成：

- 安装 Core、核心 Skill、更新组件和 Web 管理后台；
- 注册 Core 与 Web 后台的当前用户登录自启动；
- 创建桌面和开始菜单的“AgentDock Web 管理面板”快捷方式；
- 启动 Core 和 Web 后台，并在安装结束时打开默认浏览器。

以后无需 PowerShell，直接双击快捷方式，或在本机浏览器打开：

<http://127.0.0.1:18765/>

管理后台只监听本机回环地址。首次打开会自动建立当前进程的 HttpOnly 会话，
地址固定，不携带令牌。Core 默认 MCP 地址仍是 <http://127.0.0.1:8765/mcp>。

端口冲突时可指定其他本机端口：

```powershell
agentdock web-admin --listen 127.0.0.1:18766 --open
```

## 能力

- 查看 Core、健康检查、Tunnel、Nexus 和版本状态；
- 查看本地/公网 MCP 地址，在当前 Windows 用户下解密 Bearer Token 和 OAuth 密码；
- 启动、停止、重启 Core，检查或安装更新；
- 配置关闭、Quick Tunnel 或 Named Tunnel；首次启用时自动安装校验过的 `cloudflared` 组件；
- 使用 DPAPI 加密保存 Cloudflare Tunnel Token；
- 修改端口、日志级别、聊天卡片和浏览器连接方式；
- 使用一次性配对码连接 NexusDock；
- 保留未在精简页面中编辑的 ACP Profile，不会用默认值覆盖。

## 安全边界

- 管理服务拒绝绑定非 loopback 地址；
- 管理端口与 MCP/Tunnel 端口分离，不会被 Cloudflare Tunnel 转发；
- 会话令牌只存在于当前管理进程内存中，并通过 HttpOnly、SameSite Cookie 保存；
- 写操作要求同源请求、当前会话 Cookie 和管理请求头；
- 响应禁止缓存，并设置严格 CSP、禁止 iframe 和跨站引用；
- Tunnel Token 只通过权限受限的临时文件交给现有配置逻辑，随后立即删除；
- 长期凭据继续由 AgentDock 现有 DPAPI 文件保存。

此页面应始终在被管理的 Windows 电脑上打开。不要使用端口转发、反向代理或
Cloudflare 将管理端口公开到局域网或互联网。

## 手工/开发启动（仅维护人员）

安装包会自动启动后台，普通用户不需要执行命令。源码调试时仍可运行：

```powershell
& "$env:LOCALAPPDATA\AgentDock\bin\agentdock.exe" web-admin `
  --runtime-root "$env:LOCALAPPDATA\AgentDock" `
  --open
```

端口冲突时，开发人员可加 `--listen 127.0.0.1:18766`；正式快捷方式固定使用 18765。
