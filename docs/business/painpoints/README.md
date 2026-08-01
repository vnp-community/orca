# Painpoints — Phân tích theo Actor

Thư mục này chứa phân tích **painpoints** (vấn đề, nỗi đau) của từng actor khi làm việc với AI agent trong development workflow — trước khi có Orca.

---

## Actors và Painpoints

| File | Actor | Vai trò | Số Painpoints | Mức độ nghiêm trọng cao nhất |
|------|-------|---------|--------------|------------------------------|
| [PP01](./PP01-senior-developer.md) | **Alex** — Senior Full-Stack Developer | Developer chạy nhiều agent song song | 7 | 🔴 Critical (×2) |
| [PP02](./PP02-tech-lead.md) | **Maya** — Tech Lead | Review code AI và quản lý team | 7 | 🔴 Critical (×2) |
| [PP03](./PP03-remote-developer.md) | **Carlos** — Remote Developer | Chạy agent trên remote server qua SSH | 7 | 🔴 Critical (×2) |
| [PP04](./PP04-mobile-first-user.md) | **Sam** — Mobile-First Power User | Monitor và control agent từ mobile | 7 | 🔴 Critical (×2) |
| [PP05](./PP05-qa-engineer.md) | **QA Engineer** | Kiểm thử UI code từ agent | 4 | 🟠 High (×2) |
| [PP06](./PP06-devops-engineer.md) | **DevOps Engineer** | Tích hợp agent vào CI/CD | 5 | 🔴 Critical (×2) |

**Tổng: 37 painpoints** từ 6 actors

---

## Tổng hợp Painpoints Quan Trọng Nhất

### 🔴 Critical Painpoints (Blocker)

| ID | Painpoint | Actor | Impact |
|----|-----------|-------|--------|
| PP01-01 | Context switching tốn kém khi multi-agent | Alex | 10-20 giờ/tuần lãng phí |
| PP01-02 | Không thể so sánh agent results song song | Alex | Chọn solution suboptimal 40% |
| PP02-01 | Review loop GitHub ↔ terminal quá dài | Maya | 10-30 giờ/tuần |
| PP02-02 | Feedback thiếu context, agent hiểu nhầm | Maya | 60-70% feedback cần gửi lại |
| PP03-01 | SSH session mất kết nối liên tục | Carlos | 50 phút/ngày gián đoạn |
| PP03-02 | Setup môi trường remote phức tạp | Carlos | 2-4 giờ/lần setup |
| PP04-01 | Phải ngồi chờ agent xong việc | Sam | 2-4 giờ/ngày "bị giam" |
| PP04-02 | Không gửi follow-up khi xa máy tính | Sam | 1-3 giờ workflow delay/ngày |
| PP06-01 | Không có CLI/headless mode | DevOps | Không thể automate — blocker |
| PP06-03 | Không chạy được trên headless Linux | DevOps | Không deploy được server — blocker |

### 🟠 High Painpoints

| ID | Painpoint | Actor |
|----|-----------|-------|
| PP01-03 | Worktree bị lẫn lộn, conflict | Alex |
| PP01-04 | Không biết agent đang làm gì | Alex |
| PP01-07 | Review code agent tạo ra không hiệu quả | Alex |
| PP02-03 | Không có cơ chế track review progress | Maya |
| PP02-06 | Không monitor team agent sessions | Maya |
| PP03-03 | Port forwarding thủ công cồng kềnh | Carlos |
| PP03-04 | Không có file editing trực tiếp remote | Carlos |
| PP03-05 | Không biết agent status khi mất kết nối | Carlos |
| PP04-03 | Không biết agent status khi không ở máy | Sam |
| PP04-04 | Rate limit xảy ra khi Sam không ở máy | Sam |
| PP05-01 | Thiếu công cụ capture UI context | QA |
| PP05-02 | Giao bug cho agent thiếu context | QA |
| PP06-02 | Không có observability cho agent runs | DevOps |

---

## Phân tích nguyên nhân gốc rễ chung

### 1. Thiếu Unified Orchestration Layer
Tất cả actors đều bị ảnh hưởng bởi việc phải dùng nhiều công cụ riêng biệt (terminal, IDE, browser, GitHub, task tracker). Không có một layer thống nhất để orchestrate toàn bộ AI agent workflow.

### 2. Không Có Async / Decoupled Agent Interaction
Sam và Carlos đều bị ảnh hưởng bởi việc phải "hiện diện" khi agent làm việc. Thiếu mechanism để nhận kết quả và gửi input asynchronously.

### 3. Feedback Loop Agent Không Có Context Structure
Alex và Maya đều gặp vấn đề với việc gửi feedback về agent. Feedback thuần text không mang đủ context (file, line, element, intent) → agent hiểu nhầm thường xuyên.

### 4. SSH Remote Development Không Được Thiết Kế Cho Developer UX
Carlos gặp toàn bộ vấn đề của "SSH là protocol, không phải UX" — không có reconnect, không có state persistence, port forwarding thủ công, file editing cồng kềnh.

### 5. Desktop-Only Application Không Scale
Sam và DevOps đều bị block bởi việc Orca là desktop app — Sam không thể dùng mobile, DevOps không thể dùng server.

---

## Liên kết tới Giải pháp

| Painpoints File | Solutions File |
|----------------|----------------|
| [PP01-senior-developer.md](./PP01-senior-developer.md) | [SOL01-senior-developer.md](../solutions/SOL01-senior-developer.md) |
| [PP02-tech-lead.md](./PP02-tech-lead.md) | [SOL02-tech-lead.md](../solutions/SOL02-tech-lead.md) |
| [PP03-remote-developer.md](./PP03-remote-developer.md) | [SOL03-remote-developer.md](../solutions/SOL03-remote-developer.md) |
| [PP04-mobile-first-user.md](./PP04-mobile-first-user.md) | [SOL04-mobile-first-user.md](../solutions/SOL04-mobile-first-user.md) |
| [PP05-qa-engineer.md](./PP05-qa-engineer.md) | [SOL05-qa-engineer.md](../solutions/SOL05-qa-engineer.md) |
| [PP06-devops-engineer.md](./PP06-devops-engineer.md) | [SOL06-devops-engineer.md](../solutions/SOL06-devops-engineer.md) |

---

*Phân tích dựa trên URD.md §2 (User Personas), §3 (User Requirements) và PRD.md §2 (Đối tượng người dùng)*
