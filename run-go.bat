@echo off
chcp 65001 > nul
echo ========================================================
echo  🚀 KHỞI ĐỘNG TOOL ĐKHP UTH (GOLANG BACKEND HIGH-PERF)
echo ========================================================
echo.

cd /d "%~dp0"

if not defined PORT set "PORT=3000"
echo [*] Chạy một cổng: http://localhost:%PORT%

if exist "dkhp-tool-golang.exe" (
    echo [*] Tìm thấy dkhp-tool-golang.exe. Đang khởi chạy...
    start "" "http://localhost:%PORT%"
    "dkhp-tool-golang.exe"
) else (
    echo [*] Chưa có file .exe sẵn. Đang khởi chạy trực tiếp qua mã nguồn Go...
    cd /d "%~dp0go-backend"
    start "" "http://localhost:%PORT%"
    go run main.go
)
