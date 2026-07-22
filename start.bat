@echo off
cd /d "%~dp0"

echo.
echo ========================================
echo    ContentX v1.0.0
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
rem JWT_SECRET 未设置时 debug 模式会自动生成随机密钥，release 模式会启动失败

echo [OK] Environment loaded
echo [OK] Database: %DB_DRIVER%
echo [OK] Port: %SERVER_PORT%
echo.
echo URL: http://localhost:8080
echo Login: http://localhost:8080/login
echo Account: admin / admin123
echo.

server.exe

pause
