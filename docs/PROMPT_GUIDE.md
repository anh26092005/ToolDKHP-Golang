# 📖 CẨM NANG HƯỚNG DẪN SỬ DỤNG TOOL ĐKHP UTH
### Dành cho Sinh viên Trường Đại học Giao thông Vận tải TP.HCM (UTH)

> **Mục tiêu:** Hướng dẫn toàn diện cách sử dụng công cụ Đăng ký học phần (ĐKHP) tự động để tra cứu môn học, xếp thời khóa biểu trực quan không lo trùng lịch, hẹn giờ săn học phần siêu tốc và quản lý danh sách môn học, học phí, tín chỉ một cách khoa học.

---

## 📑 Mục Lục
1. [Giới thiệu & Chuẩn bị](#1-giới-thiệu--chuẩn-bị)
2. [Đăng nhập & Thiết lập ban đầu](#2-đăng-nhập--thiết-lập-ban-đầu)
3. [Quy trình sử dụng chuẩn (Workflow 4 bước)](#3-quy-trình-sử-dụng-chuẩn-workflow-4-bước)
4. [Hướng dẫn chi tiết từng chức năng](#4-hướng-dẫn-chi-tiết-từng-chức-năng)
   - [Tab 1: 🗓️ Lịch dự kiến (Xếp TKB thông minh)](#tab-1-️-lịch-dự-kiến-xếp-thời-khóa-biểu-trực-quan)
   - [Tab 2: 📝 Đăng ký HP (Săn môn tự động & Hẹn giờ)](#tab-2--đăng-ký-học-phần-tự-động)
   - [Tab 3: ✅ Môn đã ĐK (Quản lý, Tín chỉ, Học phí & Xuất file)](#tab-3--môn-đã-đăng-ký-quản-lý-học-phần-tín-chỉ--học-phí)
   - [Tab 4: 📚 Check môn học & Lọc lịch trùng](#tab-4--check-môn-học--lọc-lịch-trùng)
   - [Tab 5: 📅 Lịch học chi tiết](#tab-5--lịch-học-chi-tiết)
   - [Tab 6: 🔄 Học phần cải thiện](#tab-6--học-phần-cải-thiện)
   - [Tab 7: 🎉 Đăng ký sự kiện](#tab-7--đăng-ký-sự-kiện)
5. [Kinh nghiệm & Mẹo săn môn thành công 100%](#5-kinh-nghiệm--mẹo-săn-môn-thành-công)
6. [Bảng tra cứu lỗi thường gặp & Cách xử lý](#6-bảng-tra-cứu-lỗi-thường-gặp--cách-xử-lý)
7. [Dành cho Kỹ thuật viên (Kiến trúc & Tối ưu hóa)](#7-phụ-lục-kỹ-thuật-dành-cho-admin--developer)

---

## 1. Giới thiệu & Chuẩn bị

Tool ĐKHP UTH là ứng dụng web hiện đại được phát triển nhằm giải quyết triệt để vấn đề nghẽn mạng, sập server cổng portal của trường vào các khung giờ cao điểm mở đăng ký học phần.

### Các ưu điểm nổi bật:
- ⚡ **Tốc độ cực cao:** Gửi request đăng ký trực tiếp đến API với độ trễ (delay) tính bằng mili-giây (3ms - 10ms).
- ⏰ **Hẹn giờ mở cổng (Auto Sniper):** Tự động đồng bộ và bắn request ngay tại thời khắc cổng mở (chính xác đến từng mili-giây).
- 🗓️ **Thời khóa biểu trực quan:** Hỗ trợ xem lịch cả **Môn học chính** và **Môn học điều kiện** (Tiếng Anh, kỹ năng bổ trợ,...), tự động phát hiện và cảnh báo màu đỏ khi bị trùng lịch học giữa các môn.
- 💰 **Quản lý học phần hiện đại:** Hiển thị rõ ràng số tín chỉ tích lũy (ví dụ: `06 / 24 TC`), học phí tạm tính, phòng học cụ thể và hỗ trợ in ấn hoặc xuất danh sách ra file Excel (`.xlsx`).

---

## 2. Đăng nhập & Thiết lập ban đầu

Truy cập hệ thống qua trình duyệt web tại địa chỉ: `http://localhost:3000` (hoặc tên miền được quản trị viên cung cấp).

![Giao diện đăng nhập](file:///c:/Users/asus/Documents/Dev/web-dkhp/public/index.html)

### Các trường thông tin cần nhập:
2. **Mã sinh viên (MSSV):** Nhập mã số sinh viên của bạn (ví dụ: `2251120001`).
3. **Mật khẩu:** Mật khẩu đăng nhập vào cổng thông tin sinh viên UTH (`portal.ut.edu.vn`).

> 💡 **Lưu ý về ID Đợt (Header):**
> Ở góc trên thanh tiêu đề có ô **ID Đợt** (mặc định hiện tại là `77`). Đây là mã đợt đăng ký do trường quy định. Thông thường bạn để nguyên giá trị mặc định. Nếu trường thông báo đăng ký đợt đặc biệt (hè, học kỳ phụ, cải thiện bổ sung), bạn chỉ cần đổi số ID Đợt tại đây.

---

## 3. Quy trình sử dụng chuẩn (Workflow 4 bước)

Để đạt tỷ lệ đăng ký thành công cao nhất mà không bị trùng lịch, bạn nên làm theo quy trình chuẩn sau:

```
[BƯỚC 1: LÊN LỊCH DỰ KIẾN]
  │── Vào Tab "Lịch dự kiến"
  │── Chọn Môn chính / Môn điều kiện
  │── Chọn các lớp phù hợp ──→ Kiểm tra không bị trùng lịch (không có màu đỏ)
  ▼
[BƯỚC 2: CHUYỂN ID SANG TAB ĐĂNG KÝ]
  │── Tích chọn các lớp muốn học
  │── Bấm nút "⚡ Điền ID vào tab Đăng ký"
  ▼
[BƯỚC 3: THIẾT LẬP AUTO & HẸN GIỜ]
  │── Sang Tab "Đăng ký HP"
  │── Cài hẹn giờ mở cổng (VD: 07:59:58 - trước 2 giây)
  │── Đặt Retry: 10, Delay: 3 - 5ms
  │── Bấm "▶ Bắt đầu"
  ▼
[BƯỚC 4: KIỂM TRA KẾT QUẢ & XUẤT FILE]
  │── Vào Tab "Môn đã ĐK" bấm "🔄 Tải lại"
  │── Xem tổng số môn, số tín chỉ, học phí tạm tính, phòng học
  └── Bấm "📊 Xuất Excel" hoặc "🖨️ In kết quả"
```

---

## 4. Hướng dẫn chi tiết từng chức năng

---

### Tab 1: 🗓️ Lịch dự kiến (Xếp thời khóa biểu trực quan)

Đây là nơi bạn xếp thời khóa biểu và lên kế hoạch học tập trước giờ mở cổng.

#### Các tính năng chính:
1. **Chuyển đổi loại môn:**
   - **📘 Môn học chính:** Dành cho các học phần chuyên ngành, đại cương trong chương trình đào tạo.
   - **📗 Môn học điều kiện:** Dành cho các học phần ngoại ngữ (English B1, B2), GDTC, kỹ năng tăng cường (sử dụng API chuyên biệt của trường).
2. **Nhập mã môn hoặc chọn nhanh:**
   - Bạn có thể dán danh sách **Mã học phần** (ví dụ: `00611828`, `010112303903`) hoặc danh sách **ID lớp** (mỗi dòng một mã).
   - Hoặc bấm danh sách **"Chọn nhanh môn học"** để chọn ngay môn bạn cần mà không cần nhớ mã.
3. **Tra cứu & Hiển thị:**
   - Bấm **"🔍 Xem lịch"**: Tool sẽ gửi yêu cầu lấy danh sách toàn bộ lớp học phần đang mở kèm thời khóa biểu, phòng học và giảng viên.
4. **Thời khóa biểu trực quan (Visual Timetable):**
   - Tự động phân loại lịch học theo **Đợt 1** và **Đợt 2**.
   - Bấm nút **"Đợt 1"** hoặc **"Đợt 2"** ở góc trên bảng để xem thời khóa biểu của đợt tương ứng.
   - **Cảnh báo trùng lịch:** Nếu hai lớp bạn chọn có tiết học trùng nhau trong cùng một thứ và đợt, ô lịch sẽ chuyển sang **màu đỏ cảnh báo** (`⚠️ Trùng lịch`).
5. **Gửi sang tab Đăng ký:**
   - Tích chọn các lớp bạn ưng ý nhất trong danh sách bên dưới, sau đó bấm nút **"⚡ Điền ID vào tab Đăng ký"**. Tool sẽ tự động trích xuất các ID lớp và điền sang Tab Đăng ký HP cho bạn.

---

### Tab 2: 📝 Đăng ký học phần tự động

Công cụ tự động gửi request tốc độ cao giúp bạn giành suất học phần nhanh chóng.

#### Hướng dẫn thiết lập:
1. **ID lớp học phần:** Ô văn bản chứa các ID lớp bạn muốn đăng ký (mỗi ID một dòng, ví dụ: `31204`, `31205`).
2. **Loại môn:** Chọn đúng loại:
   - *Môn chính*
   - *Môn học phụ*
   - *Môn cải thiện*
3. **Hẹn giờ chạy (Auto Sniper):**
   - Định dạng: `HH:MM:SS` (Ví dụ: `08:00:00`).
   - *Mẹo vàng:* Đồng hồ máy tính của bạn có thể lệch từ 1-2 giây so với server trường. Vì vậy, nếu cổng mở lúc `08:00:00`, bạn nên hẹn lúc `07:59:58` để công cụ bắt đầu chu kỳ gửi request thăm dò, đảm bảo đăng ký thành công ngay mili-giây đầu tiên.
4. **Số lượt thử (Loop / Retry):** Khuyên dùng từ `10` đến `20` lần.
5. **Thời gian chờ (Delay Min - Max):**
   - Khuyên dùng: `Min: 3ms`, `Max: 5ms` (hoặc `5ms - 10ms`).
   - Khoảng thời gian siêu ngắn này giúp bạn vượt trước các sinh viên đăng ký bằng tay mà không làm tràn bộ đệm mạng.
6. **Bắt đầu & Dừng:**
   - Bấm **"▶ Bắt đầu"**: Thanh trạng thái và khung **Log đăng ký** sẽ nhảy thông tin chi tiết từng lần thử và kết quả trả về từ trường.
   - Bấm **"⏹️ Dừng"** nếu muốn hủy tiến trình bất cứ lúc nào.

---

### Tab 3: ✅ Môn đã đăng ký (Quản lý học phần, Tín chỉ & Học phí)

Giao diện quản lý toàn diện các môn bạn đã ghi danh thành công trong học kỳ.

#### 1. Thẻ tóm tắt trên đầu:
- **💾 TỔNG SỐ MÔN:** Tổng số lượng học phần chính và điều kiện đã ghi danh thành công.

#### 2. Phân tách danh sách môn học rõ ràng:
- **🔵 Môn học chính đã đăng ký:**
  Bao gồm các cột: `STT | Mã HP | Tên học phần & Ghi chú | Tên lớp | Lịch học & Phòng | Số TC | Hành động`.
  - Có icon lịch 📅 để xem nhanh chi tiết từng buổi học.
  - Hiển thị rõ ràng tên lớp, thứ, tiết học và phòng học cụ thể.
  - Hiển thị số tín chỉ (Số TC) của từng môn.
- **🟠 Môn học phụ / điều kiện đã đăng ký:**
  Bao gồm các lớp kỹ năng tăng cường, học phần điều kiện Tiếng Anh, GDTC với đầy đủ thông tin lớp, phòng và số TC.

#### 3. Các nút tiện ích:
- **🔄 Tải lại:** Làm mới ngay lập tức danh sách môn đã đăng ký từ cổng trường.
- **🗑️ Hủy môn:** Nếu trường còn mở cổng cho phép hủy môn, nút đỏ `🗑️ Hủy môn` sẽ sáng để bạn hủy lớp. Nếu nhà trường đã khóa tính năng hủy, nút sẽ tự động chuyển thành nhãn `🔒 Không thể hủy`.

---

### Tab 4: 📚 Check môn học & Lọc lịch trùng

Dùng để tra cứu danh mục toàn bộ các môn học và các lớp học phần được trường mở trong kỳ.

- Bấm **"🔍 Kiểm tra"** để tải danh mục môn học.
- Bảng bên trái hiển thị danh sách môn học. Khi bạn bấm vào bất kỳ môn nào, bảng bên phải sẽ hiển thị toàn bộ các lớp mở của môn đó kèm:
  + Mã lớp học phần & Tên lớp
  + Giảng viên phụ trách
  + Thứ, tiết học, phòng học
  + Sĩ số hiện tại / Sĩ số tối đa (ví dụ: `65/70` - còn 5 chỗ).
- **Bộ lọc thông minh "Lọc lịch trùng":** Tích vào tùy chọn này, hệ thống sẽ tự động đối chiếu và ẩn đi tất cả các lớp có thời gian học trùng với các môn bạn đã đăng ký thành công, giúp bạn chọn lớp an toàn 100%.

---

### Tab 5: 📅 Lịch học chi tiết

- Hiển thị thời khóa biểu chi tiết dạng lịch tuần của tất cả các môn bạn đã ghi danh.
- Mỗi tiết học sẽ hiển thị tên môn, phòng học, giảng viên và ghi chú đợt học.
- Nhấn **"✨ Làm mới lịch"** để đồng bộ lại dữ liệu.

---

### Tab 6: 🔄 Học phần cải thiện

- Dành riêng cho sinh viên có nhu cầu học lại hoặc học nâng điểm cải thiện.
- **🔎 Tìm Mã Học Phần từ ID / Mã lớp HP:** Nếu bạn chỉ có ID lớp (VD: `219184`) hoặc Mã lớp HP (VD: `010441203101`), chỉ cần dán vào ô tìm kiếm trên đầu tab, hệ thống sẽ tự động tra cứu ra chính xác Mã HP môn đó, sao chép nhanh và tự động cuộn đến môn học đó trong danh sách cải thiện của bạn!
- Bấm **"🔍 Kiểm tra"**: Tool sẽ tra cứu các môn bạn có điểm thấp đủ điều kiện học cải thiện, kèm điểm số cũ và danh sách các lớp mở trong kỳ để bạn chọn đăng ký.

---

### Tab 7: 🎉 Đăng ký sự kiện

- Săn vé tham gia các hội thảo (Seminar), sự kiện văn nghệ, hoạt động ngày hội việc làm do UTH tổ chức trên cổng `portal.ut.edu.vn`.
- Nhập **ID Sự kiện** và **ID Ngày** (lấy từ đường link trên portal, ví dụ `portal.ut.edu.vn/event/42`).
- Có chế độ đăng ký tự động và hẹn giờ mở cổng tương tự như đăng ký học phần.

---

## 5. Kinh nghiệm & Mẹo săn môn thành công

1. **Chuẩn bị trước giờ G ít nhất 20 phút:**
   - Đăng nhập vào tool sớm, kiểm tra kết nối mạng ổn định.
   - Sử dụng tab **Lịch dự kiến** để chuẩn bị sẵn:
     + 1 Phương án chính (các lớp mong muốn nhất).
     + 1-2 Phương án dự phòng (lớp khác của cùng môn nếu lớp chính hết chỗ).
2. **Sao chép ID lớp sẵn sàng:**
   - Dán danh sách ID lớp phương án chính vào ô đăng ký.
   - Lưu các ID lớp dự phòng vào ứng dụng ghi chú (Notepad) để paste vào ngay khi cần.
3. **Cài đặt hẹn giờ thông minh:**
   - Nếu cổng mở lúc `08:00:00`, hãy đặt hẹn giờ ở mức `07:59:58`. Tool sẽ bắt đầu gửi các request thăm dò ở những giây cuối cùng. Ngay khi hệ thống trường mở cổng, request của bạn sẽ lọt vào danh sách xử lý đầu tiên.
4. **Không nên lạm dụng delay quá nhỏ khi mạng lag:**
   - Mức delay lý tưởng là **3ms - 5ms**. Nếu mạng internet nhà bạn không ổn định hoặc cổng trường có dấu hiệu chậm phản hồi, có thể tăng delay lên **15ms - 30ms** để tránh rớt gói tin.

---

## 6. Bảng tra cứu lỗi thường gặp & Cách xử lý

| Triệu chứng / Thông báo | Nguyên nhân | Hướng dẫn khắc phục |
|:---|:---|:---|
| **"Đăng nhập thất bại / Sai tài khoản hoặc mật khẩu"** | Thông tin đăng nhập portal UTH không đúng | Kiểm tra xem bàn phím có bật CapsLock hoặc gõ dấu tiếng Việt trong mật khẩu không. Thử đăng nhập lại trên trang `portal.ut.edu.vn` để xác thực. |
| **"Phiên đăng nhập hết hạn (Session Expired)"** | Token đăng nhập của trường hết thời hạn (thường sau vài tiếng) | Bấm nút **"Đăng xuất"** ở góc trên bên phải, sau đó tiến hành đăng nhập lại để nhận token mới. |
| **"Lớp học phần đã hết số lượng cho phép"** | Lớp đã đủ sĩ số tối đa | Ngay lập tức dùng danh sách ID lớp dự phòng đã chuẩn bị ở Tab Lịch dự kiến và bấm đăng ký lại. |
| **"HTTP 429 Too Many Requests"** | Gửi request quá dày đặc khiến server trường tạm khóa kết nối ngắn hạn | Tool đã có cơ chế tự động chờ `Retry-After`. Bạn chỉ cần tăng thông số Delay lên (ví dụ: `20ms - 50ms`) rồi bấm lại. |
| **"Không thể hủy môn / Nút Hủy bị mờ"** | Trường đã hết thời gian cho phép rút học phần | Theo quy định của UTH, cổng chỉ cho phép sinh viên hủy môn trong khung giờ quy định. Sau thời hạn này, hệ thống sẽ hiển thị nhãn `🔒 Không thể hủy`. |
| **"Môn học không tồn tại trong đợt"** | Sai mã đợt đăng ký | Kiểm tra lại ô **ID Đợt** ở thanh header (mặc định là `77`). Đối chiếu với thông báo đăng ký học phần trên phòng đào tạo xem học kỳ này dùng mã đợt bao nhiêu. |

---

## 7. Phụ lục kỹ thuật dành cho Admin / Developer

Nếu bạn là quản trị viên triển khai công cụ cho nhóm sinh viên (từ 20 - 50 người dùng cùng lúc):

1. **Cơ chế Proxy Requests:**
   - Tool hoạt động như một Reverse Proxy trung gian, nhận lệnh từ giao diện trình duyệt của người dùng và gửi request mang kèm Cookie/Token xác thực đến hệ thống API của UTH.
2. **Xử lý IP Rate Limit:**
   - Để tránh việc hàng chục người dùng cùng gửi request qua 1 địa chỉ IP của server làm kích hoạt tường lửa WAF của trường, khuyến nghị triển khai cụm proxy xoay vòng (Rotating Residential Proxies) hoặc Cloudflare Workers phân tán.
3. **Thuật toán tính học phí:**
   - Học phí UTH tính theo công thức: `Số tín chỉ × Đơn giá tín chỉ hiện hành` (~`408.333 VNĐ/TC`, môn 3 TC tương đương `1.225.000 VNĐ`).
   - Các học phần kỹ năng tăng cường hoặc học phần bổ trợ có số TC bằng `0` được hệ thống gán nhãn `Miễn phí`.
4. **Bảo mật thông tin sinh viên:**
   - Mọi thông tin tài khoản và mật khẩu chỉ được lưu trữ trong phiên làm việc cục bộ của trình duyệt (`localStorage` / `sessionStorage`) và truyền an toàn qua API proxy, hoàn toàn không lưu trữ trái phép trên bất kỳ cơ sở dữ liệu bên ngoài nào.

---
*Chúc các bạn sinh viên UTH có một kỳ đăng ký học phần thành công, chọn được lịch học ưng ý nhất!*
