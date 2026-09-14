@echo off
setlocal
cd /d "%~dp0"

rem Use UTF-8 console codepage so the Go service's Chinese logs render correctly.
chcp 65001 >nul

echo ================================================
echo   App Template - Backend (source start)
echo ================================================
echo.

if not exist config.yaml (
    echo config.yaml not found. Please create it first.
    pause
    exit /b 1
)

echo Starting backend service (Go)...
go run ./cmd/server

echo.
echo Backend service exited.
pause
endlocal
