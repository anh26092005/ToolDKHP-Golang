---
name: cloudflare-dkhp-deploy
description: Kiểm tra Go một cổng, push Git khi được yêu cầu, triển khai và xác minh Cloudflare của dự án ĐKHP với Portal giả lập.
---

# Go một cổng → Kiểm tra → Git → Cloudflare

## Phạm vi và thông tin dự án

- Đọc `references/project-state.md` để biết kiến trúc, đích triển khai và mốc triển khai đã xác minh. Luôn đối chiếu lại cấu hình hiện tại trước khi thao tác.
- Các đường dẫn mã nguồn bên dưới tính từ root repo, nơi có `wrangler.toml`. Chạy lệnh tại root trừ khi ghi rõ `go-backend`.
- Khi người dùng yêu cầu toàn bộ quy trình, thực hiện theo thứ tự: **một cổng → kiểm tra/build → commit/push → deploy → xác minh**.
- Chỉ commit/push khi người dùng yêu cầu Git; chỉ deploy khi người dùng yêu cầu triển khai. Yêu cầu lưu hoặc chỉnh skill chỉ cập nhật tài liệu skill.

## 1. Kiểm tra đầu vào

1. Đọc `git status --short --branch`, `git remote -v`, `wrangler.toml`, `cloudflare-entry.js` và diff liên quan. Xác định các thay đổi thuộc công việc hiện tại trước khi stage.
2. Xác nhận Worker và tài khoản bằng `wrangler whoami`. Giữ nguyên binding và secrets ngoài phạm vi yêu cầu; không in hoặc lưu secret.
3. Phân biệt backend Go với Cloudflare Worker: Cloudflare phục vụ frontend và gateway JavaScript; vòng lặp đăng ký trên giao diện chạy trong tab. Go còn có API worker chạy nền riêng.

## 2. Go chỉ mở một cổng

1. Kiểm tra `go-backend/main.go`, `go-backend/config/config.go` và `run-go.bat`.
2. Mặc định chỉ có một HTTP server trên **3000**. Nếu có `PORT`, chỉ dùng cổng đó; giá trị hợp lệ từ 1 đến 65535. Không khôi phục chế độ mở 3000–3009.
3. Khi sửa Go, chạy trong `go-backend`:

   ```powershell
   go test -timeout 30s ./...
   go vet ./...
   go build -ldflags "-s -w" -o ..\dkhp-tool-golang.exe main.go
   ```

4. Khởi chạy file vừa build với `PORT` không được đặt, kiểm tra trang chủ và JavaScript trả HTTP 200. Dùng `Get-NetTCPConnection -OwningProcess <PID> -State Listen` để xác nhận đúng **một listener trên 3000**.
5. Nếu kiểm tra tùy chỉnh `PORT`, dừng tiến trình test trước rồi chạy trên một cổng trống và xác nhận chỉ có một listener. Không dừng tiến trình khác đang chiếm cổng; dọn đúng tiến trình test đã mở.
6. So sánh file phục vụ bằng SHA-256 của **byte gốc**. PowerShell có thể giải mã tiếng Việt khác UTF-8; tránh kết luận mã cũ chỉ vì tìm chuỗi tiếng Việt trong `.Content` thất bại.
7. `*.exe` được Git bỏ qua; báo rõ file build nằm cục bộ, mã nguồn được đẩy lên Git.

## 3. Kiểm tra frontend và phục hồi

```powershell
node --test tests/*.test.*
node --check public/js/api.js
node --check public/js/app.js
node --check public/js/events.js
git diff --check
```

- Xem `tests/browser-events.test.js`, `tests/browser-registration-retry.test.js` và các test Go về timeout/refresh/hủy request khi sửa luồng phục hồi.
- Kiểm tra lỗi tạm thời không trừ lượt nghiệp vụ, login lỗi có backoff, token mới được dùng lại và nút Dừng ngăn lượt gửi tiếp theo.
- Các tình huống Portal lỗi 30 phút dùng đồng hồ giả lập; 60 phiên độc lập dùng Portal giả lập. Báo đúng kết quả thực chạy, không cố định số test theo mốc cũ.
- Với thử tải thật qua Cloudflare, chỉ dùng Worker thử nghiệm riêng với upstream giả lập; ghi nhận số phiên, request, lỗi và độ trễ, rồi xóa đúng Worker thử nghiệm.

## 4. Commit và push khi được yêu cầu

1. `git fetch origin`, đối chiếu nhánh và remote hiện tại trước khi push.
2. Stage rõ các file thuộc thay đổi đã kiểm tra, chạy `git diff --cached --check` và xem `git diff --cached --stat`.
3. Commit với mô tả phù hợp rồi push nhánh được yêu cầu. Không force-push hoặc ghi đè công việc khác.
4. Ghi nhận commit SHA và kết quả push; kiểm tra lại `git status --short --branch`.

## 5. Triển khai và xác minh Cloudflare

1. Khi được yêu cầu triển khai, chạy `wrangler deploy` tại root bằng cấu hình chính. Không dùng cấu hình `worker/` quản lý key hoặc template `my-app/`.
2. Ghi lại URL và version ID từ output triển khai thực tế.
3. Dùng request chỉ đọc kiểm tra `/` và `/api/license/config`; endpoint cấu hình hiện trả `{ "required": false }`.
4. Tải `/js/api.js`, `/js/app.js`, `/js/events.js` với query phiên bản đang có trong HTML và so SHA-256 với file cục bộ; đối chiếu cả HTML khi cần.
5. Ưu tiên smoke test bằng HTTP. Nếu cần xem giao diện, dùng hồ sơ/ngữ cảnh trình duyệt sạch đã xác nhận hỗ trợ; hồ sơ mặc định có thể tự dùng tài khoản đã lưu và gọi Portal khi mở trang. Nếu không tạo được ngữ cảnh sạch, tiếp tục xác minh bằng HTTP và test giả lập.
6. Không dùng tài khoản thật hoặc tạo đăng ký thật để smoke test. Không coi lỗi xác thực của phiên cũ là bằng chứng deploy thất bại.
7. Có thể kiểm tra `wrangler deployments list` để xác nhận version đang nhận traffic, nhất là khi có Git integration tự triển khai sau push.

## 6. Báo kết quả

- Đưa link Cloudflare, repo/commit, version ID, trạng thái một cổng và vị trí file `.exe` đã build.
- Báo số test đạt/thất bại, endpoint HTTP và kết quả so khớp nội dung, trạng thái Git, các bước chưa hoàn thành nếu có.
- Nhắc tải lại bằng Ctrl+F5, bật Ghi nhớ nếu cần tự đăng nhập lại, giữ tab/trình duyệt hoạt động và máy không ngủ khi chạy tự động trên giao diện.
- Phân biệt kết quả giả lập với khả năng chịu tải thực tế của Portal trường.
