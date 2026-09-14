@echo off
setlocal
set ROOT=%~dp0
set FRONTEND=%ROOT%frontend
set BACKEND=%ROOT%backend
set WEB_DIST=%BACKEND%\internal\web\dist
set OUTPUT=%ROOT%release

echo ================================================
echo   Waymark - Build Script
echo ================================================
echo.

echo [1/4] Building frontend...
cd /d "%FRONTEND%"
if not exist node_modules (
    echo   Installing frontend dependencies...
    call npm install
    if errorlevel 1 goto :error
)
call npm run build
if errorlevel 1 goto :error
echo   Done.

echo [2/4] Syncing frontend output to embed dir...
if exist "%WEB_DIST%" rmdir /s /q "%WEB_DIST%"
mkdir "%WEB_DIST%"
xcopy /e /i /y "%FRONTEND%\dist" "%WEB_DIST%" >nul
if errorlevel 1 goto :error
echo   Done.

echo [3/4] Cross-compiling Go backend...
cd /d "%BACKEND%"
if not exist "%OUTPUT%" mkdir "%OUTPUT%"

echo   - Windows amd64
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -o "%OUTPUT%\waymark-windows-amd64.exe" ./cmd/server
if errorlevel 1 goto :error

echo   - Linux amd64
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -o "%OUTPUT%\waymark-linux-amd64" ./cmd/server
if errorlevel 1 goto :error

echo   - macOS amd64
set GOOS=darwin
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -o "%OUTPUT%\waymark-darwin-amd64" ./cmd/server
if errorlevel 1 goto :error

echo   - macOS arm64
set GOOS=darwin
set GOARCH=arm64
set CGO_ENABLED=0
go build -trimpath -o "%OUTPUT%\waymark-darwin-arm64" ./cmd/server
if errorlevel 1 goto :error

echo   - Linux arm64
set GOOS=linux
set GOARCH=arm64
set CGO_ENABLED=0
go build -trimpath -o "%OUTPUT%\waymark-linux-arm64" ./cmd/server
if errorlevel 1 goto :error

echo [4/4] Build finished. Output dir: %OUTPUT%
echo.
dir /b "%OUTPUT%"
echo.
echo Done.
exit /b 0

:error
echo.
echo Build failed. Please check the error messages above.
exit /b 1
