# PP01 — Painpoints: Senior Full-Stack Developer (Alex)

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | PP01 |
| **Actor** | Senior Full-Stack Developer |
| **Đại diện** | Alex — 8 năm kinh nghiệm, startup Series A |
| **Quote** | *"Tôi muốn thử 3 cách tiếp cận khác nhau cùng lúc và chọn cách tốt nhất."* |
| **Tham chiếu giải pháp** | [SOL01](../solutions/SOL01-senior-developer.md) |

---

## Bối cảnh

Alex là senior developer giàu kinh nghiệm đang tận dụng AI agent để tăng tốc độ viết code. Alex đã quen dùng Claude Code, Codex và muốn chạy chúng song song để so sánh kết quả. Tuy nhiên công cụ hiện tại (VSCode + iTerm2) không được thiết kế cho mô hình làm việc multi-agent.

---

## Danh sách Painpoints

### PP01-01: Context Switching Tốn Kém

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** Hằng ngày, liên tục  

**Mô tả:**
Khi chạy nhiều AI agent, Alex phải mở nhiều cửa sổ terminal (iTerm2), nhiều cửa sổ VSCode, nhiều tab browser (GitHub). Mỗi lần switch giữa agent A và agent B phải alt-tab nhiều lần, mất track "đang xem agent nào" và "đang review file nào".

**Biểu hiện cụ thể:**
- Phải mở 5+ cửa sổ riêng biệt để quản lý 3 agent
- Hay nhầm lẫn giữa các worktree của các agent khác nhau
- Thời gian tìm lại context (file đang xem, agent đang chạy) mất 2-5 phút mỗi lần switch
- Alt-tab liên tục làm mất tập trung, dễ nhầm lẫn

**Chi phí ước tính:** 2-4 giờ/ngày bị lãng phí vì context switching

---

### PP01-02: Không Thể So Sánh Kết Quả Song Song

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** Mỗi task phức tạp (3-5 lần/ngày)  

**Mô tả:**
Khi Alex muốn thử 3 cách giải quyết khác nhau, phải làm tuần tự: chạy agent A → xem kết quả → reset → chạy agent B → so sánh trong đầu. Không thể chạy đồng thời và so sánh trực quan.

**Biểu hiện cụ thể:**
- Phải nhớ kết quả của agent trước khi chạy agent sau
- Mất 30-60 phút để thử 3 approaches thay vì 10-20 phút nếu song song
- Không có công cụ để diff kết quả giữa các approaches
- Thường bỏ qua việc so sánh và chấp nhận solution đầu tiên "đủ tốt"

**Chi phí ước tính:** Chọn solution suboptimal 40% thời gian do không có đủ thời gian so sánh

---

### PP01-03: Worktree Bị Lẫn Lộn và Conflict

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** 2-3 lần/tuần  

**Mô tả:**
Khi tự tạo nhiều git branch để agent làm việc, Alex hay bị lẫn lộn giữa các branch. Agent A vô tình commit lên branch của agent B, hoặc changes của agent này gây conflict với agent kia vì không cô lập hoàn toàn.

**Biểu hiện cụ thể:**
- Nhầm lẫn directory khi chạy lệnh → agent làm việc sai branch
- Agent tạo file ở wrong directory
- Git status hiển thị changes của nhiều agent lẫn nhau
- Mất 20-30 phút để resolve conflicts do agent làm sai branch

**Chi phí ước tính:** 1-2 giờ/tuần giải quyết mess do worktree conflict

---

### PP01-04: Không Biết Agent Đang Làm Gì

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Hằng ngày  

**Mô tả:**
Khi chạy nhiều agent song song, Alex không có cái nhìn tổng quan về trạng thái của từng agent. Phải manually check từng terminal để biết agent nào xong, agent nào đang bị lỗi, agent nào đang chờ input.

**Biểu hiện cụ thể:**
- Không có unified status view cho tất cả agents
- Phải alt-tab vào từng terminal để check tiến trình
- Bỏ lỡ khi agent xong việc vì đang focus vào agent khác
- Không biết agent bị stuck hay vẫn đang chạy

**Chi phí ước tính:** 30-60 phút/ngày mất vào việc manually monitor agents

---

### PP01-05: Khó Tích Hợp Workflow với File Explorer và Browser

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Mỗi ngày  

**Mô tả:**
Để cho agent context đầy đủ, Alex cần share file content và browser screenshots. Hiện tại phải: mở file trong VSCode → copy content → paste vào terminal. Hoặc chụp screenshot browser → save file → drag vào prompt. Quá nhiều bước.

**Biểu hiện cụ thể:**
- Copy/paste file content thủ công vào agent prompt
- Không thể drag-drop file vào agent
- Mỗi lần share context tốn 1-2 phút
- Hay quên share đủ context → agent hiểu sai → phải làm lại

---

### PP01-06: Agent Bị Rate-Limited Không Có Fallback

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** 2-3 lần/tuần với heavy user  

**Mô tả:**
Khi Claude Code bị rate-limited, Alex phải: đóng terminal → mở terminal mới → đăng nhập tài khoản khác → chạy lại. Không có cơ chế hot-swap account hay chuyển sang agent khác giữa chừng.

**Biểu hiện cụ thể:**
- Mất 5-10 phút mỗi lần rate-limit xảy ra
- Không biết còn bao lâu nữa thì rate limit reset
- Phải nhớ có bao nhiêu account để switch
- Session cũ bị mất khi switch tài khoản

---

### PP01-07: Review Code Agent Tạo Ra Không Hiệu Quả

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Mỗi task hoàn thành  

**Mô tả:**
Sau khi agent xong, Alex phải review changes thủ công: mở `git diff` trong terminal, scroll qua output dài, copy file path ra, mở trong VSCode. Không có inline comment, không gửi được feedback dạng structured về agent.

**Biểu hiện cụ thể:**
- Review `git diff` trong terminal không có syntax highlighting
- Không thể comment trực tiếp vào dòng code cụ thể
- Feedback phải gõ lại vào terminal dưới dạng prompt vague
- Agent thường miss context khi feedback không attach đúng line number

---

## Tổng hợp Impact

| Painpoint | Mức độ | Thời gian mất/tuần | Tần suất |
|-----------|--------|-------------------|---------|
| PP01-01: Context Switching | 🔴 Critical | 10-20 giờ | Hằng ngày |
| PP01-02: Không so sánh song song | 🔴 Critical | 3-5 giờ | 3-5 lần/ngày |
| PP01-03: Worktree conflict | 🟠 High | 1-2 giờ | 2-3 lần/tuần |
| PP01-04: Không biết agent status | 🟠 High | 2-5 giờ | Hằng ngày |
| PP01-05: Khó share context | 🟡 Medium | 1-2 giờ | Hằng ngày |
| PP01-06: Rate limit không có fallback | 🟡 Medium | 0.5-1 giờ | 2-3 lần/tuần |
| PP01-07: Review code khó | 🟠 High | 2-3 giờ | Mỗi task |

**Tổng thời gian lãng phí ước tính:** **20-38 giờ/tuần** — tương đương 50-95% năng suất tiềm năng

---

## Nguyên nhân gốc rễ

1. **Công cụ không được thiết kế cho multi-agent workflow** — terminal và IDE được thiết kế cho single-process development
2. **Không có abstraction layer cho worktree isolation** — developer phải tự quản lý git worktree thủ công
3. **Thiếu unified status monitoring** — mỗi agent là một black box riêng biệt
4. **Thiếu structured feedback loop** — không có cơ chế review → annotate → send back to agent

---

*Tham chiếu: URD §2.1 (Persona Alex), §3.1 (UR-001 đến UR-005), §3.5 (UR-040 đến UR-043)*
