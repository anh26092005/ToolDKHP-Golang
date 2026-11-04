# 🚀 HƯỚNG DẪN VẬN HÀNH & TRIỂN KHAI BACKEND GOLANG (TOOL ĐKHP UTH)

---

## 1. Tổng Quan Kiến Trúc Mới

Backend được viết lại toàn bộ bằng **Golang (Go 1.21+)** nhằm thay thế triệt để backend Node.js (Express), giải quyết tận gốc hiện tượng nghẽn đơn luồng (**Single-Thread Event Loop Bottleneck**) khi chạy cùng lúc nhiều tài khoản vào thời khắc mở cổng ĐKHP.

### 📊 Bảng So Sánh Hiệu Năng: Golang vs Node.js

| Tiêu Chí | Bản Cũ (Node.js) | Bản Mới (Golang) | Lợi Ích Thực Tế |
| :--- | :--- | :--- | :--- |
| **Xử lý đồng thời** | `child_process.fork()` nhiều processes | **1 process, 1 cổng, nhiều goroutine xử lý request** | Phục vụ nhiều phiên trên cùng một HTTP server |
| **Cổng lắng nghe** | Nhiều cổng riêng | **Một cổng: `PORT`, mặc định `3000`** | Thuận tiện chạy local, Docker và VPS |
| **Kích thước file .exe** | ~42.4 MB (đóng gói Node runtime) | **~9.1 MB** (file nhị phân thuần) | Tải & giải nén tức thì, không cần cài runtime |
| **Kết nối mạng** | `axios` / `http.Agent` phân tán | **HTTP Connection Pooling chuẩn Go** (`net/http`) | Tái sử dụng TCP connection, loại bỏ handshake SSL lặp lại |
| **Hẹn giờ mili-giây** | `setTimeout` bị trôi do event loop | **`time.After` & `context.Context`** | Độ chính xác chuẩn mili-giây từng khung thời gian |
| **Giao diện Frontend** | Vanilla HTML/CSS/JS (`/public`) | **Giữ nguyên 100% frontend** | Người dùng thao tác quen thuộc, không thay đổi |

---

## 2. Cấu Trúc Mã Nguồn Golang (`go-backend/`)

```
go-backend/
├── go.mod                     # Khai báo module và dependencies
├── go.sum                     # Checksum bảo mật package
├── main.go                    # Entrypoint: một cổng (PORT, mặc định 3000), Graceful shutdown
├── config/
│   └── config.go              # Hằng số Portal URL, Origin, Timeouts, Admin Password
├── models/
│   └── models.go              # Structs: Session, Logs, Key, Gateway, Worker
├── services/
│   ├── httpclient.go          # HTTP Client tối ưu Connection Pool + Hỗ trợ SOCKS5/HTTP Proxy
│   ├── timesync.go            # Đồng bộ chênh lệch thời gian máy chủ Portal qua Date header
│   ├── worker.go              # AutoDKHP Worker: Goroutines, Loop, Auto-Relogin, Deduplication
│   └── event_worker.go        # Auto Event Worker: Đăng ký sự kiện trường tự động
└── handlers/
    ├── auth.go                # API Đăng nhập, Profile, Validate Token, Logout
    ├── proxy.go               # Generic Proxy & Obfuscated Gateway (Base64)
    ├── registration.go        # ĐKHP thủ công, Auto Engine, Realtime HTML /view-logs
    ├── events.go              # Auto Sự kiện, Realtime HTML /api/events/view-logs
```

---

## 3. Các Cách Khởi Chạy Nhanh

### Cách 1: Chạy file .exe trực tiếp (Khuyên dùng cho người dùng cuối)
- Click đúp vào file **`run-go.bat`** tại thư mục gốc dự án.
- Hoặc chạy trực tiếp file **`dkhp-tool-golang.exe`**.
- Trình duyệt sẽ tự động mở `http://localhost:3000`.
- Server chỉ lắng nghe **một cổng**, mặc định `3000`. Biến môi trường `PORT` cho phép chọn cổng khác.
- Để dùng nhiều tài khoản, mở các hồ sơ trình duyệt độc lập; các tab cùng hồ sơ dùng chung thông tin đăng nhập trong `localStorage`.

### Cách 2: Chạy trực tiếp từ mã nguồn Go (Dành cho Developer)
```powershell
# Chạy một cổng mặc định 3000
cd go-backend
go run main.go

# Đổi sang một cổng khác (ví dụ 8080)
$env:PORT="8080"
go run main.go
```

### Cách 3: Biên dịch lại file .exe khi có thay đổi mã nguồn
- Chỉ cần click đúp vào file **`build-go.bat`** tại thư mục gốc.
- File `dkhp-tool-golang.exe` mới nhất sẽ được tạo ra tại thư mục gốc dự án.

---

## 4. Triển Khai Docker / VPS

Dự án đã có sẵn file `Dockerfile` sử dụng Multi-Stage Build:

```bash
# Build Docker image siêu nhẹ (~20MB)
docker build -t dkhp-golang .

# Chạy container trên VPS (port 3000)
docker run -d --name dkhp-app -p 3000:3000 -v $(pwd)/data:/app/data dkhp-golang
```

---

## 5. Các Tính Năng Đã Được Xác Minh Hoạt Động Hoàn Hảo

1. ✅ **Giao diện & Tài nguyên tĩnh:** Tự động load đầy đủ CSS, JS, hình ảnh, icon từ thư mục `/public`.
2. ✅ **Obfuscated Gateway (`/api/gateway`):** Giải mã và mã hóa Base64 tương thích chuẩn UTF-8 Tiếng Việt, bảo mật luồng mạng với Portal trường.
3. ✅ **Cơ chế Auto-Relogin:** Khi token hết hạn (401), hệ thống tự động tái xác thực với Portal và cập nhật token mới tức thì.
5. ✅ **Trang Realtime Log HTML (`/api/registration/view-logs` & `/api/events/view-logs`):** Xem tiến trình quét lớp, huỷ nhóm môn trùng, dừng phiên mọi lúc.

---

## 6. Timeout lúc mở cổng & Cơ chế tự phục hồi

| Thông báo | Ý nghĩa |
| --- | --- |
| `http2: timeout awaiting response headers` | Đã gửi yêu cầu qua HTTP/2 nhưng Portal chưa trả header trước hạn chờ. |
| `dial tcp ...:443: i/o timeout` | Không mở được kết nối TCP tới Portal kịp thời; có thể do quá tải, mạng hoặc tuyến kết nối. |
| `Backoff lỗi Portal/phiên/kết nối` | Tiến trình đang chủ động nghỉ trước khi thử lại, không phải trình duyệt bị treo. |

### Hành vi sau bản sửa

- Mỗi request Go được giới hạn **15 giây**, trình duyệt chờ tối đa **20 giây/yêu cầu** để nhận được lỗi rõ ràng từ Go.
- Timeout, lỗi mạng, HTTP 408/429/5xx và phản hồi HTML lỗi được thử lại với backoff tăng dần, cộng độ lệch ngẫu nhiên để các tài khoản không cùng thử lại một lúc. Có tôn trọng `Retry-After` khi Portal trả về.
- Lỗi tạm thời **không trừ số lượt thử nghiệp vụ**. Tiến trình giữ nguyên ID sự kiện, ngày, đối tượng hoặc lớp đang chờ.
- Khi Portal báo hết phiên, hệ thống đăng nhập lại bằng thông tin đã ghi nhớ. Nếu API đăng nhập cũng đang quá tải, tiếp tục chờ và thử đăng nhập lại; khi lấy được token mới thì tiếp tục đăng ký.
- Thiếu thông tin đăng nhập hoặc Portal từ chối tài khoản/mật khẩu: dừng với trạng thái yêu cầu đăng nhập lại, thay vì lặp vô hạn với mật khẩu sai.
- Go dùng mã băm toàn bộ token làm mã phiên và vẫn nhận ra token khởi tạo sau khi refresh. Các JWT có cùng phần đầu không bị gom nhầm thành một phiên.
- Nút Dừng của worker Go hủy cả request đang chờ và thời gian backoff. Bản chạy trong trình duyệt hủy lịch chờ và không gửi lượt tiếp theo; request đã gửi chờ kết thúc theo timeout.
- Mỗi khung log giữ tối đa 1.000 dòng gần nhất để tránh phình bộ nhớ khi Portal lỗi kéo dài.

### Áp dụng trên máy

1. Chạy **`build-go.bat`** để tạo lại `dkhp-tool-golang.exe` khi cập nhật mã nguồn Go.
2. Khởi động lại bằng **`run-go.bat`** và tải lại trang bằng **Ctrl+F5**.
3. Đăng nhập với tùy chọn **Ghi nhớ & Tự động đăng nhập lại khi mất phiên**.
4. Vào tab **Sự kiện**, nhập ID, bấm **Bắt đầu tự động** và giữ tab/trình duyệt hoạt động, máy không ngủ. Giao diện hiện tại chạy vòng lặp trong tab, kể cả khi phục vụ bằng Go; API Go còn hỗ trợ worker chạy nền riêng.

Khi gọi trực tiếp `POST /api/events/auto/start` cho worker Go, payload hỗ trợ `eventId`, `dateId`, `objectId`, `retries`, `delayMin`, `delayMax`, `targetTime` và `credentials: { username, password }`. Cần truyền `credentials` nếu muốn worker tự đăng nhập lại; token đơn lẻ không đủ để lấy phiên mới.

### Tùy chọn chẩn đoán HTTP/2

Nếu một tuyến mạng/proxy có lỗi tương thích HTTP/2, có thể khởi động Go với HTTP/1.1:

```powershell
$env:PORTAL_DISABLE_HTTP2 = "true"
.\dkhp-tool-golang.exe
```

Biến này được đọc khi khởi động HTTP client. Timeout TCP vẫn cần chờ mạng/Portal hồi phục. Khả năng xử lý của Portal trường quyết định request có thành công hay không; tăng goroutine hoặc tăng timeout không tự làm Portal hết quá tải.

### Kiểm tra cục bộ

```powershell
node --test tests/*.test.*
cd go-backend
go test -timeout 30s ./...
go vet ./...
```

Các test dùng Portal giả lập và đồng hồ giả lập, có tình huống gián đoạn 30 phút, 60 phiên độc lập, refresh token, timeout và dừng kết nối HTTP/2 đang chờ. Đây không phải thử tải Portal thật hoặc chạy chờ 30 phút ngoài thực tế.
