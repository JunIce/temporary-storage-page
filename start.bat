@echo off
echo 正在启动 MinIO 文件上传服务器...
echo.

:: 检查是否安装了依赖
if not exist "node_modules" (
    echo 首次运行，正在安装依赖...
    npm install
    if %errorlevel% neq 0 (
        echo 依赖安装失败，请检查网络连接或 npm 配置
        pause
        exit /b 1
    )
    echo 依赖安装完成！
    echo.
)

:: 启动服务器
echo 启动服务器...
echo 服务器将在 http://localhost:3000 运行
echo 按 Ctrl+C 停止服务器
echo.

npm start

pause