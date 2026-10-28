# Key cho bản EXE và trang quản trị Cloudflare

## 1. Triển khai Cloudflare Worker

Deploy repo này bằng Cloudflare Workers, sử dụng lệnh `npx wrangler deploy` tại thư mục gốc. Worker dùng thư mục `public` và API trong `functions/`. KV namespace được gắn qua binding `DKHP_KEYS` trong `wrangler.toml`; ID phải thuộc tài khoản Cloudflare đang triển khai. Trang quản trị ở `https://<domain-worker>/admin.html` và API key ở `/api/keys/`.

Tạo secret `ADMIN_PASSWORD` trong phần biến môi trường của Cloudflare Worker cho môi trường deploy đang dùng, rồi deploy lại. Sử dụng một mật khẩu mới đủ mạnh; mật khẩu cũ có trong source không còn được dùng bởi API Cloudflare. Admin đăng nhập bằng mật khẩu secret đó. Không đưa mật khẩu hoặc thông tin tài khoản vào source.

## 2. Build EXE

Cài Go, chạy `build-go.bat` và nhập URL gốc HTTPS của Worker khi được hỏi, ví dụ `https://ten-worker.ten-tai-khoan.workers.dev` (không thêm `/admin.html` hoặc `/api`). Có thể đặt biến môi trường `KEY_SERVER_URL` trước khi chạy để bỏ qua câu hỏi. Script tạo `dkhp-tool-golang.exe` tại thư mục gốc; URL key được gắn vào file lúc build. Build lại khi URL thay đổi.

EXE cần thư mục `public` cạnh file để phục vụ giao diện. Phát hành EXE cùng thư mục này. Khởi chạy EXE và truy cập địa chỉ localhost được in ra màn hình.

## 3. Sử dụng

Tạo key trong trang admin Cloudflare. Nhập key đó vào EXE trước khi đăng nhập Portal. EXE xác thực online qua `/api/keys/validate`; nếu Cloudflare không truy cập được hoặc key không hợp lệ, việc kích hoạt thất bại. Key regular được gắn với mã thiết bị do backend Go lấy từ hệ thống, còn key shared dùng chung. Dùng chức năng reset trong admin khi cần thay thiết bị.

Phiên kích hoạt trong tiến trình EXE có thời hạn 12 giờ, được kiểm tra lại khi cần và khi mở lại trang; key được ghi nhớ trên trình duyệt. Key bị xóa/hết hạn sẽ không được chấp nhận ở lần kiểm tra kế tiếp. Những worker đã khởi chạy không tự dừng khi key bị thu hồi.

Việc kiểm tra là trực tuyến và dựa trên client EXE do người dùng nắm giữ: không thể bảo đảm chống sửa đổi binary. Cloudflare KV có tính nhất quán cuối cùng, nên cập nhật/thu hồi key hoặc gắn thiết bị có thể chưa phản ánh ngay ở mọi nơi.
