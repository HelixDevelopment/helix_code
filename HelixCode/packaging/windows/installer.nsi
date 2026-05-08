!define PRODUCT_NAME "HelixCode"
!define PRODUCT_VERSION "3.0.0"
!define PRODUCT_PUBLISHER "Helix Development"

Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
OutFile "helixcode-${PRODUCT_VERSION}-setup.exe"
InstallDir "$PROGRAMFILES64\${PRODUCT_NAME}"

Section "Install"
  SetOutPath "$INSTDIR"
  File "bin\helixcode.exe"
  File "config\config.yaml"
  CreateDirectory "$APPDATA\HelixCode"
  CreateShortCut "$SMPROGRAMS\HelixCode.lnk" "$INSTDIR\helixcode.exe"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  EnVar::SetHKLM
  EnVar::AddValue "PATH" "$INSTDIR"
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\helixcode.exe"
  Delete "$INSTDIR\config.yaml"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\HelixCode.lnk"
  EnVar::DeleteValue "PATH" "$INSTDIR"
SectionEnd
