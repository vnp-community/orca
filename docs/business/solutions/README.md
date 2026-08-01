# Solutions — Giải pháp theo Actor

Thư mục này chứa **giải pháp Orca** tương ứng với từng painpoint của mỗi actor. Mỗi giải pháp được trình bày theo cấu trúc: cơ chế hoạt động → kết quả đo lường được → tính năng Orca liên quan.

---

## Mapping: Actor → Painpoints → Solutions

| Actor | Painpoints | Solutions | Tính năng Orca chính |
|-------|-----------|-----------|----------------------|
| [Alex — Senior Dev](./SOL01-senior-developer.md) | [PP01](../painpoints/PP01-senior-developer.md) | 7 giải pháp | F01, F02, F04, F08, F10, F12 |
| [Maya — Tech Lead](./SOL02-tech-lead.md) | [PP02](../painpoints/PP02-tech-lead.md) | 7 giải pháp | F06, F08, F01, F04 |
| [Carlos — Remote Dev](./SOL03-remote-developer.md) | [PP03](../painpoints/PP03-remote-developer.md) | 7 giải pháp | F07, F02, F12, F03 |
| [Sam — Mobile User](./SOL04-mobile-first-user.md) | [PP04](../painpoints/PP04-mobile-first-user.md) | 7 giải pháp | F03, F04, F14 |
| [QA Engineer](./SOL05-qa-engineer.md) | [PP05](../painpoints/PP05-qa-engineer.md) | 4 giải pháp | F05, F07, F15 |
| [DevOps Engineer](./SOL06-devops-engineer.md) | [PP06](../painpoints/PP06-devops-engineer.md) | 5 giải pháp | F09, F14 |

---

## Tổng hợp ROI theo Actor

| Actor | Vấn đề chính | Tiết kiệm ước tính | Năng suất tăng |
|-------|-------------|-------------------|----------------|
| **Alex** (Senior Dev) | 20-38 giờ/tuần lãng phí | 16-34 giờ/tuần | **4-8x** |
| **Maya** (Tech Lead) | 21-51 giờ/tuần lãng phí | 16-40 giờ/tuần | **3-5x review capacity** |
| **Carlos** (Remote Dev) | 1-3 giờ/ngày lãng phí | 45-165 phút/ngày | **20-40% năng suất tăng** |
| **Sam** (Mobile User) | 4-8 giờ/ngày bị block | 3.5-7.5 giờ/ngày | **Async workflow** |
| **QA** | 3-6 giờ/ngày thủ công | 80-85% thời gian | **5-7x faster testing** |
| **DevOps** | Không thể automate | Fully automated | **Unblocked** |

---

## Ma trận: Painpoint → Tính năng Orca Giải quyết

| Painpoint | Tính năng Orca | Giải pháp cốt lõi |
|-----------|---------------|-------------------|
| Context switching đắt | F01, F02 | Unified workspace với sidebar navigation |
| Không so sánh song song | F01 | Fan-out prompt → parallel worktrees |
| Worktree conflict | F01 | Automatic isolation + safety guards |
| Không biết agent status | F04, F11 | Real-time status dashboard + notifications |
| Khó share context | F05, F12 | Drag-drop + Design Mode |
| Rate limit không có fallback | F04 | Account switcher + usage tracking |
| Review code không hiệu quả | F08 | Inline annotation với structured feedback |
| Review loop GitHub ↔ terminal | F06, F08 | Unified review interface |
| Feedback thiếu context | F08 | File + line + code context tự động |
| SSH drop liên tục | F07 | Transparent auto-reconnect |
| Setup remote phức tạp | F07 | Auto-deploy relay binary |
| Port forwarding thủ công | F07 | Auto port detection và forwarding |
| File editing remote | F07, F12 | Monaco editor với remote file access |
| Phải ngồi chờ agent | F03 | Push notifications |
| Không gửi follow-up từ mobile | F03 | Remote dispatch |
| Automation thủ công | F14 | Cron + event triggers |
| Không có CLI/headless | F09 | `orca serve` headless mode |
| Không có observability | F09 | JSON output + telemetry events |
| Capture UI context | F05 | Design Mode 1-click capture |
| Giao bug với context đủ | F05 | Structured bug report tự động |

---

## Cấu trúc mỗi Solution file

- **Header table**: Actor, references, tính năng liên quan
- **Tổng quan**: giải pháp tổng thể Orca mang lại
- **Chi tiết từng giải pháp**: tương ứng 1-1 với painpoints
  - Cơ chế hoạt động (step-by-step)
  - Kết quả đo lường được (before/after)
  - Tính năng Orca (link tới feature file)
- **ROI table**: tổng hợp tiết kiệm trước/sau

---

*Giải pháp dựa trên tính năng Orca v1.4.x — tham chiếu PRD.md, SRS.md và codebase*
