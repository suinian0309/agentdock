@echo off
setlocal
cd /d "%~dp0"
"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe" -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0Start-Install.ps1"
if errorlevel 1 (
  echo.
  echo Installer failed. Keep the error message above for troubleshooting.
  pause
)
endlocal
