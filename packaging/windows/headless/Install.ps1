#Requires -Version 5.1
#Requires -RunAsAdministrator
[CmdletBinding()]
param(
    [string]$InstallRoot = "$env:ProgramFiles\AgentDockHeadless",
    [ValidateRange(1024,65535)][int]$Port = 8765
)
$ErrorActionPreference = 'Stop'
function Set-PrivateDirectory([string]$Path) {
    $acl = New-Object System.Security.AccessControl.DirectorySecurity
    $acl.SetAccessRuleProtection($true, $false)
    foreach ($sid in @('S-1-5-18','S-1-5-32-544')) {
        $identity = New-Object System.Security.Principal.SecurityIdentifier($sid)
        $rule = New-Object System.Security.AccessControl.FileSystemAccessRule($identity,'FullControl','ContainerInherit,ObjectInherit','None','Allow')
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $Path -AclObject $acl
}
function Read-Secret([string]$Prompt) {
    $secret = Read-Host $Prompt -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secret)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
}
$InstallRoot = [IO.Path]::GetFullPath($InstallRoot)
if (Get-Service -Name AgentDockHeadless,AgentDockHeadlessTunnel -ErrorAction SilentlyContinue) {
    throw '精简服务已注册。为保护已有配置，本安装器不覆盖，请先阅读升级说明。'
}
if ((Test-Path -LiteralPath $InstallRoot) -and @(Get-ChildItem -LiteralPath $InstallRoot -Force).Count -gt 0) {
    throw '安装目录必须不存在或为空。不要选择 MES 根目录；可以选择其独立的 ops-agent 子目录。'
}
foreach ($name in @('agentdock-service.exe','cloudflared.exe')) {
    if (!(Test-Path -LiteralPath (Join-Path $PSScriptRoot $name) -PathType Leaf)) { throw "安装包缺少 $name" }
}
$hashFile = Join-Path $PSScriptRoot 'SHA256SUMS.json'
if (!(Test-Path -LiteralPath $hashFile)) { throw '缺少安装包校验清单' }
$hashes = Get-Content -LiteralPath $hashFile -Raw | ConvertFrom-Json
foreach ($name in @('agentdock-service.exe','cloudflared.exe')) {
    $expected = $hashes.PSObject.Properties[$name].Value
    if (!$expected -or (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot $name) -Algorithm SHA256).Hash -ne $expected) { throw "$name 校验不一致，停止安装" }
}
$publicUrl = Read-Host '这台主机的 MCP 地址（https://固定域名/mcp）'
$tunnel = Read-Secret 'Cloudflare Tunnel Token'
$bearer = Read-Secret 'MCP Token（留空自动生成，推荐）'
Write-Host '将安装两个开机自启服务，以 LocalSystem 运行，具有本机维护权限。不会启动托盘或 Web 管理面板。'
if ((Read-Host '确认安装请输入 INSTALL') -cne 'INSTALL') { throw '已取消，未安装服务' }
New-Item -ItemType Directory -Path $InstallRoot -Force | Out-Null
Set-PrivateDirectory $InstallRoot
$source = Join-Path $InstallRoot 'setup.once.json'
try {
    foreach ($name in @('agentdock-service.exe','cloudflared.exe')) { Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination (Join-Path $InstallRoot $name) }
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'Uninstall.ps1') -Destination (Join-Path $InstallRoot 'Uninstall.ps1')
    @{port=$Port; public_url=$publicUrl; auth_token=$bearer; tunnel_token=$tunnel} | ConvertTo-Json | Set-Content -LiteralPath $source -Encoding UTF8
    & (Join-Path $InstallRoot 'agentdock-service.exe') install --root $InstallRoot --source-json $source
    if ($LASTEXITCODE -ne 0) { throw '服务安装未完成，请保留错误信息；未进行公网连接测试。' }
    Write-Host '服务安装请求已完成。请在 Cloudflare 配置固定域名指向 http://127.0.0.1:'$Port
    Write-Host '管理员可执行下面的命令，读取登记到运维中枢的 MCP 地址和 Token（不要公开输出）：'
    Write-Host ('& "'+(Join-Path $InstallRoot 'agentdock-service.exe')+'" connection --root "'+$InstallRoot+'"')
} finally {
    if (Test-Path -LiteralPath $source) { Remove-Item -LiteralPath $source -Force }
    $tunnel=$null; $bearer=$null
}
