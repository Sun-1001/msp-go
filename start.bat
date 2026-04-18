@echo off
chcp 65001 >nul
title MathStudyPlatform - 一键启动

echo ========================================
echo   高等数学智能学习平台 - 一键启动
echo ========================================
echo.

:: 启动 Go 后端 (新窗口)
echo [1/2] 启动 Go 后端服务...
start "Backend - Go API" cmd /k "cd /d %~dp0backend-go && go run ./cmd/api"

:: 等待一秒让后端先启动
timeout /t 2 /nobreak >nul

:: 启动前端 (新窗口)
echo [2/2] 启动前端服务...
start "Frontend - Vite" cmd /k "cd /d %~dp0frontend && npm run dev"

echo.
echo ========================================
echo   启动完成!
echo   前端: http://localhost:5173
echo   后端: http://localhost:8000
echo   健康检查: http://localhost:8000/health
echo   注意: 未迁移的 /api/v1/* 接口会返回 501 占位响应
echo ========================================
echo.
echo 关闭此窗口不会影响已启动的服务。
echo 要停止服务，请关闭对应的命令行窗口。
pause
