; PVM Inno Setup 安装脚本
; 版本: 1.0.0

#define MyAppName "PVM"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "lucky-zsh"
#define MyAppURL "https://gitee.com/lucky-zsh/pvm"
#define MyAppExeName "pvm.exe"

[Setup]
; 应用信息
AppId={{C5D7A8E2-4B3F-4A1E-9C8D-2E5F6A7B8C9D}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}/releases

; 安装目录
DefaultDirName={userappdata}\pvm
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes

; 输出设置
OutputDir=..\dist
OutputBaseFilename=pvm-{#MyAppVersion}-windows-amd64-setup
Compression=lzma2/ultra64
SolidCompression=yes

; 安装模式
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog

; 界面设置
WizardStyle=modern
; SetupIconFile=..\docs\docs\public\pvm-logo.ico

; 版本信息
VersionInfoVersion={#MyAppVersion}
VersionInfoCompany={#MyAppPublisher}
VersionInfoDescription=PVM - Polyglot Version Manager
VersionInfoCopyright=Copyright (c) 2026 {#MyAppPublisher}
VersionInfoProductName={#MyAppName}
VersionInfoProductVersion={#MyAppVersion}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "addtopath"; Description: "添加到系统 PATH 环境变量"; GroupDescription: "环境配置:"; Flags: checkedonce

[Files]
Source: "..\dist\pvm-windows-amd64.exe"; DestDir: "{app}"; DestName: "pvm.exe"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\PVM 命令行"; Filename: "{cmd}"; Parameters: "/k ""{app}\pvm.exe"" --help"; WorkingDir: "{app}"
Name: "{group}\卸载 PVM"; Filename: "{uninstallexe}"

[Registry]
; 添加到用户 PATH
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Tasks: addtopath; Check: NeedsAddPath('{app}')

[Run]
; 安装后运行 setup
Filename: "{app}\pvm.exe"; Parameters: "setup"; Description: "运行 pvm setup 初始化"; Flags: nowait postinstall skipifsilent runhidden

[Code]
// 检查路径是否已存在于 PATH 中
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

// 卸载时清理 PATH
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Path: string;
  AppPath: string;
  P: Integer;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    if RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Path) then
    begin
      AppPath := ExpandConstant('{app}');
      P := Pos(';' + AppPath, Path);
      if P > 0 then
      begin
        Delete(Path, P, Length(';' + AppPath));
        RegWriteStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Path);
      end
      else
      begin
        P := Pos(AppPath + ';', Path);
        if P > 0 then
        begin
          Delete(Path, P, Length(AppPath + ';'));
          RegWriteStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Path);
        end;
      end;
    end;
  end;
end;
