# BÁO CÁO TÓM TẮT CUỘC TRÒ CHUYỆN (COMPACT SUMMARY)
## Chuyển Đổi Backend Web Tool ĐKHP Từ Node.js (Express) Sang Golang

---

### 1. Bối Cảnh & Mục Tiêu Ban Đầu (User Request)

- **Vấn đề của hệ thống cũ (Node.js/Express):** 
  - Gặp hiện tượng nghẽn do **Single-Thread Event Loop** khi chạy đồng thời nhiều tài khoản vào cùng một mili-giây mở cổng đăng ký học phần (ĐKHP) của trường UT.
  - Khi người dùng mở nhiều tab hoặc chạy nhiều luồng auto cùng lúc, I/O và JSON parse/serialize trên Node.js làm tăng latency, dễ lỡ thời khắc vàng mở cổng.
- **Mục tiêu chuyển đổi:**
  - Viết lại toàn bộ phần backend sang **Golang (Go 1.21+)** đạt hiệu năng cao (High-Concurrency Networking).
  - Giữ nguyên 100% giao diện frontend thuần trong thư mục `/public` (HTML, CSS, JS), không thay đổi giao diện hay trải nghiệm người dùng.
  - Tối ưu hóa kết nối mạng với **Connection Pooling** (`net/http.Client`).
  - Xử lý đa luồng với **Goroutines**, `sync.WaitGroup`, `sync.Map`, và `Channels`.
  - Hỗ trợ chế độ **Multi-Port (3000 - 3009)** chạy 10 luồng server song song trong cùng 1 process để phục vụ 10 tài khoản độc lập trên cùng máy offline, đồng thời hỗ trợ chế độ Single-Port khi chạy VPS/Docker qua biến môi trường `PORT`.
  - Đảm bảo đầy đủ các tính năng: Proxy API tới Portal trường (`https://portal.ut.edu.vn/api/v1`), Gateway mã hóa Base64, AutoDKHP Worker hẹn giờ mili-giây có bù lệch giờ (`Date` header), cơ chế tự động hủy nhóm lớp trùng, tự động đăng nhập lại khi token hết hạn (auto re-login), hỗ trợ Proxy riêng (HTTP/SOCKS5) cho từng tài khoản, hệ thống quản lý Access Key (`keys.json`) và Auto Đăng ký sự kiện (`events`).

---

### 2. Quá Trình Phân Tích Codebase Node.js Cũ

Trợ lý AI đã tiến hành duyệt và đọc toàn bộ cấu trúc mã nguồn hiện tại của dự án:
1. **`server.js`:**
   - Cơ chế Multi-port cũ dùng `child_process.fork()` để sinh 10 process con chạy từ port 3000 -> 3009 khi chạy ở dạng `.exe`.
   - Cấu hình session, CORS, routing và serve static `/public`.
2. **`routes/auth.js`:**
   - API đăng nhập trực tiếp qua MSSV/mật khẩu tới Portal trường (`/user/login`).
   - Lấy thông tin sinh viên, kiểm tra tính hợp lệ của token và đăng xuất.
3. **`routes/proxy.js`:**
   - Chuyển tiếp các request `/api/dkhp/*`, `/api/dkhpdk/*`, `/api/eventWeb/*` tới Portal trường.
   - Endpoint đặc biệt `/api/gateway` nhận payload mã hóa Base64 (`{ p: "..." }`) và phản hồi dữ liệu mã hóa Base64 (`{ r: "..." }`) nhằm bảo mật luồng mạng.
4. **`routes/registration.js`:**
   - Đăng ký học phần thủ công: chính khóa (`/main`), điều kiện (`/extra`), cải thiện (`/improvement`), hủy môn (`/cancel`).
   - **AutoDKHP Worker Engine:** Quản lý session bằng `Map`, loop đăng ký, tính năng hẹn giờ chính xác bằng cách đồng bộ giờ qua header `Date` của Portal trường, tự động loại trừ các lớp cùng môn khi đã đăng ký thành công một lớp, tự đăng nhập lại khi gặp 401, và trang web HTML realtime `/view-logs`.
5. **`routes/events.js`:**
   - Worker tự động đăng ký sự kiện trường, cơ chế hẹn giờ, retry và view logs HTML.
6. **`routes/keys.js`:**
   - Quản lý bản quyền phần mềm qua Access Key: lưu trữ file `data/keys.json`, phân loại `shared` (dùng chung) và `regular` (khóa gắn với `deviceId`), giao diện admin và thống kê.
7. **Frontend `/public/js/api.js`:**
   - Đảm bảo tất cả các endpoint Go Backend phục vụ khớp 100% với client requests.

---

### 3. Kiến Trúc Backend Golang Đã Xây Dựng (`go-backend/`)

Toàn bộ Backend Golang đã được triển khai hoàn chỉnh trong thư mục [go-backend](file:///c:/Users/asus/Documents/Dev/web-dkhp-golang/go-backend) với cấu trúc phân tầng chuyên nghiệp:

```
go-backend/
├── go.mod                     # Module Go và khai báo dependencies (Gin v1.10.0, net/proxy, ...)
├── go.sum                     # Checksum bảo mật của các packages Go
├── main.go                    # Entrypoint: cấu hình Gin, Multi-Port (3000-3009) & Single-Port, Graceful Shutdown
├── config/
│   └── config.go              # Cấu hình hằng số: PortalBaseURL, Origin, Timeouts, Admin Password
├── models/
│   └── models.go              # Định nghĩa Structs: Auth, Gateway, AutoDKHP, Events, Keys, Logs
├── services/
│   ├── httpclient.go          # HTTP Client tùy biến với Connection Pooling, hỗ trợ HTTP & SOCKS5 Proxy
│   ├── timesync.go            # Đồng bộ sai lệch thời gian mili-giây với header Date từ UT Portal
│   ├── worker.go              # AutoDKHP Worker: Goroutines, Loop, Auto-Relogin, Deduplication
│   └── event_worker.go        # Auto Event Worker: Đăng ký sự kiện trường ngầm bằng Goroutines
└── handlers/
    ├── auth.go                # Handler: Đăng nhập, Profile, Session Token, Logout
    ├── proxy.go               # Handler: Generic Proxy & Base64 Obfuscated Gateway
    ├── registration.go        # Handler: ĐKHP thủ công, Auto Start/Stop/Status, View Logs HTML
    ├── events.go              # Handler: Auto Sự kiện Start/Stop/Status, View Logs HTML
    └── keys.go                # Handler: Quản lý Key bản quyền, Device Binding, Admin CRUD, Thống kê
```

#### Chi Tiết Các Thành Phần Cốt Lõi:
1. **HTTP Client & Connection Pooling ([services/httpclient.go](file:///c:/Users/asus/Documents/Dev/web-dkhp-golang/go-backend/services/httpclient.go)):**
   - Thiết lập `MaxIdleConns: 200`, `MaxIdleConnsPerHost: 50`, `MaxConnsPerHost: 100`, `IdleConnTimeout: 90s`, `Timeout: 15s`.
   - Hàm `NewHTTPClientWithProxy(proxyURL)` hỗ trợ gắn Proxy HTTP hoặc SOCKS5 riêng biệt cho từng tài khoản/phiên đăng ký.
2. **Time Synchronization ([services/timesync.go](file:///c:/Users/asus/Documents/Dev/web-dkhp-golang/go-backend/services/timesync.go)):**
   - Gửi HEAD/GET request đo Round-Trip Time (RTT) đến Portal trường và đọc header `Date`.
   - Tính toán offset = `serverTime - (localTime + RTT/2)`, tự động bù giờ chính xác từng mili-giây.
3. **AutoDKHP Goroutine Worker ([services/worker.go](file:///c:/Users/asus/Documents/Dev/web-dkhp-golang/go-backend/services/worker.go)):**
   - Chạy nền độc lập trong Goroutine, quản lý trạng thái an toàn đa luồng bằng `sync.Map` và `sync.Mutex`.
   - Tự động phát hiện token hết hạn (401), dùng credentials đã lưu để re-login lấy token mới tức thì.
   - Nhóm môn học (`courseGroups`): Khi 1 lớp trong môn đăng ký thành công, tự động loại bỏ các lớp dự phòng còn lại của môn đó khỏi danh sách quét.
4. **Multi-Port Server Trong Cùng 1 Process ([main.go](file:///c:/Users/asus/Documents/Dev/web-dkhp-golang/go-backend/main.go)):**
   - Khác với Node.js phải `fork` 10 tiến trình con nặng nề, Go khởi chạy **10 Goroutines `http.Server`** lắng nghe đồng thời từ cổng 3000 đến 3009 trên cùng 1 process nhẹ nhàng và tiết kiệm RAM vượt trội.
   - Khi có biến môi trường `PORT` (ví dụ trên Docker/VPS), tự động chuyển sang chế độ Single-Port.
   - Hỗ trợ **Graceful Shutdown** bắt tín hiệu `SIGINT`, `SIGTERM`.

---

### 4. Kết Quả Xác Thực & Biên Dịch (Compilation Verification)

- **Môi trường Go:** Đã tự động cài đặt `go1.23.3 windows/amd64`.
- **Dependencies:** Đã chạy `go mod tidy` tải đầy đủ các package chuẩn:
  - `github.com/gin-gonic/gin v1.10.0`
  - `golang.org/x/net/proxy` (hỗ trợ SOCKS5)
- **Kiểm thử biên dịch:**
  - Lệnh: `go build -v ./...`
  - Kết quả: **Exit code 0** — Toàn bộ module, packages và handlers được biên dịch hoàn toàn sạch sẽ, không có bất kỳ lỗi cú pháp hay import nào.

---

### 5. Hướng Dẫn Sử Dụng & Khởi Chạy

#### Chạy phát triển (Development):
```powershell
cd c:\Users\asus\Documents\Dev\web-dkhp-golang\go-backend
go run main.go
```
- Khi chạy không có biến môi trường `PORT`: Hệ thống sẽ tự động mở 10 cổng từ `http://localhost:3000` đến `http://localhost:3009`.
- Mở trình duyệt truy cập `http://localhost:3000` (tab 1), `http://localhost:3001` (tab 2), v.v. để thao tác đồng thời nhiều tài khoản.

#### Chạy chế độ VPS / Docker (Single Port):
```powershell
$env:PORT = "8080"
go run main.go
```

#### Biên dịch thành file thực thi `.exe` độc lập:
```powershell
cd c:\Users\asus\Documents\Dev\web-dkhp-golang\go-backend
go build -ldflags="-s -w" -o dkhp-server.exe main.go
```
File `dkhp-server.exe` sinh ra là file nhị phân độc lập chạy trực tiếp trên Windows mà không cần cài thêm Node.js hay bất kỳ môi trường runtime nào.
