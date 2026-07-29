@echo off
cd /d "%~dp0"

echo.
echo ========================================
echo    ContentX v1.2.0  (dev mode)
echo ========================================
echo.

set SERVER_HOST=0.0.0.0
set SERVER_PORT=8080
set SERVER_MODE=debug
set DB_DRIVER=sqlite
set DB_NAME=contentx
set JWT_ACCESS_TTL=15m
set JWT_REFRESH_TTL=168h
set LIMITS_API_RATE=300
set ADMIN_PASSWORD=admin123
rem JWT_SECRET auto-generated in debug mode; required in release mode

echo [OK] Environment loaded
echo [OK] Database: %DB_DRIVER%
echo [OK] Port: %SERVER_PORT%
echo.

rem Wait for port to be ready, then open browser (go run takes 10-30s to compile)
start "" /b powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='SilentlyContinue'; for ($i=0; $i -lt 60; $i++) { $c = New-Object Net.Sockets.TcpClient; try { $c.Connect('127.0.0.1',8080); $c.Close(); Start-Process 'http://localhost:8080'; break } catch { Start-Sleep -Seconds 1 } }"

echo URL: http://localhost:8080
echo Login: http://localhost:8080/login
echo Account: admin / %ADMIN_PASSWORD%
echo.
echo Waiting for server to start, browser will open automatically...
echo.

go run ./cmd/server

pause
