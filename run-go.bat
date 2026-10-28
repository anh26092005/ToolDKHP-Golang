@echo off
chcp 65001 > nul
echo ========================================================
echo  🚀 KHỞI ĐỘNG TOOL ĐKHP UTH (GOLANG BACKEND HIGH-PERF)
echo ========================================================
echo.

cd /d "%~dp0"

if exist "dkhp-tool-golang.exe" (
    echo [*] Tìm thấy dkhp-tool-golang.exe. Đang khởi chạy...
    start http://localhost:3000
    "dkhp-tool-golang.exe"
) else (
    echo [*] Chưa có file .exe sẵn. Đang khởi chạy trực tiếp qua mã nguồn Go...
    cd /d "%~dp0go-backend"
    start http://localhost:3000
    go run main.go
)