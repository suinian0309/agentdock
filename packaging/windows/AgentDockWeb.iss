#ifndef AppVersion
#define AppVersion "0.0.0"
#endif

#ifndef OutputDir
#define OutputDir "..\..\dist-web"
#endif

#ifndef PayloadDir
#define PayloadDir "..\..\dist-web\payload"
#endif

[Setup]
AppId={{D6788C7A-4104-48D4-B5C3-F4858B5606EA}
AppName=AgentDock Web
AppVersion={#AppVersion}
AppPublisher=AgentDock
AppPublisherURL=https://github.com/suinian0309/agentdock
DefaultDirName={localappdata}\AgentDock
DefaultGroupName=AgentDock
DisableProgramGroupPage=yes
DisableDirPage=yes
PrivilegesRequired=lowest
OutputDir={#OutputDir}
OutputBaseFilename=AgentDock-Web-Setup-amd64
SetupIconFile=assets\agentdock.ico
UninstallDisplayIcon={app}\installer\agentdock.ico
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no
SetupLogging=yes
ArchitecturesAllowed=x64compatible

[Languages]
Name: "chinesesimplified"; MessagesFile: "compiler:Default.isl,languages\ChineseSimplified.isl"

[Files]
Source: "..\..\scripts\install\install.ps1"; Flags: dontcopy
Source: "{#PayloadDir}\agentdock_windows_amd64.zip"; Flags: dontcopy
Source: "{#PayloadDir}\agentdock_windows_amd64.zip.sha256"; Flags: dontcopy
Source: "..\..\scripts\install\uninstall-windows.ps1"; DestDir: "{app}\installer"; Flags: ignoreversion
Source: "assets\agentdock.ico"; DestDir: "{app}\installer"; Flags: ignoreversion

[Icons]
Name: "{group}\AgentDock Web 管理面板"; Filename: "{app}\bin\agentdock-tray.exe"; WorkingDir: "{app}"; IconFilename: "{app}\installer\agentdock.ico"
Name: "{userdesktop}\AgentDock Web 管理面板"; Filename: "{app}\bin\agentdock-tray.exe"; WorkingDir: "{app}"; IconFilename: "{app}\installer\agentdock.ico"

[Run]
Filename: "{app}\bin\agentdock-tray.exe"; Description: "打开 AgentDock Web 管理面板"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File ""{app}\installer\uninstall-windows.ps1"" -InstallDir ""{app}\bin"" -KeepInstallDir"; Flags: runhidden waituntilterminated; RunOnceId: "AgentDockWebCleanup"

[UninstallDelete]
Type: filesandordirs; Name: "{app}"
Type: files; Name: "{userdesktop}\AgentDock Web 管理面板.lnk"

[Code]
function QuoteArgument(const Value: String): String;
begin
  Result := '"' + Value + '"';
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  PowerShellPath: String;
  Parameters: String;
  ExitCode: Integer;
begin
  Result := '';
  ExtractTemporaryFile('install.ps1');
  ExtractTemporaryFile('agentdock_windows_amd64.zip');
  ExtractTemporaryFile('agentdock_windows_amd64.zip.sha256');
  PowerShellPath := ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe');
  Parameters :=
    '-NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File ' +
    QuoteArgument(ExpandConstant('{tmp}\install.ps1')) +
    ' -OfflineArchive ' + QuoteArgument(ExpandConstant('{tmp}\agentdock_windows_amd64.zip')) +
    ' -OfflineChecksumFile ' + QuoteArgument(ExpandConstant('{tmp}\agentdock_windows_amd64.zip.sha256')) +
    ' -InstallDir ' + QuoteArgument(ExpandConstant('{app}\bin')) +
    ' -RegisterStartup -StartupMode preserve -TunnelMode none -InstallChannel script';
  if not Exec(PowerShellPath, Parameters, '', SW_HIDE, ewWaitUntilTerminated, ExitCode) then
  begin
    Result := '无法启动 AgentDock 安装程序。';
    Exit;
  end;
  if ExitCode <> 0 then
    Result := 'AgentDock 安装失败，错误代码：' + IntToStr(ExitCode) + '。请查看安装日志。';
end;
