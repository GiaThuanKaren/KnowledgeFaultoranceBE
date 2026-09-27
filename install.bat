@echo off
setlocal
set "PORT=%~1"
if "%PORT%"=="" set "PORT=5005"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\install-service.ps1" -Port "%PORT%"
pause
