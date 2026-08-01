# SOL05 — Giải pháp cho QA Engineer

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | SOL05 |
| **Actor** | QA Engineer |
| **Tham chiếu Painpoints** | [PP05](../painpoints/PP05-qa-engineer.md) |
| **Tính năng Orca liên quan** | F05, F07, F15 |

---

## Tổng quan giải pháp

Orca cung cấp cho QA hai công cụ mạnh: **Design Mode** để capture UI context chính xác và gửi bug report có cấu trúc về agent, và **Localhost Proxy** để test nhiều worktrees song song mà không bị nhầm lẫn.

---

## Giải pháp cho từng Painpoint

### SOL05-01: Giải quyết PP05-01 — Thiếu Công Cụ Capture UI Context

**Giải pháp: Design Mode — 1-Click Element Capture**

Design Mode cho phép QA click vào bất kỳ element nào trong browser tích hợp → tự động capture HTML, CSS, và screenshot.

**Cơ chế hoạt động:**
1. QA mở browser trong Orca → navigate tới app cần test
2. Click "Design Mode" → cursor chuyển sang inspect mode
3. Click vào button lỗi → Orca capture:
   - Outer HTML với ancestors
   - Computed CSS styles
   - Screenshot crop quanh element
4. Data tự động inject vào agent prompt với context đầy đủ
5. QA chỉ cần thêm: "Button này không responsive trên mobile viewport"

**Kết quả đo lường được:**
- Capture time: từ 3-5 phút (thủ công DevTools) → < 30 giây
- Context completeness: từ 40-60% → 95%+
- Agent fix đúng lần đầu: từ 60% → 85-90%

**Tính năng Orca:** [F05 Design Mode](../../features/F05-design-mode.md)

---

### SOL05-02: Giải quyết PP05-02 — Giao Bug Với Context Thiếu

**Giải pháp: Design Mode + Structured Bug Report**

Design Mode tạo structured bug report tự động — element selector, visual context, và reproduction steps.

**Cơ chế hoạt động:**
1. QA click vào element lỗi trong Design Mode
2. Capture HTML + CSS + screenshot
3. Orca tạo structured prompt:
   ```
   Element: button.submit-btn [line 245 in LoginForm.tsx]
   CSS: display:block, width:100%, background:#1a73e8
   Screenshot: [attached]
   
   Bug: Button không hiển thị trên viewport < 375px
   Expected: Button vẫn hiển thị và clickable
   ```
4. Agent nhận đầy đủ context → fix đúng issue

**Kết quả đo lường được:**
- Agent fix đúng lần đầu: từ 40-60% → 85%+
- Số vòng feedback: từ 3-5 → 1-2
- Thời gian mô tả bug: từ 10-15 phút → < 2 phút

**Tính năng Orca:** [F05 Design Mode](../../features/F05-design-mode.md)

---

### SOL05-03: Giải quyết PP05-03 — Môi Trường Test Không Nhất Quán

**Giải pháp: Localhost Proxy với Label-based Routing**

Orca proxy manager tự động assign unique local ports cho từng worktree và hiển thị rõ ràng mapping.

**Cơ chế hoạt động:**
- Worktree A (fix-login): remote:3000 → local:3001 → label "fix-login"
- Worktree B (add-tests): remote:3000 → local:3002 → label "add-tests"
- QA thấy rõ: "localhost:3001 = Worktree A [fix-login]"
- Browser tab có thể mở cả 2 URL cùng lúc, không bị nhầm
- Orca dashboard hiển thị port mapping table

**Kết quả đo lường được:**
- Nhầm lẫn test sai worktree: từ thường xuyên → 0
- Setup port mapping: từ manual tracking → tự động
- Parallel testing: QA có thể test 2-3 worktrees cùng lúc

**Tính năng Orca:** [F07 SSH Worktrees](../../features/F07-ssh-worktrees.md) (Localhost Proxy)

---

### SOL05-04: Giải quyết PP05-04 — Không Có UI Test Automation

**Giải pháp: Computer Use — Agent Thực Hiện UI Test**

Computer Use cho phép agent điều khiển browser/desktop để chạy regression test tự động.

**Cơ chế hoạt động:**
1. QA viết test scenario bằng ngôn ngữ tự nhiên cho agent
2. Agent dùng computer use để:
   - Navigate tới URL
   - Click elements
   - Fill forms
   - Verify visual output qua screenshot
3. Agent báo cáo kết quả: pass/fail + screenshot evidence

**Ví dụ automation:**
```
"Test login flow:
1. Navigate to /login
2. Fill email: test@example.com, password: Test123!
3. Click Submit
4. Verify redirect to /dashboard
5. Verify user name shown in header"
```

**Kết quả đo lường được:**
- Regression test time: từ 2-4 giờ manual → agent chạy tự động < 30 phút
- Test frequency: từ 1 lần/sprint → sau mỗi agent change

**Tính năng Orca:** [F15 Computer Use](../../features/F15-computer-use.md)

---

## Tổng hợp ROI cho QA

| Painpoint | Trước Orca | Sau Orca | Tiết kiệm |
|-----------|-----------|---------|-----------|
| Capture UI context | 3-5 phút/element | < 30 giây | 80% thời gian |
| Mô tả bug cho agent | 10-15 phút | < 2 phút | 85% thời gian |
| Port confusion | 10-20 phút/session | 0 | 100% |
| Regression testing | 2-4 giờ manual | < 30 phút | 85% |

---

*Tham chiếu: PP05 Painpoints, PRD §3.4 (F05 Design Mode), §3.10 (F15 Computer Use)*
