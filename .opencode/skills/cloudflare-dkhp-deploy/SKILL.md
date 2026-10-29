---
name: cloudflare-dkhp-deploy
description: Kiểm tra, triển khai và xác minh Worker Cloudflare của dự án ĐKHP, kể cả thử tải an toàn với Portal giả lập.
---

# Triển khai Cloudflare ĐKHP

1. Kiểm tra `git status --short --branch`, `wrangler.toml`, `cloudflare-entry.js` và các thay đổi liên quan. Xác nhận đúng Worker và tài khoản bằng `wrangler whoami`; không in hoặc lưu secret.
2. Chạy `node --test tests/*.test.*`. Khi thay đổi backend Go, chạy thêm `go test ./...` và `go vet ./...` trong `go-backend`.
3. Kiểm tra tải bằng Portal giả lập, không gửi đăng ký thật tới trường. Test `tests/browser-registration-retry.test.js` mô phỏng lỗi Portal 30 phút và 60 phiên; đó không phải thử tải production thật.
4. Khi người dùng yêu cầu triển khai, chạy `wrangler deploy` tại root. Không thay đổi binding KV hoặc secrets ngoài yêu cầu. Ghi lại URL và version ID từ output.
5. Kiểm tra URL Worker và `/api/license/config`, đồng thời đối chiếu JS được triển khai với file cục bộ. Không dùng tài khoản thật hoặc tạo đăng ký thật để smoke test.
6. Báo rõ kết quả, phạm vi thử nghiệm, các thay đổi chưa commit và việc tab phải luôn hoạt động cho chế độ đăng ký trên trình duyệt. Triển khai trực tiếp không tự commit hoặc push; Git integration có thể triển khai lại phiên bản theo commit sau này.

Để thử tải thật qua Cloudflare mà không gọi Portal, có thể triển khai Worker thử nghiệm riêng sử dụng gateway và upstream giả lập; dùng tên duy nhất, thống kê các trạng thái dự kiến, sau đó xóa đúng Worker thử nghiệm. Không thay thế Worker chính bằng bản giả lập.
