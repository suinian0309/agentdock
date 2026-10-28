#Requires -Version 5.1
#Requires -RunAsAdministrator
[CmdletBinding()]
param([string]$InstallRoot=$PSScriptRoot)
$ErrorActionPreference='Stop'
$InstallRoot=[IO.Path]::GetFullPath($InstallRoot)
Write-Host '将停止并注销 AgentDockHeadless 与 AgentDockHeadlessTunnel。配置与日志保留。'
if ((Read-Host '确认卸载请输入 UNINSTALL') -cne 'UNINSTALL') { throw '已取消' }
& (Join-Path $InstallRoot 'agentdock-service.exe') uninstall --root $InstallRoot
if ($LASTEXITCODE -ne 0) { throw '卸载未完成，请检查服务状态；没有删除安装目录。' }
