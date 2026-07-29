# ContentX 启动脚本 (PowerShell)
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

Write-Host ""
Write-Host "========================================"
Write-Host "   ContentX v1.2.0"
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
$env:ADMIN_PASSWORD = "admin123"

Write-Host "[OK] 环境变量已加载" -ForegroundColor Green
Write-Host "[OK] 数据库: $env:DB_DRIVER" -ForegroundColor Green
Write-Host "[OK] 端口: $env:SERVER_PORT" -ForegroundColor Green
Write-Host ""
Write-Host "访问地址: http://localhost:8080" -ForegroundColor Cyan
Write-Host "管理后台: http://localhost:8080/login" -ForegroundColor Cyan
Write-Host "账号: admin / admin123" -ForegroundColor Yellow
Write-Host ""

# 后台等待端口就绪后打开浏览器（避免编译期间浏览器先打开看到空白页）
Start-Job -ScriptBlock {
    for ($i = 0; $i -lt 60; $i++) {
        try {
            $c = New-Object Net.Sockets.TcpClient
            $c.Connect('127.0.0.1', 8080)
            $c.Close()
            Start-Process 'http://localhost:8080'
            break
        } catch {
            Start-Sleep -Seconds 1
        }
    }
} | Out-Null

# 优先使用预编译二进制；不存在则自动 go run
$binary = Join-Path $PSScriptRoot "bin\contentx.exe"
if (Test-Path $binary) {
    Write-Host "Starting prebuilt binary: bin\contentx.exe" -ForegroundColor Green
    & $binary
} else {
    Write-Host "Binary not found, falling back to: go run ./cmd/server" -ForegroundColor Yellow
    go run ./cmd/server
}
