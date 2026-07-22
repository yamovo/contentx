# ContentX 启动脚本
Write-Host ""
Write-Host "========================================"
Write-Host "   ContentX v1.0.0"
Write-Host "   正在启动..."
Write-Host "========================================"
Write-Host ""

# 设置环境变量
$env:SERVER_HOST = "0.0.0.0"
$env:SERVER_PORT = "8080"
$env:SERVER_MODE = "debug"
$env:DB_DRIVER = "sqlite"
$env:DB_NAME = "contentx"
# JWT_SECRET 未设置时 debug 模式会自动生成随机密钥，release 模式会启动失败
$env:JWT_ACCESS_TTL = "15m"
$env:JWT_REFRESH_TTL = "168h"
$env:LIMITS_API_RATE = "300"

Write-Host "[OK] 环境变量已加载" -ForegroundColor Green
Write-Host "[OK] 数据库: $env:DB_DRIVER" -ForegroundColor Green
Write-Host "[OK] 端口: $env:SERVER_PORT" -ForegroundColor Green
Write-Host ""
Write-Host "访问地址: http://localhost:8080" -ForegroundColor Cyan
Write-Host "管理后台: http://localhost:8080/login" -ForegroundColor Cyan
Write-Host "账号: admin / admin123" -ForegroundColor Yellow
Write-Host ""

# 切换到脚本所在目录
Set-Location $PSScriptRoot

# 启动服务
.\contentx.exe
