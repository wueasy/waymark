@echo off
setlocal
cd /d "%~dp0"

echo ================================================
echo   App Template - Frontend (source start)
echo ================================================
echo.

if not exist node_modules (
    echo First run. Installing dependencies...
    call npm install
    if errorlevel 1 goto :error
)

echo Starting frontend dev server (Vite)...
call npm run dev
goto :end

:error
echo.
echo Frontend failed to start. Check the error above.
pause

:end
endlocal
