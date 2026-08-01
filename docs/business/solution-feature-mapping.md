# Solution → Feature Mapping

**Tài liệu:** Bảng mapping giữa Giải pháp (theo Actor) và Tính năng Orca  
**Ngày:** 2026-07-21  
**Tham chiếu:** [solutions/](./solutions/), [features/](../features/)

---

## 1. Ma trận tổng quan (Solution × Feature)

Bảng dưới đây thể hiện tính năng nào của Orca giải quyết giải pháp nào. Ký hiệu:

- ● **Primary** — Tính năng chính, trực tiếp giải quyết painpoint
- ○ **Supporting** — Tính năng hỗ trợ, đóng góp gián tiếp
- _(trống)_ — Không liên quan

| Solution / Feature | F01<br>Parallel<br>Worktrees | F02<br>Terminal<br>Splits | F03<br>Mobile<br>Companion | F04<br>AI Agent<br>Support | F05<br>Design<br>Mode | F06<br>GitHub &<br>Linear | F07<br>SSH<br>Worktrees | F08<br>Annotate<br>AI Diffs | F09<br>Orca<br>CLI | F10<br>Quick<br>Open | F11<br>Notif. | F12<br>File<br>Explorer | F13<br>Text<br>Search | F14<br>Auto-<br>mations | F15<br>Computer<br>Use |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| **SOL01-01** Context Switching | ● | ● | | ○ | | | | | | ○ | | | | | |
| **SOL01-02** So sánh song song | ● | ○ | | ● | | | | | | | | | | | |
| **SOL01-03** Worktree isolation | ● | | | | | | | | | | | | | | |
| **SOL01-04** Agent status realtime | | | | ● | | | | | | | ● | | | | |
| **SOL01-05** Share context | | | | | ● | | | | | | | ● | | | |
| **SOL01-06** Rate limit fallback | | | | ● | | | | | | | | | | | |
| **SOL01-07** Review code | | | | | | ○ | | ● | | | | | | | |
| **SOL02-01** Review loop ngắn | | | | | | ● | | ● | | | | | | | |
| **SOL02-02** Feedback có context | | | | | | | | ● | | | | | | | |
| **SOL02-03** Track review progress | | | | | | ● | | ● | | | | | | | |
| **SOL02-04** Worktree từ issue | ● | | | | | ● | | | | | | | | | |
| **SOL02-05** Commit message AI | | | | | | ● | | | | | | | | | |
| **SOL02-06** Monitor team agents | ● | | | ● | | | | | | | ● | | | | |
| **SOL02-07** PR reviewer suggest | | | | | | ● | | | | | | | | | |
| **SOL03-01** SSH auto-reconnect | | | | | | | ● | | | | | | | | |
| **SOL03-02** Auto-deploy relay | | | | | | | ● | | | | | | | | |
| **SOL03-03** Port forwarding auto | | | | | | | ● | | | | | | | | |
| **SOL03-04** File edit remote | | | | | | | ● | | | | | ● | | | |
| **SOL03-05** Agent status khi offline | | | ● | | | | ● | | | | | | | | |
| **SOL03-06** Quản lý nhiều server | | | | | | | ● | | | | | | | | |
| **SOL03-07** Git operations remote | | | | | | | ● | | | | | | | | |
| **SOL04-01** Push notification | | | ● | | | | | | | | ● | | | | |
| **SOL04-02** Remote dispatch | | | ● | | | | | | | | | | | | |
| **SOL04-03** Mobile status monitor | | | ● | | | | | | | | | | | | |
| **SOL04-04** Rate limit alert mobile | | | ● | ● | | | | | | | | | | | |
| **SOL04-05** Session history mobile | | | ● | | | | | | | | | | | | |
| **SOL04-06** Automation scheduling | | | ○ | | | | | | | | | | | ● | |
| **SOL04-07** QR pairing | | | ● | | | | | | | | | | | | |
| **SOL05-01** Capture UI context | | | | | ● | | | | | | | | | | |
| **SOL05-02** Bug report có context | | | | | ● | | | | | | | | | | |
| **SOL05-03** Port test environment | | | | | | | ● | | | | | | | | |
| **SOL05-04** UI test automation | | | | | | | | | | | | | | | ● |
| **SOL06-01** CLI/Headless mode | | | | | | | | | ● | | | | | | |
| **SOL06-02** Observability metrics | | | | | | | | | ● | | | | | | |
| **SOL06-03** Headless Linux | | | | | | | | | ● | | | | | | |
| **SOL06-04** Cleanup policy | | | | | | | | | ● | | | | | ● | |
| **SOL06-05** Access control | | | | ● | | | | | ● | | | | | | |

---

## 2. Feature → Solutions Mapping

Mỗi tính năng giải quyết bao nhiêu painpoints, và của actor nào.

### F01 — Parallel Worktrees

**Giải quyết:** 6 solutions từ 2 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL01-01 | Alex | Context switching tốn kém |
| SOL01-02 | Alex | Không thể so sánh song song |
| SOL01-03 | Alex | Worktree bị lẫn lộn/conflict |
| SOL02-04 | Maya | Khó tạo worktree từ issue |
| SOL02-06 | Maya | Không monitor team agent sessions |

**Mức độ ảnh hưởng:** 🔴 Rất cao — Là tính năng P0 cốt lõi, giải quyết painpoint **Critical** của 2 actor chính

---

### F02 — Terminal Splits

**Giải quyết:** 2 solutions từ 1 actor

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL01-01 | Alex | Context switching (terminal trong unified window) |
| SOL01-02 | Alex | So sánh song song (split view nhiều agent) |

**Mức độ ảnh hưởng:** 🟠 Cao — Foundation cho trải nghiệm unified workspace

---

### F03 — Mobile Companion App

**Giải quyết:** 7 solutions từ 2 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL03-05 | Carlos | Không biết agent status khi mất kết nối |
| SOL04-01 | Sam | Phải ngồi chờ agent xong |
| SOL04-02 | Sam | Không gửi follow-up từ mobile |
| SOL04-03 | Sam | Không biết agent status khi không ở máy |
| SOL04-04 | Sam | Rate limit xảy ra khi không ở máy |
| SOL04-05 | Sam | Không xem history từ mobile |
| SOL04-07 | Sam | Setup pairing phức tạp |

**Mức độ ảnh hưởng:** 🔴 Rất cao — Toàn bộ painpoints của Sam (Mobile User) phụ thuộc vào F03

---

### F04 — AI Agent Support

**Giải quyết:** 5 solutions từ 3 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL01-02 | Alex | So sánh song song (agent management) |
| SOL01-04 | Alex | Không biết agent đang làm gì |
| SOL01-06 | Alex | Rate limit không có fallback |
| SOL02-06 | Maya | Không monitor team agent sessions |
| SOL04-04 | Sam | Rate limit alert mobile |
| SOL06-05 | DevOps | Access control / trust presets |

**Mức độ ảnh hưởng:** 🔴 Rất cao — Core infrastructure cho mọi agent workflow

---

### F05 — Design Mode

**Giải quyết:** 3 solutions từ 2 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL01-05 | Alex | Khó share UI context với agent |
| SOL05-01 | QA | Thiếu công cụ capture UI context |
| SOL05-02 | QA | Giao bug với context thiếu |

**Mức độ ảnh hưởng:** 🟠 Cao — Giải quyết painpoint của cả Developer và QA, đặc biệt hữu ích cho UI-heavy work

---

### F06 — GitHub & Linear Integration

**Giải quyết:** 6 solutions từ 2 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL01-07 | Alex | Review code agent tạo ra không hiệu quả |
| SOL02-01 | Maya | Review loop GitHub ↔ terminal quá dài |
| SOL02-03 | Maya | Không track review progress |
| SOL02-04 | Maya | Khó tạo worktree từ issue |
| SOL02-05 | Maya | Commit message kém chất lượng |
| SOL02-07 | Maya | PR reviewer suggestion |

**Mức độ ảnh hưởng:** 🔴 Rất cao — Maya phụ thuộc gần như hoàn toàn vào F06 để tăng review efficiency

---

### F07 — SSH Worktrees

**Giải quyết:** 8 solutions từ 2 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL03-01 | Carlos | SSH session mất kết nối |
| SOL03-02 | Carlos | Setup môi trường remote phức tạp |
| SOL03-03 | Carlos | Port forwarding thủ công |
| SOL03-04 | Carlos | Không có file editing remote |
| SOL03-05 | Carlos | Không biết agent status khi mất kết nối |
| SOL03-06 | Carlos | Quản lý nhiều server |
| SOL03-07 | Carlos | Git operations chậm |
| SOL05-03 | QA | Môi trường test không nhất quán |

**Mức độ ảnh hưởng:** 🔴 Rất cao — Toàn bộ painpoints của Carlos (Remote Dev) phụ thuộc vào F07

---

### F08 — Annotate AI Diffs

**Giải quyết:** 5 solutions từ 2 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL01-07 | Alex | Review code agent tạo ra không hiệu quả |
| SOL02-01 | Maya | Review loop GitHub ↔ terminal quá dài |
| SOL02-02 | Maya | Feedback thiếu context, agent hiểu nhầm |
| SOL02-03 | Maya | Không track review progress |

**Mức độ ảnh hưởng:** 🟠 Cao — Core feature cho review workflow, giải quyết bottleneck chính của Maya

---

### F09 — Orca CLI

**Giải quyết:** 5 solutions từ 1 actor

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL06-01 | DevOps | Không có CLI/headless mode |
| SOL06-02 | DevOps | Không có observability metrics |
| SOL06-03 | DevOps | Không chạy được headless Linux |
| SOL06-04 | DevOps | Thiếu cleanup policy worktrees |
| SOL06-05 | DevOps | Không có access control |

**Mức độ ảnh hưởng:** 🟠 Cao — Unblocks toàn bộ DevOps use case, từ "không thể dùng" → "fully automated"

---

### F10 — Quick Open

**Giải quyết:** 1 solution từ 1 actor

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL01-01 | Alex | Context switching (navigation) |

**Mức độ ảnh hưởng:** 🟡 Trung bình — Utility feature, hỗ trợ navigation

---

### F11 — Notifications & Unread State

**Giải quyết:** 4 solutions từ 3 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL01-04 | Alex | Không biết agent đang làm gì |
| SOL02-06 | Maya | Không monitor team agent sessions |
| SOL04-01 | Sam | Phải ngồi chờ agent xong |

**Mức độ ảnh hưởng:** 🟠 Cao — Supporting feature cho F03 (mobile) và F04 (agent monitoring)

---

### F12 — File Explorer & Editor

**Giải quyết:** 3 solutions từ 2 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL01-05 | Alex | Khó share context với agent (drag-drop) |
| SOL03-04 | Carlos | Không có file editing remote |

**Mức độ ảnh hưởng:** 🟡 Trung bình — Foundation feature, quan trọng nhưng expected

---

### F13 — Text Search

**Giải quyết:** 0 solutions trực tiếp

**Mức độ ảnh hưởng:** 🟢 Thấp — General utility, không liên trực tiếp đến mapped painpoints

---

### F14 — Automations

**Giải quyết:** 2 solutions từ 2 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL04-06 | Sam | Automation workflows không chạy tự động |
| SOL06-04 | DevOps | Thiếu cleanup policy worktrees |

**Mức độ ảnh hưởng:** 🟡 Trung bình — P2 feature, quan trọng cho power users

---

### F15 — Computer Use

**Giải quyết:** 2 solutions từ 2 actors

| Solution | Actor | Painpoint |
|---------|-------|----------|
| SOL05-04 | QA | Không có UI test automation |
| SOL06-01 | DevOps | CLI automation (UI interaction) |

**Mức độ ảnh hưởng:** 🟡 Trung bình — P2 feature, high value cho QA workflow

---

### F16–F21 — Features hỗ trợ khác

| Feature | Painpoints giải quyết | Ghi chú |
|---------|----------------------|---------|
| F16 Rich Repo Previews | 0 direct | General UX improvement |
| F17 Memory / AI Vault | 0 direct | Emerging use case |
| F18 Ephemeral VM | 0 direct | Advanced/future use case |
| F19 Localization | 0 direct | Adoption enabler |
| F20 Speech Input | 0 direct | Accessibility / Sam use case (future) |
| F21 Auto Update | Implicit tất cả | Reliability — mọi actor hưởng lợi |

---

## 3. Solution → Feature Coverage

Mỗi solution phụ thuộc vào bao nhiêu features và đây là những features quan trọng nhất.

| Solution | Actor | Features chính | Features phụ | Total |
|---------|-------|---------------|-------------|-------|
| SOL01-01 Context Switching | Alex | F01, F02 | F04, F10 | 4 |
| SOL01-02 Song song | Alex | F01, F04 | F02 | 3 |
| SOL01-03 Isolation | Alex | F01 | — | 1 |
| SOL01-04 Agent status | Alex | F04, F11 | — | 2 |
| SOL01-05 Share context | Alex | F05, F12 | — | 2 |
| SOL01-06 Rate limit | Alex | F04 | — | 1 |
| SOL01-07 Review code | Alex | F08 | F06 | 2 |
| SOL02-01 Review loop | Maya | F06, F08 | — | 2 |
| SOL02-02 Feedback context | Maya | F08 | — | 1 |
| SOL02-03 Review tracking | Maya | F08, F06 | — | 2 |
| SOL02-04 Issue → Worktree | Maya | F01, F06 | — | 2 |
| SOL02-05 Commit message | Maya | F06 | — | 1 |
| SOL02-06 Monitor team | Maya | F01, F04, F11 | — | 3 |
| SOL02-07 PR assignment | Maya | F06 | — | 1 |
| SOL03-01 SSH reconnect | Carlos | F07 | — | 1 |
| SOL03-02 Relay deploy | Carlos | F07 | — | 1 |
| SOL03-03 Port forwarding | Carlos | F07 | — | 1 |
| SOL03-04 File edit remote | Carlos | F07, F12 | — | 2 |
| SOL03-05 Status offline | Carlos | F07, F03 | — | 2 |
| SOL03-06 Multi-server | Carlos | F07 | — | 1 |
| SOL03-07 Git remote | Carlos | F07 | — | 1 |
| SOL04-01 Push notification | Sam | F03, F11 | — | 2 |
| SOL04-02 Remote dispatch | Sam | F03 | — | 1 |
| SOL04-03 Mobile status | Sam | F03 | — | 1 |
| SOL04-04 Rate limit mobile | Sam | F03, F04 | — | 2 |
| SOL04-05 Session history | Sam | F03 | — | 1 |
| SOL04-06 Automation | Sam | F14 | F03 | 2 |
| SOL04-07 QR pairing | Sam | F03 | — | 1 |
| SOL05-01 Capture UI | QA | F05 | — | 1 |
| SOL05-02 Bug report | QA | F05 | — | 1 |
| SOL05-03 Port test env | QA | F07 | — | 1 |
| SOL05-04 UI test auto | QA | F15 | — | 1 |
| SOL06-01 CLI/Headless | DevOps | F09 | — | 1 |
| SOL06-02 Observability | DevOps | F09 | — | 1 |
| SOL06-03 Headless Linux | DevOps | F09 | — | 1 |
| SOL06-04 Cleanup policy | DevOps | F09, F14 | — | 2 |
| SOL06-05 Access control | DevOps | F04, F09 | — | 2 |

---

## 4. Xếp hạng Feature theo Tầm quan trọng

Dựa trên số lượng solutions mà feature giải quyết và mức độ nghiêm trọng của painpoints.

| Hạng | Feature | Solutions giải quyết | Actor ảnh hưởng | Priority |
|------|---------|---------------------|----------------|---------|
| 1 | **F07 SSH Worktrees** | 8 solutions | Carlos (100%), QA | P1 |
| 2 | **F03 Mobile Companion** | 7 solutions | Sam (100%), Carlos | P0 |
| 3 | **F01 Parallel Worktrees** | 6 solutions | Alex (43%), Maya | P0 |
| 4 | **F06 GitHub & Linear** | 6 solutions | Maya (86%), Alex | P1 |
| 5 | **F04 AI Agent Support** | 6 solutions | Alex, Maya, Sam, DevOps | P0 |
| 6 | **F08 Annotate AI Diffs** | 4 solutions | Alex, Maya | P1 |
| 7 | **F09 Orca CLI** | 5 solutions | DevOps (100%) | P1 |
| 8 | **F11 Notifications** | 3 solutions | Alex, Maya, Sam | P1 |
| 9 | **F05 Design Mode** | 3 solutions | Alex, QA | P1 |
| 10 | **F12 File Explorer** | 2 solutions | Alex, Carlos | P1 |
| 11 | **F14 Automations** | 2 solutions | Sam, DevOps | P2 |
| 12 | **F15 Computer Use** | 2 solutions | QA, DevOps | P2 |
| 13 | **F02 Terminal Splits** | 2 solutions | Alex | P0 |
| 14 | **F10 Quick Open** | 1 solution | Alex | P1 |
| 15 | **F13–F21** | 0 solutions | General | P2–P3 |

---

## 5. Actor → Feature Dependency Map

Tính năng nào là thiết yếu (critical path) cho mỗi actor.

```
Alex (Senior Dev)
├── [CRITICAL] F01 Parallel Worktrees   ← Không có = không thể multi-agent
├── [CRITICAL] F04 AI Agent Support     ← Core infrastructure
├── [HIGH]     F08 Annotate AI Diffs    ← Review workflow
├── [HIGH]     F02 Terminal Splits      ← Unified workspace
├── [MEDIUM]   F05 Design Mode          ← UI context sharing
├── [MEDIUM]   F12 File Explorer        ← File management
└── [MEDIUM]   F11 Notifications        ← Status awareness

Maya (Tech Lead)
├── [CRITICAL] F06 GitHub & Linear      ← Không có = phải dùng GitHub browser
├── [CRITICAL] F08 Annotate AI Diffs    ← Core review tool
├── [HIGH]     F01 Parallel Worktrees   ← Team monitoring
├── [HIGH]     F04 AI Agent Support     ← Agent management
└── [MEDIUM]   F11 Notifications        ← Team visibility

Carlos (Remote Dev)
└── [CRITICAL] F07 SSH Worktrees        ← Tất cả 7 painpoints đều qua F07
    (F12, F03 là supporting)

Sam (Mobile-First)
└── [CRITICAL] F03 Mobile Companion     ← Tất cả 7 painpoints của Sam qua F03
    ├── [HIGH]   F04 AI Agent Support   ← Rate limit management
    └── [MEDIUM] F14 Automations        ← Recurring tasks

QA Engineer
├── [HIGH]     F05 Design Mode          ← Capture + bug report
├── [HIGH]     F07 SSH Worktrees        ← Port proxy (supporting)
└── [MEDIUM]   F15 Computer Use         ← UI test automation

DevOps Engineer
└── [CRITICAL] F09 Orca CLI             ← Tất cả 5 painpoints đều qua F09
    └── [MEDIUM] F14 Automations        ← Cleanup + scheduling
```

---

## 6. Gaps — Solutions Chưa Có Feature Tương ứng

Các painpoints được đề cập trong solutions nhưng chưa có feature spec hoàn chỉnh.

| Gap | Solution | Mô tả | Đề xuất |
|----|---------|-------|---------|
| Team visibility dashboard | SOL02-06 | Maya cần xem status team agents | Mở rộng F01 + F04 với team view |
| Session share/export | SOL04-05 | Sam xem history từ mobile | Mở rộng F03 với session viewer |
| Mobile account switch | SOL04-04 | Remote switch account từ mobile | Mở rộng F03 + F04 |
| Regression test integration | SOL05-04 | QA test chạy sau mỗi agent change | Tích hợp F14 + F15 |
| Agent JSON output | SOL06-02 | DevOps cần structured metrics | Mở rộng F09 với metrics endpoint |

---

*Tài liệu được tạo dựa trên phân tích [solutions/](./solutions/) và [features/](../features/)*
