#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$CloudflaredPath,
    [Parameter(Mandatory=$true)][ValidatePattern('^[a-fA-F0-9]{64}$')][string]$CloudflaredSHA256,
    [string]$Go='go',
    [string]$OutputRoot='',
    [string]$Version='local'
)
$ErrorActionPreference='Stop'
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if (!$OutputRoot) { $OutputRoot=Join-Path $repo 'dist-headless' }
$OutputRoot=[IO.Path]::GetFullPath($OutputRoot)
if ($Version -notmatch '^[a-zA-Z0-9._-]+$') { throw 'Version 只允许字母、数字、点、下划线和短横线' }
$package=Join-Path $OutputRoot ('AgentDock-Headless-Windows-amd64-'+$Version)
if (Test-Path -LiteralPath $package) { throw '输出目录已存在，请指定新的 Version 或 OutputRoot，避免覆盖已有产物。' }
$CloudflaredPath=(Resolve-Path -LiteralPath $CloudflaredPath).Path
if ((Get-FileHash -LiteralPath $CloudflaredPath -Algorithm SHA256).Hash -ne $CloudflaredSHA256) { throw 'cloudflared SHA256 与维护者提供的值不符' }
New-Item -ItemType Directory -Path $package -Force | Out-Null
$previousGOOS=$env:GOOS; $previousGOARCH=$env:GOARCH; $previousCGO=$env:CGO_ENABLED
Push-Location $repo
try {
    $env:GOOS='windows'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
    & $Go build -trimpath -ldflags '-s -w' -o (Join-Path $package 'agentdock-service.exe') './cmd/agentdock-service'
    if ($LASTEXITCODE -ne 0) { throw '服务版编译失败，未生成可交付安装包' }
    Copy-Item -LiteralPath $CloudflaredPath -Destination (Join-Path $package 'cloudflared.exe')
    foreach ($name in @('Install.ps1','Uninstall.ps1')) { Copy-Item -LiteralPath (Join-Path $PSScriptRoot ('headless\'+$name)) -Destination (Join-Path $package $name) }
    Copy-Item -LiteralPath (Join-Path $repo 'docs\windows-headless-service.md') -Destination (Join-Path $package 'README.md')
    Copy-Item -LiteralPath (Join-Path $repo 'LICENSE') -Destination (Join-Path $package 'LICENSE-AgentDock')
    Copy-Item -LiteralPath (Join-Path $repo 'LICENSE') -Destination (Join-Path $package 'LICENSE-cloudflared')
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'headless\THIRD-PARTY-NOTICES.md') -Destination $package
    $hashes=@{};foreach ($name in @('agentdock-service.exe','cloudflared.exe')) { $hashes[$name]=(Get-FileHash -LiteralPath (Join-Path $package $name) -Algorithm SHA256).Hash }
    $hashes | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $package 'SHA256SUMS.json') -Encoding UTF8
    $zip=$package+'.zip';if (Test-Path -LiteralPath $zip) { throw 'ZIP 已存在，拒绝覆盖' }
    Compress-Archive -LiteralPath $package -DestinationPath $zip
    Write-Host ('安装包：'+$zip)
    Write-Host '仅完成构建和打包；未注册服务、未建立隧道、未验证无人登录运行。'
} finally { Pop-Location; $env:GOOS=$previousGOOS; $env:GOARCH=$previousGOARCH; $env:CGO_ENABLED=$previousCGO }
