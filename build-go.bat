@echo off
chcp 65001 > nul
echo ========================================================
echo  🛠️  BIÊN DỊCH TOOL ĐKHP GOLANG SANG FILE THỰC THI .EXE
echo ========================================================
echo.

cd /d "%~dp0go-backend"

echo [*] Đang kiểm tra mã nguồn Go và tải dependencies...
go mod download
if %ERRORLEVEL% neq 0 (
    echo [x] Lỗi: Không thể tải dependencies Go. Vui lòng kiểm tra kết nối mạng và Go SDK.
    pause
    exit /b %ERRORLEVEL%
)

echo [*] Đang biên dịch tối ưu hóa (stripping debug info)...
go build -ldflags "-s -w" -o "%~dp0dkhp-tool-golang.exe" main.go
if %ERRORLEVEL% neq 0 (
    echo [x] Lỗi biên dịch thất bại!
    pause
    exit /b %ERRORLEVEL%
)

echo.
echo ========================================================
echo  ✅ BIÊN DỊCH THÀNH CÔNG!
echo  📁 File thực thi: %~dp0dkhp-tool-golang.exe
echo ========================================================
echo.
pause
