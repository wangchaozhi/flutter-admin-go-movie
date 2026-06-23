@echo off
setlocal

set "SCRIPT_DIR=%~dp0"

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%start-dev.ps1" -Restart %*
set "EXIT_CODE=%ERRORLEVEL%"

if not "%EXIT_CODE%"=="0" (
  echo.
  echo start-dev.ps1 failed with exit code %EXIT_CODE%.
)

echo.
pause

exit /b %EXIT_CODE%
