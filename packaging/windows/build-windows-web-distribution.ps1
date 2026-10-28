#Requires -Version 5.1
[CmdletBinding()]
param([string]$Go='go',[string]$Python='python',[string]$Version='0.9.2-web.5',[string]$OutputRoot='')
$ErrorActionPreference='Stop'
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if (!$OutputRoot) { $OutputRoot=Join-Path $repo 'dist-web-packages' }
$OutputRoot=[IO.Path]::GetFullPath($OutputRoot)
if ($Version -notmatch '^\d+\.\d+\.\d+([-.][a-zA-Z0-9.-]+)?$') { throw '版本格式不正确' }
$payload=Join-Path $OutputRoot 'payload'
if (Test-Path -LiteralPath $OutputRoot) { throw '输出目录已存在，请使用新的输出目录以保护已有产物。' }
New-Item -ItemType Directory -Path $payload -Force | Out-Null
$savedPath=$env:PATH;$savedOS=$env:GOOS;$savedArch=$env:GOARCH;$savedCGO=$env:CGO_ENABLED
Push-Location $repo
try {
    $resolvedGo=(Get-Command $Go -ErrorAction Stop).Source
    $env:PATH=(Split-Path $resolvedGo)+';'+$env:PATH
    $env:GOOS='windows';$env:GOARCH='amd64';$env:CGO_ENABLED='0'
    $flags="-s -w -X github.com/uvwt/agentdock/internal/buildinfo.Version=$Version"
    foreach ($command in @('agentdock','agentdock-arbiter','agentdock-shim','agentdock-tray')) {
        $commandFlags=$flags;if($command -eq 'agentdock-tray'){$commandFlags+=' -H=windowsgui'}
        & $resolvedGo build -trimpath -ldflags $commandFlags -o (Join-Path $payload ($command+'.exe')) ('./cmd/'+$command)
        if($LASTEXITCODE -ne 0){throw "$command 编译失败"}
    }
    & $resolvedGo build -trimpath -ldflags ($flags+' -H=windowsgui') -o (Join-Path $payload 'agentdock-tray-shim.exe') './cmd/agentdock-shim'
    if($LASTEXITCODE -ne 0){throw 'tray shim 编译失败'}
    & (Join-Path $PSScriptRoot 'build-wsl-helper-payload.ps1') -OutputDirectory (Join-Path $payload 'wsl-helper')
    & $Python (Join-Path $repo 'packaging\build-core-skill-bundle.py') --output (Join-Path $payload 'share\agentdock\core-skills')
    if($LASTEXITCODE -ne 0){throw '核心 Skill 打包失败'}
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'assets\agentdock.ico') -Destination $payload
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'compat\manage-windows.ps1') -Destination $payload
    $zip=Join-Path $OutputRoot 'agentdock_windows_amd64.zip'
    Compress-Archive -Path (Join-Path $payload '*') -DestinationPath $zip
    $hash=(Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText(($zip+'.sha256'),$hash+'  agentdock_windows_amd64.zip'+"`n",[Text.UTF8Encoding]::new($false))
    & (Join-Path $PSScriptRoot 'build-windows-web-setup.ps1') -Version $Version -AgentDockArchive $zip -AgentDockChecksumFile ($zip+'.sha256') -OutputDirectory $OutputRoot
    Write-Host 'Web版仅编译打包，未执行安装或浏览器测试。'
} finally {Pop-Location;$env:PATH=$savedPath;$env:GOOS=$savedOS;$env:GOARCH=$savedArch;$env:CGO_ENABLED=$savedCGO}
