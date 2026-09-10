# User Requirements Document (URD)

**Sản phẩm:** Orca — AI Orchestrator IDE  
**Phiên bản tài liệu:** 2.0  
**Ngày:** 2026-07-21 | **Cập nhật:** 2026-08-01  
**Phiên bản sản phẩm:** 5.0 (Profile Hierarchy + AI Provider + Workflow + Task Graph + Project Workspace + Remote Git UI + Full-Flow Tracing)  
**Tài liệu liên quan:** PRD.md, SRS.md  

---

## 1. Giới thiệu

Tài liệu URD (User Requirements Document) mô tả **yêu cầu từ góc nhìn người dùng** đối với Orca. Tài liệu này trả lời câu hỏi: *Người dùng cần gì từ hệ thống?* — không phải *hệ thống làm gì*.

### 1.1 Phạm vi

Tài liệu này bao gồm yêu cầu người dùng đối với:
- Ứng dụng desktop Orca (macOS, Windows, Linux)
- Ứng dụng companion mobile (iOS, Android)
- Orca CLI

### 1.2 Phân loại yêu cầu

- **MUST**: Yêu cầu bắt buộc (Must Have)
- **SHOULD**: Nên có (Should Have)
- **COULD**: Có thể có (Could Have)
- **WON'T**: Không có trong phạm vi này

---

## 2. Hồ sơ người dùng (User Personas)

### Persona 1: Alex — Senior Full-Stack Developer

> *"Tôi muốn thử 3 cách tiếp cận khác nhau cùng lúc và chọn cách tốt nhất."*

- **Background:** 8 năm kinh nghiệm, làm việc tại startup giai đoạn Series A
- **Tools hiện tại:** VSCode, iTerm2, GitHub, Linear
- **Nỗi đau:** Phải mở nhiều terminal, nhiều cửa sổ IDE, mất nhiều thời gian context switching
- **Kỳ vọng:** Một nơi để chạy nhiều AI agent cùng lúc, so sánh kết quả, và chọn ra solution tốt nhất

### Persona 2: Maya — Tech Lead

> *"Tôi cần review code AI sinh ra một cách nhanh chóng, có thể comment và gửi feedback ngay."*

- **Background:** Tech lead cho team 6 người, review 20+ PR/tuần
- **Nỗi đau:** AI agents tạo ra code nhưng phải review qua GitHub rồi quay lại terminal để sửa
- **Kỳ vọng:** Review AI diff inline, comment từng dòng, gửi feedback về agent mà không rời app

### Persona 3: Carlos — Remote Developer

> *"Máy local của tôi yếu. Tôi cần chạy agent trên server mạnh nhưng vẫn kiểm soát từ laptop."*

- **Background:** Freelancer, làm việc trên nhiều dự án
- **Nỗi đau:** SSH session mất kết nối liên tục, phải setup lại môi trường
- **Kỳ vọng:** Kết nối SSH ổn định, auto-reconnect, port forwarding tự động

### Persona 4: Sam — Mobile-First Power User

> *"Tôi cần biết ngay khi agent xong việc, dù đang ở đâu."*

- **Background:** CTO startup nhỏ, di chuyển nhiều
- **Nỗi đau:** Phải ngồi trước màn hình chờ agent xong, không làm việc khác được
- **Kỳ vọng:** Nhận notification trên điện thoại, gửi follow-up từ mobile

### Persona 5: Admin — Quản trị viên Orca Server

> *"Tôi cần quản lý ai được dùng Orca Server, xem ai đang online và audit mọi action."*

- **Background:** DevOps/Platform engineer quản lý Orca triển khai nội bộ cho team 10–50 người
- **Nỗi đau:** Không có công cụ quản lý user, không biết ai đang dùng session nào, không có audit trail
- **Kỳ vọng:** Admin dashboard rõ ràng, tạo/vô hiệu hóa user dễ dàng, kill session khi cần, xem audit log

### Persona 6: Agent Developer — Developer viết AI Agent WebSocket

> *"Tôi muốn viết một AI agent tích hợp với Orca qua WebSocket — cần protocol rõ ràng và SDK/guide."*

- **Background:** Developer xây dựng AI agent tùy chỉnh (Python, Go, TypeScript) cho team
- **Nỗi đau:** Không biết cách kết nối agent vào Orca, không có spec cụ thể
- **Kỳ vọng:** Wire protocol rõ ràng, SDK hoặc guide cho từng ngôn ngữ, token management dễ dàng

### Persona 9: DevOps / Observability Engineer

> *"Tôi cần thấy trực tiếp thao tác nào chậm, lỗi xảy ra ở tầng nào — không cần mở SSH terminal."*

- **Background:** DevOps hoặc Platform Engineer quản lý Orca Server
- **Nỗi đau:** Khi user báo lỗi, không biết lỗi xảy ra ở đâu (browser? relay? agent?), phải xem log thủ công trên nhiều tầng
- **Kỳ vọng:** Một UI trace panel hiển thị toàn bộ span timeline real-time, bật/tắt trace không cần restart

---

## 3. Yêu cầu người dùng

### 3.1 Quản lý Agent và Worktree

#### UR-001: Tạo và quản lý worktree

**Nhóm người dùng:** Alex, Maya  
**Ưu tiên:** MUST  

> *Là một developer, tôi muốn tạo nhiều worktree git độc lập để nhiều AI agent có thể làm việc song song mà không ảnh hưởng nhau.*

**Tiêu chí chấp nhận:**
- Người dùng có thể tạo worktree mới từ nhánh bất kỳ trong < 30 giây
- Mỗi worktree có terminal và file explorer riêng
- Người dùng có thể xem tất cả worktree đang chạy trong một danh sách
- Người dùng có thể xóa worktree khi không cần nữa
- Hệ thống cảnh báo trước khi xóa worktree có uncommitted changes

---

#### UR-002: Fan-out prompt tới nhiều agent

**Nhóm người dùng:** Alex  
**Ưu tiên:** MUST  

> *Tôi muốn gửi cùng một prompt tới 3-5 AI agent và xem mỗi agent giải quyết vấn đề theo cách nào.*

**Tiêu chí chấp nhận:**
- Người dùng có thể chọn số lượng worktree để fan-out (1-10)
- Mỗi worktree nhận prompt giống nhau và chạy độc lập
- Người dùng có thể xem tiến trình của từng agent trong thời gian thực
- Người dùng có thể dừng một agent cụ thể mà không ảnh hưởng agent khác

---

#### UR-003: Chọn và merge worktree thắng

**Nhóm người dùng:** Alex  
**Ưu tiên:** MUST  

> *Sau khi xem xét kết quả, tôi muốn merge worktree tốt nhất vào nhánh chính.*

**Tiêu chí chấp nhận:**
- Người dùng có thể so sánh diff giữa các worktree
- Người dùng có thể merge worktree được chọn vào nhánh chính
- Các worktree khác được cleanup tự động hoặc thủ công

---

#### UR-004: Quản lý nhiều AI agent providers

**Nhóm người dùng:** Alex, Sam  
**Ưu tiên:** MUST  

> *Tôi muốn sử dụng Claude Code, Codex, và Copilot trong cùng một workspace tùy theo task.*

**Tiêu chí chấp nhận:**
- Orca phát hiện tự động các CLI agent đã cài đặt trên hệ thống
- Người dùng có thể chọn agent nào cho từng worktree
- Người dùng có thể cấu hình default agent cho project
- Các agent khác nhau không ảnh hưởng nhau

---

#### UR-005: Theo dõi usage và rate limits

**Nhóm người dùng:** Alex, Sam  
**Ưu tiên:** SHOULD  

> *Tôi muốn biết mình đã dùng bao nhiêu tokens/credits và còn bao nhiêu trước khi bị rate limit.*

**Tiêu chí chấp nhận:**
- Người dùng thấy usage hiện tại theo từng provider (Claude, Codex, v.v.)
- Hệ thống hiển thị thời gian rate limit reset
- Người dùng có thể chuyển đổi account khi bị rate limit
- Cảnh báo khi gần đạt giới hạn

---

### 3.2 Terminal

#### UR-010: Sử dụng terminal tích hợp

**Nhóm người dùng:** Tất cả  
**Ưu tiên:** MUST  

> *Tôi muốn có terminal tích hợp mà không cần mở ứng dụng terminal riêng.*

**Tiêu chí chấp nhận:**
- Terminal khởi động trong < 1 giây
- Terminal hỗ trợ đầy đủ màu sắc 256-bit, emoji, unicode
- Terminal hỗ trợ các shell phổ biến (bash, zsh, fish, PowerShell)
- Người dùng có thể copy/paste bình thường

---

#### UR-011: Chia màn hình terminal

**Nhóm người dùng:** Alex, Carlos  
**Ưu tiên:** MUST  

> *Tôi muốn xem output của nhiều agent cùng lúc trong cùng một màn hình.*

**Tiêu chí chấp nhận:**
- Người dùng có thể chia terminal theo chiều ngang và chiều dọc
- Có thể mở nhiều tab terminal
- Mỗi split/tab có thể chạy process riêng
- Người dùng có thể resize các pane bằng cách kéo

---

#### UR-012: Khôi phục scrollback sau restart

**Nhóm người dùng:** Alex, Carlos  
**Ưu tiên:** SHOULD  

> *Tôi muốn xem lại output của agent từ session trước dù đã đóng app.*

**Tiêu chí chấp nhận:**
- Scrollback buffer được lưu khi đóng app
- Khi mở lại, người dùng thấy đầy đủ output từ session trước
- Buffer không bị giới hạn quá nhỏ (tối thiểu 100K dòng)

---

#### UR-013: Terminal hoạt động qua SSH

**Nhóm người dùng:** Carlos  
**Ưu tiên:** MUST  

> *Tôi muốn terminal kết nối tới server remote và chạy agent ở đó.*

**Tiêu chí chấp nhận:**
- Terminal mở được trên SSH host
- Copy/paste hoạt động qua SSH
- Terminal tự động reconnect khi mất kết nối
- Độ trễ hiển thị chấp nhận được ngay cả với kết nối chậm

---

### 3.3 SSH và Remote Development

#### UR-020: Kết nối SSH

**Nhóm người dùng:** Carlos  
**Ưu tiên:** MUST  

> *Tôi muốn kết nối tới server remote qua SSH và làm việc như local.*

**Tiêu chí chấp nhận:**
- Hỗ trợ xác thực bằng SSH key, password, SSH agent
- Orca đọc cấu hình từ ~/.ssh/config
- Người dùng có thể thêm host mới trong < 1 phút
- Kết nối được lưu để tái sử dụng

---

#### UR-021: Tự động kết nối lại

**Nhóm người dùng:** Carlos  
**Ưu tiên:** MUST  

> *Khi mạng bị gián đoạn, tôi không muốn phải tự tay kết nối lại.*

**Tiêu chí chấp nhận:**
- Orca tự động phát hiện mất kết nối
- Orca thử kết nối lại tự động trong vòng 10 giây
- Agent tiếp tục chạy trên remote trong khi chờ reconnect
- Người dùng thấy trạng thái reconnecting rõ ràng

---

#### UR-022: Port forwarding tự động

**Nhóm người dùng:** Carlos  
**Ưu tiên:** SHOULD  

> *Khi agent mở port trên server, tôi muốn truy cập được từ browser local.*

**Tiêu chí chấp nhận:**
- Orca phát hiện port mới mở trên remote
- Tự động forward port về local
- Thông báo cho người dùng biết local port nào đang được forward
- Người dùng có thể mở trong browser chỉ bằng một click

---

### 3.4 Mobile Companion

#### UR-030: Kết nối mobile với desktop

**Nhóm người dùng:** Sam  
**Ưu tiên:** MUST  

> *Tôi muốn dễ dàng kết nối điện thoại với Orca trên máy tính.*

**Tiêu chí chấp nhận:**
- Hiển thị mã QR trong app desktop
- Mobile scan QR là xong pairing
- Pairing hoàn thành trong < 30 giây
- Kết nối bền vững không bị mất khi chuyển mạng

---

#### UR-031: Nhận thông báo khi agent xong

**Nhóm người dùng:** Sam  
**Ưu tiên:** MUST  

> *Khi agent hoàn thành task, tôi muốn biết ngay dù đang rời xa máy tính.*

**Tiêu chí chấp nhận:**
- Push notification tới điện thoại khi agent kết thúc
- Notification hiển thị tên agent và trạng thái (success/error)
- Delivery trong < 5 giây sau khi agent kết thúc
- Notification hoạt động kể cả khi app mobile ở background

---

#### UR-032: Gửi follow-up từ mobile

**Nhóm người dùng:** Sam  
**Ưu tiên:** SHOULD  

> *Sau khi nhận thông báo, tôi muốn gửi thêm instructions mà không cần về máy tính.*

**Tiêu chí chấp nhận:**
- Người dùng có thể gõ prompt từ mobile và gửi về agent
- Agent nhận và xử lý prompt từ mobile
- Người dùng thấy trạng thái agent cập nhật trong thời gian thực

---

### 3.5 Review và Source Control

#### UR-040: Xem Pull Request

**Nhóm người dùng:** Maya  
**Ưu tiên:** MUST  

> *Tôi muốn xem và review PR trong Orca mà không cần mở GitHub trên browser.*

**Tiêu chí chấp nhận:**
- Danh sách PR hiển thị đầy đủ (tiêu đề, author, status, CI checks)
- Người dùng có thể xem diff của từng file trong PR
- Diff hiển thị đúng màu sắc, syntax highlighting
- Người dùng có thể comment trên PR

---

#### UR-040b: Xem Full-Flow Trace trong TracePanel UI

**Nhóm người dùng:** Persona 9 (DevOps), Maya, Carlos  
**Ưu tiên:** SHOULD  

> *Khi có thao tác chậm hoặc lỗi, tôi muốn thấy toàn bộ trace span từ Browser → Relay → Agent trong một UI, không cần SSH vào server xem log.*

**Tiêu chí chấp nhận:**
- TracePanel hiển thị tất cả spans real-time (nhận qua SSE `/api/trace-stream`)
- Mỗi span có `id`, `flow`, các bước step và `elapsedMs`
- `fail` events hiển thị màu đỏ, luôn hiển thị kể cả khi trace tắt
- Không cần SSH hoặc terminal để xem trace

---

#### UR-040c: Bật / Tắt trace không cần restart

**Nhóm người dùng:** Persona 9, Carlos  
**Ưu tiên:** SHOULD  

> *Tôi muốn bật trace để debug, rồi tắt lại — không được làm chậm hệ thống khi tắt.*

**Tiêu chí chấp nhận:**
- Node.js: set env var `ORCA_TRACE=1` và restart đủ làm cho console trace bật
- Browser: `localStorage.setItem('ORCA_TRACE', '1')` và reload bật console trace
- `fail` events luôn log dù flag nào
- Performance overhead gần bằng 0 khi trace tắt (sink không được gọi khi tắt console)

---

#### UR-041: Annotate AI diff

**Nhóm người dùng:** Maya  
**Ưu tiên:** MUST  

> *Khi agent thay đổi code, tôi muốn comment vào từng dòng và gửi feedback về agent để sửa.*

**Tiêu chí chấp nhận:**
- Người dùng có thể click vào bất kỳ dòng nào trong diff để thêm comment
- Comment được gửi về agent kèm context (file, line, original code)
- Agent nhận comment và điều chỉnh code
- Người dùng thấy thay đổi mới trong diff cập nhật

---

#### UR-042: Tạo worktree từ issue/task

**Nhóm người dùng:** Maya  
**Ưu tiên:** SHOULD  

> *Khi tôi mở một GitHub issue hoặc Linear task, tôi muốn tạo ngay worktree và giao cho agent.*

**Tiêu chí chấp nhận:**
- Từ danh sách issues/tasks, người dùng có thể tạo worktree bằng 1-2 click
- Worktree mới được tạo với branch name từ issue title
- Agent được khởi động với context từ issue description
- Issue/task được link với worktree để dễ tracking

---

#### UR-043: Tự động tạo commit message

**Nhóm người dùng:** Maya, Alex  
**Ưu tiên:** SHOULD  

> *Tôi không muốn tốn thời gian viết commit message, muốn AI tạo ra dựa trên thay đổi.*

**Tiêu chí chấp nhận:**
- Người dùng có thể request AI tạo commit message từ staged changes
- Message được tạo trong < 10 giây
- Người dùng có thể chỉnh sửa trước khi commit
- Message tuân thủ convention của dự án

---

### 3.6 Design Mode

#### UR-050: Trích xuất UI element cho agent

**Nhóm người dùng:** Alex  
**Ưu tiên:** SHOULD  

> *Khi tôi muốn agent sửa một phần giao diện, tôi muốn click vào element đó và agent tự hiểu context.*

**Tiêu chí chấp nhận:**
- Người dùng có thể mở browser trong Orca
- Click vào UI element để capture HTML, CSS, screenshot
- Context được tự động thêm vào agent prompt
- Người dùng không cần copy/paste HTML thủ công

---

### 3.7 Editor và File Management

#### UR-060: Chỉnh sửa file

**Nhóm người dùng:** Tất cả  
**Ưu tiên:** MUST  

> *Tôi muốn chỉnh sửa file trực tiếp trong Orca mà không cần mở editor riêng.*

**Tiêu chí chấp nhận:**
- Editor có syntax highlighting cho ngôn ngữ phổ biến
- Autosave khi rời khỏi file
- Undo/redo hoạt động bình thường
- Tìm/thay thế trong file

---

#### UR-061: Kéo thả file vào agent prompt

**Nhóm người dùng:** Alex, Maya  
**Ưu tiên:** SHOULD  

> *Tôi muốn kéo file hoặc ảnh thả trực tiếp vào chat với agent.*

**Tiêu chí chấp nhận:**
- Drag file từ file explorer vào prompt của agent
- Drag ảnh từ máy tính vào prompt
- File content được tự động đính kèm vào prompt

---

#### UR-062: Preview file

**Nhóm người dùng:** Maya  
**Ưu tiên:** COULD  

> *Tôi muốn preview Markdown, PDF, và ảnh ngay trong Orca.*

**Tiêu chí chấp nhận:**
- Markdown được render với đầy đủ formatting
- PDF được hiển thị inline
- Ảnh được hiển thị đúng kích thước

---

### 3.8 Automation và CLI

#### UR-070: Tự động hóa workflow

**Nhóm người dùng:** Sam, Carlos  
**Ưu tiên:** COULD  

> *Tôi muốn đặt lịch để Orca tự động chạy một agent vào mỗi sáng thứ Hai.*

**Tiêu chí chấp nhận:**
- Người dùng có thể tạo automation với trigger (cron, event)
- Automation có thể tạo worktree, chạy agent, commit kết quả
- Lịch sử automation runs được lưu lại
- Người dùng có thể bật/tắt automation

---

#### UR-071: Script Orca qua CLI

**Nhóm người dùng:** Carlos, DevOps  
**Ưu tiên:** SHOULD  

> *Tôi muốn tích hợp Orca vào CI/CD pipeline bằng command line.*

**Tiêu chí chấp nhận:**
- CLI hoạt động trên macOS, Linux, Windows
- `orca worktree create` tạo worktree từ tham số
- `orca serve` chạy Orca ở chế độ headless
- CLI có documentation đầy đủ (`--help`)

---

### 3.9 Cài đặt và Cấu hình

#### UR-080: Cài đặt dễ dàng

**Nhóm người dùng:** Tất cả  
**Ưu tiên:** MUST  

> *Tôi muốn cài Orca và dùng được ngay trong < 5 phút.*

**Tiêu chí chấp nhận:**
- Một lần download, không cần cấu hình phức tạp
- Onboarding wizard hướng dẫn setup agent đầu tiên
- Hỗ trợ Homebrew, AUR cho power user
- Không yêu cầu tài khoản để dùng cơ bản

---

#### UR-081: Tự động cập nhật

**Nhóm người dùng:** Tất cả  
**Ưu tiên:** MUST  

> *Tôi không muốn phải tự tay cập nhật app, muốn nhận bản mới tự động.*

**Tiêu chí chấp nhận:**
- App kiểm tra update tự động
- Thông báo khi có bản mới
- Người dùng có thể chọn khi nào cài update
- Cập nhật không làm mất dữ liệu

---

#### UR-082: Cấu hình per-project

**Nhóm người dùng:** Alex, Carlos  
**Ưu tiên:** SHOULD  

> *Mỗi dự án có cài đặt riêng (default agent, branch naming, v.v.).*

**Tiêu chí chấp nhận:**
- File `orca.yaml` trong root dự án
- Cấu hình default agent, environment variables
- Cấu hình được apply tự động khi mở project

---

### 3.10 Hiệu năng và Độ tin cậy

#### UR-090: Startup nhanh

**Nhóm người dùng:** Tất cả  
**Ưu tiên:** MUST  

> *Tôi muốn Orca mở ngay, không phải chờ lâu.*

**Tiêu chí chấp nhận:**
- App khởi động và sẵn sàng trong < 3 giây
- Không block UI trong quá trình load background data

---

#### UR-091: Không treo khi xử lý nhiều agent

**Nhóm người dùng:** Alex  
**Ưu tiên:** MUST  

> *Dù có 5 agent đang chạy, UI vẫn phải phản hồi ngay lập tức.*

**Tiêu chí chấp nhận:**
- UI không freeze khi agent đang xử lý
- Scrolling và typing vẫn mượt với 5+ agent chạy song song
- Memory usage không tăng không giới hạn

---

### 3.11 Bảo mật và Quyền riêng tư

#### UR-100: Kiểm soát permissions của agent

**Nhóm người dùng:** Maya, Alex  
**Ưu tiên:** MUST  

> *Tôi muốn kiểm soát agent được phép làm gì (có thể xóa file không? chạy lệnh nào?).*

**Tiêu chí chấp nhận:**
- Người dùng cấu hình trust level cho từng agent
- Agent không thể làm những gì ngoài phạm vi được cấp
- Hệ thống thông báo khi agent yêu cầu permission mới

---

#### UR-101: Quyền riêng tư dữ liệu

**Nhóm người dùng:** Tất cả  
**Ưu tiên:** MUST  

> *Tôi không muốn Orca thu thập code hoặc prompt của tôi.*

**Tiêu chí chấp nhận:**
- Chỉ thu thập dữ liệu usage ẩn danh (không có nội dung)
- Người dùng có thể opt-out telemetry hoàn toàn
- Tài liệu rõ ràng về những gì được thu thập
- Không có server phân tích nội dung code/prompt

---

### 3.12 Multi-User Server Mode

#### UR-110: Đăng nhập vào Orca Server

**Nhóm người dùng:** Mọi user khi dùng Orca Web Server  
**Ưu tiên:** MUST  

> *Tôi muốn đăng nhập bằng email/password và có session riêng tư, không phải xử lý PairCode thủ công.*

**Tiêu chí chấp nhận:**
- Login form ở `/login` với email + password fields
- Session tồn tại 8h, tự động renew khi có activity
- `GET /auth/me` trả về thông tin hiện tại
- Logout xóa session ngay lập tức
- PairCode vẫn hoạt động song song (backward compat)

---

#### UR-111: Isolation giữa các user

**Nhóm người dùng:** Mọi user khi dùng Orca Web Server  
**Ưu tiên:** MUST  

> *Tôi không muốn user khác đọc được project của mình hay nhìn thấy worktrees của tôi.*

**Tiêu chí chấp nhận:**
- Mỗi user có process Node.js riêng (`fork()`)
- Data path riêng: `~/.orca/users/<userId>/`
- SSH connection store isolated per user
- Một user crash không ảnh hưởng user khác

---

#### UR-112: Admin quản lý users và sessions

**Nhóm người dùng:** Admin  
**Ưu tiên:** MUST  

> *Tôi muốn xem tất cả users, tạo user mới, vô hiệu hóa user rời khỏi team, và kill session khẩn cấp.*

**Tiêu chí chấp nhận:**
- Dashboard `/admin` hiển thị tổng số users, sessions, uptime
- Tạo user mới với email, name, role (developer/lead/admin)
- Vô hiệu hóa user ngay lập tức (kick tất cả sessions)
- Kill session bất kỳ đang active
- Tìm kiếm users theo email, role, status

---

#### UR-113: Audit log rõ ràng

**Nhóm người dùng:** Admin  
**Ưu tiên:** MUST  

> *Tôi cần biết ai đã đăng nhập khi nào, ai đã tạo/xóa user, ai khởi tạo SSH connection.*

**Tiêu chí chấp nhận:**
- Audit log có timestamp, actor, action, target, IP
- Filter theo user, action type, date range
- Export audit log ra CSV
- Events: login, logout, user CRUD, session kill, ssh.connect

---

### 3.13 Fleet Management

#### UR-120: Khai báo fleet as-code

**Nhóm người dùng:** Admin, Carlos (DevOps)  
**Ưu tiên:** MUST  

> *Tôi muốn định nghĩa danh sách dev servers trong một file YAML và import vào Orca một lần.*

**Tiêu chí chấp nhận:**
- `orca-fleet.yaml` có cấu trúc rõ ràng với servers, projects, defaults
- `orca fleet import --file orca-fleet.yaml` import thành công
- Server mới được thêm vào SSH targets của Orca
- Không làm đứt SSH targets hiện có

---

#### UR-121: Wizard đăng ký dev server

**Nhóm người dùng:** Carlos, Admin  
**Ưu tiên:** MUST  

> *Khi tôi muốn thêm dev server mới, tôi muốn được dẫn dắt từng bước đến khi relay được deploy và kết nối thành công.*

**Tiêu chí chấp nhận:**
- Wizard step-by-step: SSH → Platform detect → Agent detect → Preflight → Deploy relay → Done
- Progress bar hiển thị từng bước
- Nếu step thất bại: hiển thị lỗi rõ ràng và cho retry
- Sau khi hoàn tất: dev server hiển thị trong danh sách

---

#### UR-122: Theo dõi sức khỏe fleet

**Nhóm người dùng:** Admin, DevOps  
**Ưu tiên:** SHOULD  

> *Tôi muốn biết ngay khi một server xuống, và xem CPU/RAM của từng server.*

**Tiêu chí chấp nhận:**
- Dashboard hiển thị status card cho từng server (healthy/degraded/unhealthy)
- Metrics: CPU%, RAM%, disk%, SSH latency
- Webhook alert khi server đổi trạng thái
- Prometheus metrics tại `/metrics`

---

### 3.14 Agent WebSocket Integration

#### UR-130: Kết nối agent tự viết qua WebSocket

**Nhóm người dùng:** Agent Developer  
**Ưu tiên:** MUST  

> *Tôi muốn kết nối AI agent tôi viết (Python/Go) với Orca qua WebSocket mà không cần chạy qua SSH.*

**Tiêu chí chấp nhận:**
- Tài liệu wire protocol rõ ràng (13-byte header format)
- Guide cách implement WS server cho relay-websocket mode
- Guide cách implement WS client cho direct-websocket mode
- Code example cho TypeScript, Python, Go

---

#### UR-131: Quản lý agent token

**Nhóm người dùng:** Agent Developer, Admin  
**Ưu tiên:** MUST  

> *Tôi muốn tạo token cho agent và revoke khi cần, từ UI.*

**Tiêu chí chấp nhận:**
- UI hiển thị agent token trong DevServer settings panel
- Nút tạo token mới (revoke cũ)
- Token hiển thị một lần duy nhất khi tạo, sau đó masked
- Copy token button

---

## 4. Ma trận yêu cầu người dùng

| ID | Yêu cầu | Persona | Ưu tiên | Phần trong SRS |
|----|---------|---------|---------|----------------|
| UR-001 | Tạo và quản lý worktree | Alex, Maya | MUST | FR-1.1 |
| UR-002 | Fan-out prompt | Alex | MUST | FR-1.2 |
| UR-003 | Merge worktree | Alex | MUST | FR-1.3 |
| UR-004 | Multi-agent provider | Alex, Sam | MUST | FR-2.1 |
| UR-005 | Usage tracking | Alex, Sam | SHOULD | FR-2.2 |
| UR-010 | Terminal tích hợp | Tất cả | MUST | FR-3.1 |
| UR-011 | Terminal splits | Alex, Carlos | MUST | FR-3.2 |
| UR-012 | Scrollback persistence | Alex, Carlos | SHOULD | FR-3.3 |
| UR-013 | Terminal qua SSH | Carlos | MUST | FR-4.1 |
| UR-020 | Kết nối SSH | Carlos | MUST | FR-4.1 |
| UR-021 | Auto-reconnect | Carlos | MUST | FR-4.2 |
| UR-022 | Port forwarding | Carlos | SHOULD | FR-4.3 |
| UR-030 | Mobile pairing | Sam | MUST | FR-5.1 |
| UR-031 | Push notification | Sam | MUST | FR-5.2 |
| UR-032 | Follow-up từ mobile | Sam | SHOULD | FR-5.3 |
| UR-040 | Xem PR | Maya | MUST | FR-6.1 |
| UR-040b | TracePanel UI (full-flow trace) | DevOps, Dev | SHOULD | FR-9.1, FR-9.3 |
| UR-040c | Bật/Tắt trace flag | DevOps, Dev | SHOULD | FR-9.1 |
| UR-041 | Annotate AI diff | Maya | MUST | FR-6.2 |
| UR-042 | Worktree từ issue | Maya | SHOULD | FR-6.3 |
| UR-043 | AI commit message | Maya, Alex | SHOULD | FR-6.4 |
| UR-050 | Design Mode | Alex | SHOULD | FR-7.1 |
| UR-060 | File editor | Tất cả | MUST | FR-8.1 |
| UR-061 | Drag file vào prompt | Alex, Maya | SHOULD | FR-8.2 |
| UR-062 | Preview file | Maya | COULD | FR-8.3 |
| UR-070 | Automation | Sam, Carlos | COULD | FR-9.1 |
| UR-071 | CLI scripting | Carlos, DevOps | SHOULD | FR-9.2 |
| UR-080 | Cài đặt dễ | Tất cả | MUST | NFR-1 |
| UR-081 | Auto-update | Tất cả | MUST | NFR-2 |
| UR-082 | Per-project config | Alex, Carlos | SHOULD | NFR-3 |
| UR-090 | Startup nhanh | Tất cả | MUST | NFR-4 |
| UR-091 | Không treo | Alex | MUST | NFR-5 |
| UR-100 | Agent permissions | Maya, Alex | MUST | NFR-6 |
| UR-101 | Quyền riêng tư | Tất cả | MUST | NFR-7 |
| UR-110 | Đăng nhập Orca Server | Mọi user | MUST | FR-11.1 |
| UR-111 | User isolation | Mọi user | MUST | FR-12.1 |
| UR-112 | Admin quản lý users/sessions | Admin | MUST | FR-13.1 |
| UR-113 | Audit log | Admin | MUST | FR-13.2 |
| UR-120 | Fleet as-code | Admin, Carlos | MUST | FR-15.1 |
| UR-121 | Dev Server wizard | Carlos, Admin | MUST | FR-16 |
| UR-122 | Fleet health monitoring | Admin, DevOps | SHOULD | FR-15.3 |
| UR-130 | Agent WebSocket connection | Agent Dev | MUST | FR-17 |
| UR-131 | Agent token management | Agent Dev, Admin | MUST | FR-17.1 |

---

## 5. Ràng buộc người dùng

### 5.1 Môi trường sử dụng

- Người dùng sử dụng trên macOS 12+, Windows 10+, hoặc Linux (Ubuntu 20.04+)
- Kết nối internet để sử dụng AI agent (bắt buộc)
- Ít nhất 8GB RAM để chạy nhiều agent song song hiệu quả
- Git 2.25+ đã được cài đặt

### 5.2 Kỹ năng người dùng

- Người dùng có kiến thức cơ bản về Git (commit, branch, merge)
- Người dùng đã biết sử dụng ít nhất một AI agent CLI
- Không yêu cầu kiến thức về SSH để dùng tính năng cơ bản
- Admin: cần hiểu cơ bản về user management và SSH
- Agent Developer: cần biết WebSocket và binary protocol

### 5.3 Ngôn ngữ

- Giao diện chính bằng tiếng Anh
- Hỗ trợ tiếng Trung (Simplified), Nhật, Hàn, Tây Ban Nha, Bồ Đào Nha

---

*Tài liệu này được cập nhật dựa trên codebase Orca v4.1 (2026-07-28).*

---

## Personas bổ sung (v5.0)

### Persona 7: Company Admin — Quản trị viên doanh nghiệp

**Mô tả:** Người quản trị nền tảng Orca cho toàn tổ chức.  
**Mục tiêu:** Chuẩn hóa cấu hình AI cho tất cả developer, quản lý AI provider accounts, audit.  
**Nhu cầu:** Company profile management, AI provider setup per dev server, fleet overview, full audit log.

### Persona 8: Team Lead — Trưởng nhóm kỹ thuật

**Mô tả:** Người dẫn dắt team backend/frontend, phân công và theo dõi tiến độ.  
**Mục tiêu:** Chuẩn hóa workflow cho team, phân rã task, theo dõi agent execution.  
**Nhu cầu:** Team profile, workflow templates (team-scope), task graph + AI decompose, task grant management.

---

## Yêu cầu người dùng bổ sung (v5.0)

### 3.15 Profile Hierarchy & Project Management

| UR | Mô tả | Persona | Mức độ | FR-ref |
|----|-------|---------|--------|--------|
| UR-140 | Xem và chỉnh sửa Company profile (agent settings, security, models) | Admin | MUST | FR-19.1 |
| UR-141 | Xem profile hiệu lực (merged từ 3 tầng) với source attribution | Tất cả | MUST | FR-19.2 |
| UR-142 | Override profile ở tầng cá nhân (envVars, shell settings, model preference) | Developer | MUST | FR-19.2 |
| UR-143 | Tạo và gắn project vào dev server cụ thể | Lead, Admin | MUST | FR-20 |
| UR-144 | Agent tự động chạy trên dev server của project (không cần chọn thủ công) | Developer | MUST | FR-20 |
| UR-145 | Xem danh sách projects của mình và switch workspace | Developer | MUST | FR-24 |

### 3.16 AI Provider Account Management

| UR | Mô tả | Persona | Mức độ | FR-ref |
|----|-------|---------|--------|--------|
| UR-150 | Setup AI provider account (Anthropic, OpenAI, Ollama...) trên dev server | Admin | MUST | FR-21 |
| UR-151 | Test kết nối provider trước khi lưu | Admin | MUST | FR-21 |
| UR-152 | Xem trạng thái health của tất cả provider accounts | Admin | SHOULD | FR-21 |
| UR-153 | Nhận alert khi quota vượt 80% | Admin | MUST | FR-21 |
| UR-154 | Rotate API key mà không downtime quá 30s | Admin | SHOULD | FR-21 |
| UR-155 | Agent tự động chọn provider phù hợp theo priority | Developer | MUST | FR-21 |
| UR-156 | Cài đặt provider scope (server/project/user) | Lead, Admin | MUST | FR-21 |

### 3.17 Workflow Orchestration

| UR | Mô tả | Persona | Mức độ | FR-ref |
|----|-------|---------|--------|--------|
| UR-160 | Tạo workflow chạy trên nhiều dev servers | Lead, Developer | SHOULD | FR-22 |
| UR-161 | Kế thừa workflow từ company/team template | Developer | MUST | FR-22.3 |
| UR-162 | Tùy chỉnh (override steps) trong workflow kế thừa | Developer | MUST | FR-22.3 |
| UR-163 | Xem tiến trình real-time của từng step | Developer | MUST | FR-22.2 |
| UR-164 | Chia sẻ workflow với team/company/public | Lead, Developer | SHOULD | FR-22.3 |
| UR-165 | Browse workflow library, rate và clone template | Developer | COULD | FR-22.3 |
| UR-166 | Workflow tiếp tục sau khi Orca restart | Developer | MUST | FR-22.2 |

### 3.18 Task Graph Management

| UR | Mô tả | Persona | Mức độ | FR-ref |
|----|-------|---------|--------|--------|
| UR-170 | Tạo task hierarchy (Epic → Story → Task → Subtask) | Lead, Developer | MUST | FR-23.1 |
| UR-171 | Định nghĩa dependency giữa các tasks (A depends on B) | Lead, Developer | MUST | FR-23.1 |
| UR-172 | AI đề xuất cách phân rã task thành subtasks | Lead | MUST | FR-23.2 |
| UR-173 | AI generate agent prompt từ task metadata | Developer | MUST | FR-23.2 |
| UR-174 | Chạy AI agent trực tiếp từ Task Detail | Developer | MUST | FR-23.4 |
| UR-175 | Xem agent output trong Task Activity Feed | Developer | MUST | FR-23.4 |
| UR-176 | Grant quyền cho task/task tree theo company/team/user | Lead | MUST | FR-23.3 |
| UR-177 | Chia sẻ task/task tree qua link (public view) | Lead, Developer | SHOULD | FR-23.3 |
| UR-178 | Task tự động advance status khi agent hoàn thành | Developer | MUST | FR-23.4 |
| UR-179 | Xem progress (% done) của task theo subtask completion | Lead, Developer | MUST | FR-23.1 |
| UR-180 | Xem critical path của task graph | Lead | COULD | FR-23.2 |

### 3.19 Project Workspace

| UR | Mô tả | Persona | Mức độ | FR-ref |
|----|-------|---------|--------|--------|
| UR-190 | Chọn project → load unified workspace (Explorer + Git + Agent + Tasks) | Developer | MUST | FR-24 |
| UR-191 | Workspace vẫn dùng được (read-only) khi dev server offline | Developer | MUST | FR-24.1 |
| UR-192 | Duyệt cây thư mục của repo trên dev server | Developer | MUST | FR-24.2 |
| UR-193 | Xem nội dung file trên dev server (syntax highlighted) | Developer | MUST | FR-24.2 |
| UR-194 | Tìm kiếm file theo tên hoặc nội dung trên dev server | Developer | SHOULD | FR-24.2 |
| UR-195 | Git status badges trên file tree (M/A/D/?) | Developer | MUST | FR-24.2 |
| UR-196 | Terminal mở với cwd = current worktree path | Developer | MUST | FR-24.3 |
| UR-197 | Auto-refresh Git tab và Explorer khi agent hoàn thành | Developer | MUST | FR-24.1 |

### 3.20 Remote Git UI

| UR | Mô tả | Persona | Mức độ | FR-ref |
|----|-------|---------|--------|--------|
| UR-200 | Xem git status (modified/staged/untracked) không cần SSH | Developer | MUST | FR-25 |
| UR-201 | Xem visual diff của từng file thay đổi | Developer | MUST | FR-25 |
| UR-202 | Stage/Unstage file hoặc tất cả thay đổi | Developer | MUST | FR-25 |
| UR-203 | Commit với message tự nhập hoặc AI generate | Developer | MUST | FR-25 |
| UR-204 | Push lên remote với progress stream | Developer | MUST | FR-25 |
| UR-205 | Pull với conflict detection và AI resolve | Developer | MUST | FR-25 |
| UR-206 | Tạo/switch branch không cần terminal | Developer | MUST | FR-25 |
| UR-207 | Tạo Pull Request từ UI (GitHub/GitLab) với AI description | Developer, Lead | MUST | FR-25 |
| UR-208 | Switch worktree và all panels tự động sync | Developer | MUST | FR-25 |
| UR-209 | Xem git log (50 commits, branch graph) | Developer | SHOULD | FR-25 |
| UR-210 | Commit message reference task ID → auto-close task | Developer | COULD | FR-25 |

---

## Cập nhật Ma trận yêu cầu (v5.0 additions)

| UR | Mô tả | Persona | Mức độ | FR-ref |
|----|-------|---------|--------|--------|
| UR-140 | Company profile management | Admin | MUST | FR-19 |
| UR-143 | Project-dev server binding | Lead, Admin | MUST | FR-20 |
| UR-150 | AI provider setup per server | Admin | MUST | FR-21 |
| UR-160 | Multi-server workflow | Lead, Developer | SHOULD | FR-22 |
| UR-170 | Task graph (epic/story/task) | Lead, Developer | MUST | FR-23 |
| UR-174 | Run agent from task | Developer | MUST | FR-23.4 |
| UR-190 | Project Workspace | Developer | MUST | FR-24 |
| UR-200 | Remote git status/diff | Developer | MUST | FR-25 |
| UR-203 | Commit + AI message | Developer | MUST | FR-25 |
| UR-207 | Create PR from UI | Developer, Lead | MUST | FR-25 |
| UR-040b | TracePanel UI | DevOps, Dev | SHOULD | FR-9.1, FR-9.3 |
| UR-040c | Toggle trace flag | DevOps, Dev | SHOULD | FR-9.1 |

---

*Tài liệu này được cập nhật dựa trên codebase Orca v5.0 — Profile Hierarchy, AI Provider Management, Workflow Orchestration, Task Graph, Project Workspace, Remote Git UI, Full-Flow Tracing (2026-08-01).*
