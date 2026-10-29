---
name: dkhp-kiem-tra-test
description: Kiểm tra và test ứng dụng ĐKHP, khả năng tự thử lại khi Portal lỗi và 60 phiên đồng thời qua Cloudflare.
---

# Kiểm tra và test ĐKHP

1. Xác định phiên bản, thay đổi hiện tại (`git status --short --branch`) và kiến trúc: Cloudflare Worker phục vụ frontend và gateway; tiến trình đăng ký chạy trong tab người dùng.
2. Tại root chạy `node --test tests/*.test.*`. Chạy `node --check public/js/api.js` và `node --check public/js/app.js` khi sửa frontend. Khi cần kiểm tra Go, chạy `go test ./...` và `go vet ./...` trong `go-backend`; chạy `go test -race ./...` nếu CGO và trình biên dịch C có sẵn.
3. Xem `tests/browser-registration-retry.test.js`: các test mô phỏng Portal trả lỗi, đăng nhập lại, và 60 phiên độc lập gặp lỗi trong 30 phút bằng đồng hồ giả lập. Kiểm tra danh sách lớp, lượt thử, thành công và trạng thái dừng; nói rõ đây không phải chạy 30 phút thực hay thử tải Portal thật.
4. Nếu được yêu cầu kiểm tra Cloudflare đang chạy, kiểm tra trang chủ và `/api/license/config` bằng các request chỉ đọc; có thể so sánh nội dung JS trên Worker với file cục bộ. Không dùng tài khoản thật hoặc gửi đăng ký thật.
5. Với thử tải trên Cloudflare, chỉ dùng Worker thử nghiệm riêng với Portal giả lập. Xác định tên duy nhất, theo dõi số người, request, trạng thái dự kiến, lỗi và độ trễ; xóa đúng Worker thử nghiệm khi xong. Phân biệt rõ kết quả hạ tầng Cloudflare với mô phỏng trên máy cục bộ.
6. Báo số test đạt/thất bại, điều kiện thử, các giới hạn, các lỗi tìm thấy và trạng thái triển khai. Nhấn mạnh người dùng giữ tab và trình duyệt hoạt động; kết quả giả lập không bảo đảm Portal trường chịu tải hoặc request thành công.

Không tự triển khai hoặc thay đổi dữ liệu production khi người dùng chỉ yêu cầu kiểm tra. Không in token, mật khẩu hay các bí mật Cloudflare.
