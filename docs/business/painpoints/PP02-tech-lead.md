# PP02 — Painpoints: Tech Lead (Maya)

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | PP02 |
| **Actor** | Tech Lead / Architect |
| **Đại diện** | Maya — Tech lead team 6 người, review 20+ PR/tuần |
| **Quote** | *"Tôi cần review code AI sinh ra một cách nhanh chóng, có thể comment và gửi feedback ngay."* |
| **Tham chiếu giải pháp** | [SOL02](../solutions/SOL02-tech-lead.md) |

---

## Bối cảnh

Maya là tech lead chịu trách nhiệm chất lượng code của toàn team. Kể từ khi team áp dụng AI agent, lượng code sinh ra tăng 3-5x nhưng quy trình review không thay đổi. Maya phải review nhiều PR hơn với tốc độ nhanh hơn, trong khi vẫn đảm bảo chất lượng và architectural integrity.

---

## Danh sách Painpoints

### PP02-01: Review Loop Giữa GitHub và Terminal Quá Dài

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** 20+ lần/tuần  

**Mô tả:**
Quy trình review AI code hiện tại của Maya: Xem diff trên GitHub → phát hiện vấn đề → mở terminal → gõ feedback cho agent → agent sửa → commit mới → quay lại GitHub để verify. Mỗi vòng loop này mất 10-15 phút và cần 3-5 vòng cho mỗi PR.

**Biểu hiện cụ thể:**
- Phải chuyển qua lại giữa browser (GitHub) và terminal (agent) liên tục
- Mỗi feedback phải mô tả bằng lời vague: "ở function X line Y, sửa như này..."
- Agent hay hiểu nhầm vì feedback thiếu context code cụ thể
- Mất 30-90 phút per PR thay vì 10-15 phút như mong muốn

**Chi phí ước tính:** 10-30 giờ/tuần cho review loop không hiệu quả

---

### PP02-02: Feedback Thiếu Context, Agent Hay Hiểu Nhầm

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** 60-70% số lần feedback  

**Mô tả:**
Khi Maya phát hiện vấn đề trong code, phải mô tả bằng ngôn ngữ tự nhiên: "Ở hàm validateUser, cần check null trước khi access property". Agent thường hiểu nhầm vì không có context về line number chính xác, surrounding code, và intent ban đầu.

**Biểu hiện cụ thể:**
- Phải gõ lại feedback 2-3 lần cho đến khi agent hiểu đúng
- Agent sửa sai chỗ, hoặc sửa đúng nhưng break chỗ khác
- Mô tả bằng lời không thể diễn đạt đầy đủ "sửa chỗ này nhưng giữ logic kia"
- 30% feedback phải gõ lại hoàn toàn vì agent miss context

---

### PP02-03: Không Có Cơ Chế Track "Đã Review / Chưa Review"

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Mỗi PR review  

**Mô tả:**
Khi review PR có 20-30 file thay đổi, Maya không có cách track file nào đã review, file nào chưa. Phải dùng sticky notes hoặc mental note. Thường bỏ sót file hoặc review lại file đã review.

**Biểu hiện cụ thể:**
- Không có "mark as reviewed" cho file level
- Không biết khi nào review session bị interrupt (meeting đột xuất)
- Resume review sau gián đoạn phải bắt đầu lại từ đầu
- 20-30% thời gian review là re-review đã làm

---

### PP02-04: Khó Tạo Worktree / Branch Từ Issue

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** 5-10 lần/tuần  

**Mô tả:**
Khi Maya muốn giao task cho agent từ Linear hoặc GitHub issue, phải làm thủ công: copy issue title → tạo branch → checkout → mở terminal → gõ prompt dài mô tả task. Không có luồng "issue → worktree → agent" liền mạch.

**Biểu hiện cụ thể:**
- Phải copy/paste thủ công từ issue vào agent prompt
- Hay quên include context quan trọng từ issue comments
- Tên branch không consistent với issue naming convention
- Mất 5-10 phút setup thay vì 30 giây

---

### PP02-05: Commit Message Kém Chất Lượng Từ AI

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Mỗi khi agent commit  

**Mô tả:**
Agent tự tạo commit message thường quá generic ("fix bug", "update code") hoặc quá dài không có structure. Maya phải sửa thủ công trước khi merge, tốn thêm thời gian.

**Biểu hiện cụ thể:**
- 80% commit message từ agent cần chỉnh sửa
- Không theo Conventional Commits format của team
- Missing scope, breaking change indicator
- Mất 2-3 phút/commit để sửa message

---

### PP02-06: Không Thể Monitor Team Agent Sessions

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Hằng ngày  

**Mô tả:**
Maya cần biết các agent trong team đang làm gì, đang ở bước nào. Hiện tại phải hỏi từng developer qua Slack. Không có visibility tập trung về progress của tất cả agent sessions trong team.

**Biểu hiện cụ thể:**
- Không biết agent nào đã hoàn thành, agent nào bị stuck
- Phải interrupt developer để hỏi status → distract cả team
- Không phát hiện kịp khi agent đi theo hướng sai
- Không có data để estimate sprint velocity với AI

---

### PP02-07: PR Review Cùng Code Ownership Khó Assign

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** 5-10 lần/tuần  

**Mô tả:**
Khi agent thay đổi nhiều file, Maya cần assign review cho đúng người theo code ownership. Hiện tại GitHub không tự suggest reviewer cho AI-generated changes theo pattern khác với human-written code. Phải manually assign.

**Biểu hiện cụ thể:**
- Không có smart reviewer suggestion cho AI PR
- Hay assign sai người → lại phải reassign
- AI thay đổi code ở nhiều domain khác nhau trong một PR (cross-cutting changes)
- Khó enforce single responsibility principle với AI PRs

---

## Tổng hợp Impact

| Painpoint | Mức độ | Thời gian mất/tuần | Tần suất |
|-----------|--------|-------------------|---------|
| PP02-01: Review loop dài | 🔴 Critical | 10-30 giờ | 20+ PR/tuần |
| PP02-02: Feedback thiếu context | 🔴 Critical | 5-10 giờ | 60% số feedback |
| PP02-03: Không track review progress | 🟠 High | 2-4 giờ | Mỗi PR |
| PP02-04: Setup task thủ công | 🟡 Medium | 1-2 giờ | 5-10 task/tuần |
| PP02-05: Commit message kém | 🟡 Medium | 0.5-1 giờ | Mỗi commit |
| PP02-06: Không monitor team | 🟠 High | 2-3 giờ | Hằng ngày |
| PP02-07: PR review assignment | 🟡 Medium | 0.5-1 giờ | 5-10 lần/tuần |

**Tổng thời gian lãng phí ước tính:** **21-51 giờ/tuần** — Tech lead không thể scale với tốc độ AI code generation

---

## Nguyên nhân gốc rễ

1. **Review tool (GitHub) tách biệt với execution tool (terminal/agent)** — không có unified review + feedback loop
2. **Agent không nhận được structured feedback** — chỉ nhận plain text, thiếu context line/file/intent
3. **Thiếu project management integration** — issue tracker và agent execution không connected
4. **AI code generation vượt quá capacity review của con người** — cần review tool smarter, không chỉ nhanh hơn

---

*Tham chiếu: URD §2.1 (Persona Maya), §3.5 (UR-040 đến UR-043)*
