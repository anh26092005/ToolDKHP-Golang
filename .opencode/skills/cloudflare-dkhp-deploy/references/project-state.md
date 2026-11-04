# Thông tin dự án và mốc triển khai đã xác minh

Đây là mốc tham khảo được lưu ngày **2026-11-04**. Luôn kiểm tra lại Git, cấu hình và trạng thái Cloudflare khi dùng skill; không coi các ID bên dưới là phiên bản hiện tại mãi mãi.

## Đích triển khai

- Repo: https://github.com/anh26092005/ToolDKHP-Golang
- Remote: `origin`; nhánh tại mốc này: `main`.
- Worker chính: `tooldkhp-golang`.
- Cấu hình: `wrangler.toml` tại root; entrypoint `cloudflare-entry.js`.
- Static assets: `./public`, binding `ASSETS`.
- URL đã xác minh: https://tooldkhp-golang.ngocanhproxh20000.workers.dev
- `worker/` là Worker quản lý key riêng; `my-app/` là template Worker, không phải đích triển khai ứng dụng chính.

## Kiến trúc và hành vi cần giữ

- Go chỉ mở một cổng, mặc định `3000`; `PORT` chọn cổng khác. `run-go.bat` dùng cùng cổng khi mở trình duyệt.
- Bản Windows: `dkhp-tool-golang.exe` tại root, build từ `go-backend`; file này được `.gitignore` bỏ qua.
- Cloudflare chạy frontend và gateway JavaScript, không chạy file Go `.exe`. Vòng lặp đăng ký của giao diện nằm trong tab.
- Mỗi request Go giới hạn 15 giây; API client trình duyệt chờ tối đa 20 giây cho mỗi request.
- Auto sự kiện giữ lượt nghiệp vụ khi timeout/408/429/5xx; retry có backoff và tự đăng nhập lại khi có credentials đã ghi nhớ.
- Thiếu credentials hoặc tài khoản/mật khẩu bị từ chối thì báo cần đăng nhập lại. Worker Go gọi trực tiếp phải nhận `credentials` nếu cần refresh token.
- Mã phiên sự kiện Go băm toàn bộ token để JWT có cùng phần đầu không bị gom nhầm phiên; token khởi tạo vẫn tìm được phiên sau refresh.

## Mốc triển khai ngày 2026-11-04

- Commit: `753a21dc95791d67cc53997f9475005b74253322`.
- Nội dung: `Fix Portal outage recovery and default Go to a single port`.
- Cloudflare version ID: `1b15228a-5688-4318-9fb1-29f159183f8c`.
- Kiểm tra tại mốc này: 37 test JavaScript, 13 hàm test Go cấp cao nhất cùng các subtest, `go vet` và kiểm tra cú pháp đạt.
- File `.exe` chạy thử với `PORT` không được đặt chỉ có một listener trên 3000; thử cổng tùy chỉnh cũng chỉ có một listener.
- Cloudflare: HTML và ba file JavaScript trả HTTP 200, SHA-256 khớp file cục bộ; `/api/license/config` trả HTTP 200 với `required: false`.
- Gián đoạn 30 phút và 60 phiên được kiểm tra bằng Portal/đồng hồ giả lập; không phải thử tải thực tế Portal trường.
