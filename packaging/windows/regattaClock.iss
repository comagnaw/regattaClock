; Inno Setup script for the regattaClock per-user Windows installer.
;
; Built in .github/workflows/release.yml with:
;   iscc /DMyAppVersion=<version> /DMyAppVersionNumeric=<x.y.z> /DMySrcExe=<regattaClock.exe> packaging\windows\regattaClock.iss
; MyAppVersion is the display/filename string (may be a pre-release like
; 0.5.1-alpha); MyAppVersionNumeric is the plain x.y.z for VERSIONINFO.
;
; Per-user install (no UAC), Start Menu shortcut, proper uninstaller. Config
; lives in the Windows registry via Fyne Preferences, so there is nothing else
; to clean up on removal.

#ifndef MyAppVersion
  #define MyAppVersion "0.0.0"
#endif
; Numeric x.y.z for the installer's own VERSIONINFO (Inno rejects a pre-release
; string there). Defaults to MyAppVersion when the caller does not pass it.
#ifndef MyAppVersionNumeric
  #define MyAppVersionNumeric MyAppVersion
#endif
#ifndef MySrcExe
  #define MySrcExe "regattaClock.exe"
#endif

#define MyAppName "regattaClock"
#define MyAppPublisher "comagnaw"
#define MyAppURL "https://github.com/comagnaw/regattaClock"
#define MyAppExeName "regattaClock.exe"

[Setup]
; AppId is the permanent identity for upgrades and uninstall - never change it.
AppId={{B746DB1C-49C4-4A3A-8EE2-22CE6F8042A2}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
VersionInfoVersion={#MyAppVersionNumeric}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}/releases
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputBaseFilename=regattaClock-{#MyAppVersion}-windows-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#MySrcExe}"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent
