@echo off
chcp 65001 >nul
cls
echo ==============================================
echo  Go 项目一键编译 + 打包 (含config.yaml)
echo  输出格式：node-agent-OS-ARCH.zip
echo ==============================================
echo.

:: 项目名称
set APP_NAME=node-agent

:: 清空旧包
del /q %APP_NAME%-*.zip 2>nul

:: ==============================================
:: 编译平台 1：linux amd64
:: ==============================================
set GOOS=linux
set GOARCH=amd64
set OUTPUT=%APP_NAME%-%GOOS%-%GOARCH%

echo [1/4] 编译：%OUTPUT%
set CGO_ENABLED=0
 go build -trimpath -ldflags="-s -w" -o %APP_NAME% ./cmd/agent

:: 创建临时目录
mkdir temp
move %APP_NAME% temp\ >nul
copy configs\config.example.yaml temp\config.yaml >nul

:: 打包zip
powershell -Command "Compress-Archive -Path 'temp\*' -DestinationPath '%OUTPUT%.zip' -Force"

:: 清理
rmdir /s /q temp

:: ==============================================
:: 编译平台 2：linux arm64
:: ==============================================
set GOOS=linux
set GOARCH=arm64
set OUTPUT=%APP_NAME%-%GOOS%-%GOARCH%

echo [2/4] 编译：%OUTPUT%
set CGO_ENABLED=0
go build -trimpath -ldflags="-s -w" -o %APP_NAME% ./cmd/agent

:: 创建临时目录
mkdir temp
move %APP_NAME% temp\ >nul
copy configs\config.example.yaml temp\config.yaml >nul

:: 打包zip
powershell -Command "Compress-Archive -Path 'temp\*' -DestinationPath '%OUTPUT%.zip' -Force"

:: 清理
rmdir /s /q temp

echo.
echo ==============================================
echo ✅ 编译打包完成！
echo 生成文件：
dir %APP_NAME%-*.zip
echo ==============================================
pause