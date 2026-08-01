# SOL02 — Giải pháp cho Tech Lead (Maya)

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | SOL02 |
| **Actor** | Tech Lead — Maya |
| **Tham chiếu Painpoints** | [PP02](../painpoints/PP02-tech-lead.md) |
| **Tính năng Orca liên quan** | F06, F08, F01, F04 |

---

## Tổng quan giải pháp

Orca biến Maya từ một bottleneck review thành một **AI-augmented tech lead** — review nhanh hơn nhờ inline annotation, feedback chính xác hơn nhờ structured context, và tự động hóa repetitive tasks nhờ integration với GitHub/Linear.

---

## Giải pháp cho từng Painpoint

### SOL02-01: Giải quyết PP02-01 — Review Loop GitHub ↔ Terminal Quá Dài

**Giải pháp: Unified Review Interface trong Orca**

Orca tích hợp diff viewer, PR browser, và agent terminal vào một ứng dụng duy nhất. Maya không cần switch giữa GitHub và terminal.

**Cơ chế hoạt động:**
- Mở PR list từ GitHub panel trong Orca
- Xem diff với syntax highlighting ngay trong Orca (không mở browser)
- Click vào dòng code → thêm comment → gửi về agent trong cùng một flow
- Agent sửa → diff tự động refresh trong cùng view
- Approve và merge PR mà không rời Orca

**Luồng review mới:**
```
Orca: PR list → click PR → xem diff → annotate → send → verify → merge
(Không cần browser, không cần terminal riêng)
```

**Kết quả đo lường được:**
- Review loop: từ 10-15 phút/vòng → 3-5 phút/vòng (3x faster)
- Số tab browser cần mở: từ 3-5 → 0
- Context switch per review session: từ 20+ lần → 2-3 lần

**Tính năng Orca:** [F06 GitHub & Linear Integration](../../features/F06-github-linear-integration.md), [F08 Annotate AI Diffs](../../features/F08-annotate-ai-diffs.md)

---

### SOL02-02: Giải quyết PP02-02 — Feedback Thiếu Context

**Giải pháp: Structured Inline Annotation với File + Line Context**

Thay vì mô tả bằng lời vague, Maya click trực tiếp vào dòng code có vấn đề — Orca tự động attach file path, line number, và original code vào feedback gửi về agent.

**Cơ chế hoạt động:**
- Maya click vào line 42 của file `auth.ts` → textbox mở ngay dòng đó
- Nhập: "Cần check null trước khi access .user.id"
- Orca tạo structured message gửi agent:
  ```
  File: src/auth.ts, Line 42
  Context: `if (user.role === 'admin') {`
  Feedback: Cần check null trước khi access .user.id
  ```
- Agent hiểu chính xác chỗ nào cần sửa và tại sao

**Kết quả đo lường được:**
- Agent hiểu đúng feedback: từ 30-40% → 85-90%
- Số feedback cần gửi lại: từ 60-70% → 10-15%
- Thời gian per feedback cycle: từ 15-30 phút → 5-7 phút

**Tính năng Orca:** [F08 Annotate AI Diffs](../../features/F08-annotate-ai-diffs.md)

---

### SOL02-03: Giải quyết PP02-03 — Không Track Review Progress

**Giải pháp: File-level Review Tracking**

Orca theo dõi tiến trình review — file nào đã xem, file nào chưa, comment nào đã gửi — và persist state qua gián đoạn.

**Cơ chế hoạt động:**
- Diff viewer hiển thị checkmark (✓) cho file đã review
- "Mark as reviewed" button cho từng file
- Session state được lưu — nếu meeting interrupt, resume từ đúng chỗ
- Progress indicator: "5/12 files reviewed"
- Filter: "Show unreviewed files only"

**Kết quả đo lường được:**
- 0 file bị bỏ sót trong review
- Resume review sau gián đoạn: < 30 giây thay vì bắt đầu lại
- Re-review không cần thiết: giảm từ 20-30% → < 5%

**Tính năng Orca:** [F08 Annotate AI Diffs](../../features/F08-annotate-ai-diffs.md), [F06 GitHub & Linear Integration](../../features/F06-github-linear-integration.md)

---

### SOL02-04: Giải quyết PP02-04 — Khó Tạo Worktree Từ Issue

**Giải pháp: One-click Worktree Creation từ GitHub Issue / Linear Task**

Từ issue list trong Orca, Maya tạo worktree và giao cho agent bằng 1-2 click — không cần copy/paste thủ công.

**Cơ chế hoạt động:**
1. Mở Linear panel → chọn issue "Fix login timeout"
2. Click "Create Worktree" → Orca:
   - Tạo branch tự động: `fix/login-timeout-123`
   - Tạo worktree từ branch đó
   - Khởi động agent với issue description làm prompt
   - Link worktree với issue (status tự động → In Progress)
3. Maya monitor progress từ sidebar

**Kết quả đo lường được:**
- Setup time: từ 5-10 phút → < 1 phút
- Context completeness: 100% (issue description tự động inject)
- Branch naming: 100% consistent với convention

**Tính năng Orca:** [F06 GitHub & Linear Integration](../../features/F06-github-linear-integration.md), [F01 Parallel Worktrees](../../features/F01-parallel-worktrees.md)

---

### SOL02-05: Giải quyết PP02-05 — Commit Message Kém Chất Lượng

**Giải pháp: AI-Generated Commit Messages với Convention Enforcement**

Orca generate commit message từ staged changes, tuân thủ Conventional Commits format và project convention.

**Cơ chế hoạt động:**
1. Agent hoàn thành và Maya review xong
2. Click "Commit" → Orca collect staged changes + branch context
3. AI generate message theo format:
   ```
   fix(auth): handle null user before accessing properties
   
   Prevents NullReferenceException when user session expires
   during multi-step authentication flow. Refs #123.
   ```
4. Maya review, chỉnh sửa nếu cần → commit

**Kết quả đo lường được:**
- Commit messages cần chỉnh sửa: từ 80% → < 20%
- Thời gian per commit: từ 3-5 phút → 30-60 giây
- Convention compliance: 100% (enforced by template)

**Tính năng Orca:** [F06 GitHub & Linear Integration](../../features/F06-github-linear-integration.md)

---

### SOL02-06: Giải quyết PP02-06 — Không Monitor Team Agent Sessions

**Giải pháp: Team Worktree Overview + Status Sharing**

Orca cung cấp overview về tất cả worktrees trong project — Maya thấy được tiến trình tất cả agent sessions của team.

**Cơ chế hoạt động:**
- Project view hiển thị tất cả worktrees: owner, agent, status, branch, duration
- Maya thấy worktree nào đang running, done, hay error
- Real-time status update không cần hỏi thủ công qua Slack
- Filter theo: teammate, status, branch, agent type
- Alert khi có worktree ở error state quá lâu

**Kết quả đo lường được:**
- Số lần phải hỏi status qua Slack: từ 10-15 lần/ngày → 0
- Time to detect stuck agent: từ 30-120 phút → < 5 phút
- Team meeting effectiveness: tăng vì Maya có data trước meeting

**Tính năng Orca:** [F01 Parallel Worktrees](../../features/F01-parallel-worktrees.md), [F04 AI Agent Support](../../features/F04-ai-agent-support.md)

---

### SOL02-07: Giải quyết PP02-07 — PR Review Assignment Khó

**Giải pháp: GitHub PR Integration với Reviewer Suggestions**

Orca hiển thị code ownership data và suggest reviewer phù hợp dựa trên file changes.

**Cơ chế hoạt động:**
- Khi tạo PR từ Orca, phân tích files changed → suggest reviewers
- Dựa trên: git blame history, CODEOWNERS file, past review patterns
- Maya xem suggestions và approve/change với 1 click
- Auto-assign không cần manual lookup

**Kết quả đo lường được:**
- PR assignment time: từ 5-10 phút → < 1 phút
- Wrong assignment: từ 20-30% → < 5%
- Review coverage: đảm bảo tất cả domain có expert reviewer

**Tính năng Orca:** [F06 GitHub & Linear Integration](../../features/F06-github-linear-integration.md)

---

## Tổng hợp ROI cho Maya

| Painpoint | Trước Orca | Sau Orca | Tiết kiệm/tuần |
|-----------|-----------|---------|----------------|
| Review loop dài | 10-30 giờ | 3-8 giờ | 7-22 giờ |
| Feedback thiếu context | 5-10 giờ | 1-2 giờ | 4-8 giờ |
| Track review progress | 2-4 giờ | < 30 phút | 1.5-3.5 giờ |
| Setup task cho agent | 1-2 giờ | < 15 phút | 45-105 phút |
| Commit messages | 30-60 phút | < 10 phút | 20-50 phút |
| Monitor team | 2-3 giờ | 15-30 phút | 1.5-2.5 giờ |
| PR assignment | 30-60 phút | < 10 phút | 20-50 phút |
| **TỔNG** | **21-51 giờ** | **5-11 giờ** | **16-40 giờ/tuần** |

**Maya có thể review 3-5x nhiều PR hơn** với cùng thời gian, hoặc giảm overtime đáng kể.

---

*Tham chiếu: PP02 Painpoints, PRD §3.5 (F06), §3.7 (F08), §3.1 (F01)*
