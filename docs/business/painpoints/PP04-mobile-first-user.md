# PP04 — Painpoints: Mobile-First Power User (Sam)

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | PP04 |
| **Actor** | Mobile-First Power User / CTO |
| **Đại diện** | Sam — CTO startup nhỏ, di chuyển nhiều |
| **Quote** | *"Tôi cần biết ngay khi agent xong việc, dù đang ở đâu."* |
| **Tham chiếu giải pháp** | [SOL04](../solutions/SOL04-mobile-first-user.md) |

---

## Bối cảnh

Sam là CTO của một startup nhỏ, phụ trách cả kỹ thuật lẫn business. Sam dùng AI agent để tự động hóa phần lớn development workflow nhưng thường không ở trước máy tính — đang trong cuộc họp, gặp đối tác, hoặc di chuyển. Sam cần kiểm soát và theo dõi agent mà không bị gán chặt vào màn hình.

---

## Danh sách Painpoints

### PP04-01: Phải Ngồi Chờ Agent Xong Việc

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** Mỗi task agent chạy (5-15 lần/ngày)  

**Mô tả:**
Sam khởi động agent lúc 9 giờ để làm task, nhưng task cần 45 phút để hoàn thành. Sam phải ở lại trước màn hình để không bỏ lỡ khi agent xong, vì không có thông báo nào. Thời gian chờ này không thể tận dụng cho việc khác.

**Biểu hiện cụ thể:**
- Phải để terminal visible để không bỏ lỡ khi agent xong
- Không thể ra ngoài hoặc làm meeting vì không có notification
- Đôi khi không biết agent đã xong từ lúc nào → delay tiếp tục workflow
- Ước tính trung bình mất 30-60 phút chờ mỗi agent session

**Chi phí ước tính:** 2-4 giờ/ngày bị "giam cầm" trước màn hình chờ agent

---

### PP04-02: Không Thể Gửi Follow-up Khi Xa Máy Tính

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** 3-5 lần/ngày  

**Mô tả:**
Agent hoàn thành task 1, nhưng task 2 phụ thuộc vào kết quả task 1. Sam đang ở cuộc họp không thể về máy tính ngay để gửi follow-up prompt. Workflow bị block 30-90 phút chờ Sam về.

**Biểu hiện cụ thể:**
- Workflow bị chặn trong thời gian Sam không ở máy tính
- Agent idle và lãng phí resources khi Sam đi họp
- Mỗi lần bị block như vậy mất 30-90 phút tiến độ
- Không thể parallel: gặp đối tác + agent làm việc cùng lúc

**Chi phí ước tính:** 1-3 giờ workflow delay mỗi ngày vì không gửi được follow-up từ mobile

---

### PP04-03: Không Biết Agent Status Khi Không Ở Máy

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Hằng ngày  

**Mô tả:**
Sam không có cách nào biết agent đang làm gì khi không ở trước máy tính. Đang chạy? Đã xong? Bị lỗi và stuck từ 2 giờ trước? Sam chỉ biết khi ngồi vào máy check.

**Biểu hiện cụ thể:**
- Hay phát hiện agent đã fail từ lâu, lãng phí thời gian server
- Không có cách ưu tiên khi nào cần về máy kiểm tra gấp
- Stress khi không biết status → check phone liên tục (nhưng không có app)
- Không thể plan thời gian dựa trên estimated completion

---

### PP04-04: Rate Limit Xảy Ra Khi Sam Không Ở Máy

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** 1-2 lần/tuần  

**Mô tả:**
Agent bị rate limited trong khi Sam đang họp. Agent dừng lại và không có ai để switch account hoặc switch agent. Khi Sam về sẽ phát hiện agent đã stuck 2 giờ trước.

**Biểu hiện cụ thể:**
- Rate limit xảy ra → agent stop → 2 giờ lãng phí
- Không có alert về rate limit khi không ở máy
- Không thể remote switch account từ điện thoại
- Mỗi incident như vậy block 2-4 giờ tiến độ

---

### PP04-05: Không Có Lịch Sử Agent Sessions Trên Mobile

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Hằng ngày  

**Mô tả:**
Khi đang di chuyển, Sam muốn xem lại những gì agent đã làm trong session vừa rồi để chuẩn bị cho cuộc họp hoặc review với team. Không có cách nào xem session history từ điện thoại.

**Biểu hiện cụ thể:**
- Không thể review agent output từ điện thoại
- Phải nhớ hoặc ghi chép thủ công những gì agent đã làm
- Không thể show progress cho stakeholder từ mobile
- Phải mở laptop để xem history — bất tiện trong cuộc họp

---

### PP04-06: Automation Workflows Không Chạy Tự Động

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Hằng ngày (tasks lặp đi lặp lại)  

**Mô tả:**
Sam có nhiều tasks lặp đi lặp lại (daily standup prep, weekly code review, issue triage) mà muốn tự động hóa. Hiện tại phải thủ công trigger mỗi lần — nếu Sam không ở máy thì không ai trigger.

**Biểu hiện cụ thể:**
- Phải nhớ trigger automation mỗi sáng thứ Hai
- Hay quên → tasks không được thực hiện → miss deadline nhỏ
- Không có cron-based automation cho recurring tasks
- Mỗi manual trigger mất 2-5 phút setup

---

### PP04-07: Setup Mobile ↔ Desktop Pairing Phức Tạp

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Lần đầu setup và khi đổi device  

**Mô tả:**
Nếu có mobile companion app, việc kết nối mobile với desktop cần đơn giản và nhanh. Nếu pairing process phức tạp (nhập IP địa chỉ thủ công, cần same WiFi network, v.v.), Sam sẽ bỏ không dùng.

**Biểu hiện cụ thể:**
- Các giải pháp hiện có (ngrok, port forwarding) quá phức tạp để setup
- Không hoạt động khi mobile và desktop không cùng mạng
- Security concern khi expose internal service ra internet
- Mất 30-60 phút setup cho một giải pháp thủ công

---

## Tổng hợp Impact

| Painpoint | Mức độ | Thời gian mất/ngày | Tần suất |
|-----------|--------|-------------------|---------|
| PP04-01: Phải chờ agent xong | 🔴 Critical | 2-4 giờ | 5-15 lần/ngày |
| PP04-02: Không gửi follow-up từ mobile | 🔴 Critical | 1-3 giờ | 3-5 lần/ngày |
| PP04-03: Không biết agent status | 🟠 High | 30-60 phút | Hằng ngày |
| PP04-04: Rate limit unattended | 🟠 High | 2-4 giờ/incident | 1-2 lần/tuần |
| PP04-05: Không xem history từ mobile | 🟡 Medium | 15-30 phút | Hằng ngày |
| PP04-06: Không có automation | 🟡 Medium | 15-30 phút | Hằng ngày |
| PP04-07: Pairing phức tạp | 🟡 Medium | 30-60 phút | One-time |

**Tổng thời gian lãng phí ước tính:** **4-8 giờ/ngày** — 50-100% năng suất tiềm năng với AI bị lãng phí do Sam phải ở trước máy tính

---

## Nguyên nhân gốc rễ

1. **AI agent workflow bị "giam" trong desktop application** — không có mobile-first interface
2. **Push notification không được thiết kế cho developer workflow** — chỉ có OS notification, không có smart notification
3. **Không có async workflow** — mọi interaction với agent cần Sam hiện diện real-time
4. **Thiếu automation engine tích hợp** — không có cách trigger agent tự động theo lịch

---

*Tham chiếu: URD §2.1 (Persona Sam), §3.4 (UR-030 đến UR-032)*
