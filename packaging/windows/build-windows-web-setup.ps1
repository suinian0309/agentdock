[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string] $Version,
    [Parameter(Mandatory = $true)]
    [string] $AgentDockArchive,
    [Parameter(Mandatory = $true)]
    [string] $AgentDockChecksumFile,
    [Parameter(Mandatory = $true)]
    [string] $OutputDirectory
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Resolve-RequiredFile {
    param([string] $Path, [string] $Description)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "$Description was not found: $Path"
    }
    return (Resolve-Path -LiteralPath $Path).Path
}

function Resolve-InnoSetupCompiler {
    $command = Get-Command 'ISCC.exe' -ErrorAction SilentlyContinue
    if ($null -ne $command) { return $command.Source }
    foreach ($candidate in @(
        (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 7\ISCC.exe'),
        (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 7\ISCC.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 6\ISCC.exe')
    )) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
    }
    throw 'Inno Setup 6 compiler was not found.'
}

$archive = Resolve-RequiredFile $AgentDockArchive 'AgentDock archive'
$checksum = Resolve-RequiredFile $AgentDockChecksumFile 'AgentDock checksum file'
$expected = ((Get-Content -LiteralPath $checksum -Raw).Trim() -split '\s+')[0].ToLowerInvariant()
$actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actual -ne $expected) { throw "AgentDock archive SHA-256 mismatch: expected $expected, got $actual" }

$outputRoot = [IO.Path]::GetFullPath($OutputDirectory)
$payloadRoot = Join-Path ([IO.Path]::GetTempPath()) ('agentdock-web-setup-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $outputRoot -Force | Out-Null
New-Item -ItemType Directory -Path $payloadRoot -Force | Out-Null
try {
    Copy-Item -LiteralPath $archive -Destination (Join-Path $payloadRoot 'agentdock_windows_amd64.zip') -Force
    Copy-Item -LiteralPath $checksum -Destination (Join-Path $payloadRoot 'agentdock_windows_amd64.zip.sha256') -Force
    $iscc = Resolve-InnoSetupCompiler
    & $iscc "/DAppVersion=$Version" "/DOutputDir=$outputRoot" "/DPayloadDir=$payloadRoot" (Join-Path $PSScriptRoot 'AgentDockWeb.iss')
    if ($LASTEXITCODE -ne 0) { throw "ISCC failed with exit code $LASTEXITCODE" }
    $setup = Join-Path $outputRoot 'AgentDock-Web-Setup-amd64.exe'
    if (-not (Test-Path -LiteralPath $setup -PathType Leaf)) { throw "Setup was not produced: $setup" }
    Write-Output $setup
} finally {
    Remove-Item -LiteralPath $payloadRoot -Recurse -Force -ErrorAction SilentlyContinue
}
