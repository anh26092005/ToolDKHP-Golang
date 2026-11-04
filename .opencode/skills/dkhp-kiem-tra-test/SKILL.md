---
name: dkhp-kiem-tra-test
description: Kiểm tra ĐKHP Go một cổng, auto sự kiện/học phần, tự đăng nhập lại khi Portal lỗi và 60 phiên độc lập với upstream giả lập.
---

# Kiểm tra và test ĐKHP

1. Xác định phiên bản, thay đổi hiện tại (`git status --short --branch`) và kiến trúc: Cloudflare Worker phục vụ frontend và gateway; tiến trình đăng ký chạy trong tab người dùng.
2. Tại root chạy `node --test tests/*.test.*`. Khi sửa frontend, kiểm tra cú pháp cả `public/js/api.js`, `public/js/app.js`, `public/js/events.js` bằng `node --check`. Khi cần kiểm tra Go, chạy `go test -timeout 30s ./...` và `go vet ./...` trong `go-backend`; chạy `go test -race ./...` nếu CGO và trình biên dịch C có sẵn.
3. Xem `tests/browser-registration-retry.test.js`, `tests/browser-events.test.js`, `go-backend/services/portal_test.go` và `go-backend/handlers/recovery_test.go`. Kiểm tra timeout/408/429/5xx không trừ lượt nghiệp vụ, login cũng lỗi rồi hồi phục, dùng token mới, các phiên độc lập, giữ nguyên ID/payload và Dừng trong lúc request/login/backoff. Các test có tình huống gián đoạn 30 phút và 60 phiên bằng Portal/đồng hồ giả lập; nói rõ đây không phải chạy 30 phút thực hay thử tải Portal thật.
4. Khi sửa cách chạy Go, build lại `.exe` rồi xác nhận đúng một listener thuộc PID test: mặc định 3000 khi không đặt `PORT`, hoặc đúng cổng tùy chỉnh. Dừng đúng tiến trình test khi xong. So khớp file JavaScript bằng byte/SHA-256 để tránh lỗi giải mã tiếng Việt của PowerShell.
5. Nếu được yêu cầu kiểm tra Cloudflare đang chạy, kiểm tra trang chủ và `/api/license/config` bằng request chỉ đọc, so khớp JavaScript trên Worker với file cục bộ. Không dùng tài khoản thật hoặc gửi đăng ký thật. Nếu xem giao diện, cần ngữ cảnh trình duyệt sạch; hồ sơ mặc định có thể tự khôi phục phiên đã lưu và gọi Portal.
6. Với thử tải trên Cloudflare, chỉ dùng Worker thử nghiệm riêng với Portal giả lập. Xác định tên duy nhất, theo dõi số người, request, trạng thái dự kiến, lỗi và độ trễ; xóa đúng Worker thử nghiệm khi xong. Phân biệt rõ kết quả hạ tầng Cloudflare với mô phỏng trên máy cục bộ.
7. Báo số test đạt/thất bại theo output thực tế, điều kiện thử, các giới hạn, các lỗi tìm thấy và trạng thái triển khai. Nhấn mạnh người dùng giữ tab và trình duyệt hoạt động; kết quả giả lập không bảo đảm Portal trường chịu tải hoặc request thành công.

Không tự triển khai hoặc thay đổi dữ liệu production khi người dùng chỉ yêu cầu kiểm tra. Không in token, mật khẩu hay các bí mật Cloudflare.
