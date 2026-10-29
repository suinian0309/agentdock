# Windows 精简服务版

这是新增的独立服务发行入口，复用现有 AgentDock Core；不替换原桌面版/Web版安装方式，不改动其连接设置。

## 安装后的运行方式

- `AgentDockHeadless`：Windows SCM 自动延迟启动服务，直接运行 Core，监听 `127.0.0.1:8765/mcp`（端口可在安装时设置）。
- `AgentDockHeadlessTunnel`：另一个 SCM 服务，托管 cloudflared，通过既有固定域名隧道转发请求。
- 两个服务均以 LocalSystem 运行，具有本机高级维护权限。无需用户登录，不创建托盘图标或 Web 管理面板。SCM 注册的服务可在 Windows 服务列表查看、停止和卸载。
- 不启用 Browser、ACP、聊天卡片和 Nexus 配对，不引入数据库、Node.js、Go 开发环境到宿主机。不继承桌面用户的 DPAPI 配置或登录计划任务。
- 服务异常退出后分别在10/30/60秒恢复。日志每组件2MiB、保留3份。未做内存或CPU占用承诺，二进制仍复用 Core 模块，不等同于彻底裁剪所有链接依赖。

## 一次性准备与安装

1. 在 Cloudflare 准备此主机使用的远程管理隧道、固定域名和 Tunnel Token。将该域名的源站设置为 `http://127.0.0.1:8765`，云端连接地址为 `https://域名/mcp`。本安装器不替你创建 Cloudflare 账号、域名或路由。
2. 先完整解压安装包，再双击 `Start-Install.cmd`，接受 Windows 管理员权限提示。不要直接双击服务 EXE；`Install.ps1` 仍可从已打开的管理员 PowerShell 中运行。
3. 填写 MCP 公网地址、Tunnel Token。MCP Token 可留空，由安装器生成独立随机凭据。
4. 确认安装：程序将复制文件、保存配置并注册启动两个服务。未自行运行公网连接测试。
5. 按安装器提示执行 `connection --root <安装目录>`，读取此主机的地址与 MCP Token，在运维中枢主机管理中登记。这个命令输出敏感凭据，不要截图或写入普通日志。

安装后不需要再打开宿主机 Web 页面。默认安装到 `C:\Program Files\AgentDockHeadless`。若放在 MES 内，请选择空的专用 `ops-agent` 子目录，将它排除在 MES 更新/清理/回退范围外；不要把 MES 根目录作为安装目录。服务使用自己的 `work` 默认目录，维护命令可显式选择 MES 目录。

首次安装需要本机管理员权限；已存在服务或配置时拒绝覆盖，避免静默替换凭据和破坏现有执行端。现有桌面版若占用8765端口，应在维护窗口中明确决定停用旧实例或为新版指定其他端口，并同步隧道路由。安装器不会擅自停旧版。

## 凭据与目录

安装目录仅 SYSTEM 和本地管理员可访问，配置保存为机器范围 DPAPI 加密的 `config.dpapi`，适用于安装管理员与 LocalSystem 之间的配置交接，必须同时保留严格ACL。该文件不能直接复制到另一台机器使用。

一次性明文 JSON 只写入已受保护的安装目录，并在安装结束或失败时清理。Tunnel Token 不写服务命令行，只有 cloudflared 子进程收到 `TUNNEL_TOKEN` 环境变量；Core 命令执行环境不接收它。不要把 Tunnel Token 填进运维中枢的 MCP Token 字段。

`state` 保存独立运行状态，`work` 是默认命令目录，`logs` 保存轮转日志。停止 MES 不停止这两个服务。

## 状态、卸载和后续升级

管理员执行 `agentdock-service.exe status --root <目录>` 查看 SCM 运行状态；SCM Running 仅表明服务进程运行，不代表隧道公网可访问或 MES 健康。

运行安装目录内 `Uninstall.ps1`，明确确认后停止并注销两个服务。程序、配置和日志保留，不递归删除文件。

本版尚不包含自动更新或云端登记码。升级请先保留原二进制与加密配置，在维护窗口停止两个服务后替换经确认的新二进制，再启动并检查。不要调用原桌面版自动更新来升级本独立服务发行物。将来的升级脚本需带独立版本、备份和回退规则。

## 构建

在仓库根目录执行：

```powershell
.\packaging\windows\build-windows-headless.ps1 `
  -Go 'go' `
  -CloudflaredPath 'C:\已核验组件\cloudflared.exe' `
  -CloudflaredSHA256 '<维护者核对的64位SHA256>' `
  -Version '0.1.0'
```

需要符合 go.mod 的 Go 工具链及依赖下载能力；目标宿主机不需要这些。cloudflared 使用维护者提供并核验的固定版本，安装时不自动下载最新版本。打包器生成 SHA256 清单，校验用于完整性核对，不等于代码签名或发布者身份认证。cloudflared 原有许可与通知应随正式分发保留。

## 验证边界

当前是服务版实现与打包流程，安装器不会在开发机或生产设备上自动安装。首次发行需集中做一次编译检查，再在指定设备安装，确认无人登录/注销、重启后恢复和停止 MES 后仍能维护。只验证新增的服务化行为，不重复原有连接可行性测试，不做浏览器巡检。

参考：Cloudflare 官方支持 `TUNNEL_TOKEN` 运行参数：https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/configure-tunnels/run-parameters/

## 构建状态与 GitHub 自动构建

2026-10-28：已使用本机便携 Go1.27.2 编译服务版。工具链位于 C:\Users\Administrator\AppData\Local\Temp\codex-go-1.27.2\go\bin\go.exe，不依赖全局 PATH。服务EXE约21MB；不代表已经完成服务安装、无人登录和重启验收。

新增 Windows Packages (Web and Service) 工作流：main 或 codex/web-admin-installer 上相关源码变化时自动构建，也可手动运行。每次提供 AgentDock-Headless-Windows-amd64 和 AgentDock-Web-Windows-amd64 两份 Actions Artifact，保留30天。构建不会安装服务、连接隧道或运行浏览器巡检。cloudflared 固定版本与SHA256在 packaging/windows/headless/cloudflared.json 中登记。

## 安装窗口闪退修复（0.1.1）

新增 Start-Install.cmd 和自动提权启动器；提权后的窗口在安装成功或失败时都会等待回车，失败会保留具体错误。中文PowerShell脚本按带BOM的UTF-8保存，兼容Windows PowerShell5.1。仅修改安装入口，服务二进制保持原版，不在开发机运行安装。
