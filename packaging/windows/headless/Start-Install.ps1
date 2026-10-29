#Requires -Version 5.1
# ASCII-only bootstrap: readable by Windows PowerShell 5.1 on all system locales.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (!$principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    try {
        $entry = $PSCommandPath.Replace("'", "''")
        $command = "& '$entry'"
        $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
        $powershell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
        Start-Process -FilePath $powershell -Verb RunAs -ArgumentList @('-NoLogo','-NoProfile','-ExecutionPolicy','Bypass','-EncodedCommand',$encoded) | Out-Null
        exit 0
    } catch {
        Write-Host ('Could not open administrator installer: ' + $_.Exception.Message) -ForegroundColor Red
        exit 1
    }
}
$result = 0
try {
    & (Join-Path $PSScriptRoot 'Install.ps1')
    Write-Host 'Installation command finished. Review the messages above.' -ForegroundColor Green
} catch {
    $result = 1
    Write-Host ''
    Write-Host 'Installation failed. The window will stay open.' -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
    if ($_.InvocationInfo.PositionMessage) { Write-Host $_.InvocationInfo.PositionMessage }
} finally {
    Write-Host ''
    [void](Read-Host 'Press Enter to close this window')
}
exit $result
