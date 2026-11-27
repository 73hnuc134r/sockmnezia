; sockmnezia desktop installer: per-user (no administrator rights), %LOCALAPPDATA%\Programs\sockmnezia.
; makensis -DVERSION=0.1.0 -DEXE=path\to\sockmnezia.exe -DICON=appicon.ico -DOUTFILE=setup.exe installer.nsi
Unicode true
!include "MUI2.nsh"
!include "LogicLib.nsh"

Name "sockmnezia"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\sockmnezia"
InstallDirRegKey HKCU "Software\sockmnezia" "InstallDir"
RequestExecutionLevel user
SetCompressor /SOLID lzma
!define MUI_ICON "${ICON}"
!define MUI_UNICON "${ICON}"
!define UNINST "Software\Microsoft\Windows\CurrentVersion\Uninstall\sockmnezia"
VIProductVersion "${NUMERIC}"
VIAddVersionKey "ProductName" "sockmnezia"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "sockmnezia installer"
VIAddVersionKey "LegalCopyright" "MIT License"

!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\sockmnezia.exe"
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "Russian"

Section "Install"
  ; Quit a running copy first (it restores the system proxy settings on quit).
  nsExec::Exec 'taskkill /IM sockmnezia.exe'
  Sleep 1000
  SetOutPath "$INSTDIR"
  File "/oname=sockmnezia.exe" "${EXE}"
  File "/oname=wintun.dll" "${WINTUN}"
  File "/oname=wintun-LICENSE.txt" "${WINTUNLICENSE}"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  CreateShortCut "$SMPROGRAMS\sockmnezia.lnk" "$INSTDIR\sockmnezia.exe"
  WriteRegStr HKCU "Software\sockmnezia" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${UNINST}" "DisplayName" "sockmnezia"
  WriteRegStr HKCU "${UNINST}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST}" "DisplayIcon" "$INSTDIR\sockmnezia.exe"
  WriteRegStr HKCU "${UNINST}" "Publisher" "sockmnezia"
  WriteRegStr HKCU "${UNINST}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKCU "${UNINST}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST}" "NoRepair" 1
SectionEnd

Section "Uninstall"
  ; The TUN service (installed by the app for TUN mode) runs from its own copy; removing it needs
  ; administrator rights: one prompt, only when it is installed.
  nsExec::ExecToStack 'sc query sockmnezia-tun'
  Pop $0
  ${If} $0 == 0
    ExecShellWait "runas" "$INSTDIR\sockmnezia.exe" "--tun-uninstall" SW_HIDE
  ${EndIf}
  nsExec::Exec 'taskkill /IM sockmnezia.exe'
  Sleep 1000
  Delete "$INSTDIR\sockmnezia.exe"
  Delete "$INSTDIR\wintun.dll"
  Delete "$INSTDIR\wintun-LICENSE.txt"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\sockmnezia.lnk"
  ; Login item, if the user turned it on (the app's data in %LOCALAPPDATA%\sockmnezia stays).
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "sockmnezia"
  DeleteRegKey HKCU "${UNINST}"
  DeleteRegKey HKCU "Software\sockmnezia"
SectionEnd
