# 04 — Code Health, Test Coverage & `AGENTS.md` Compliance

Quy mô đối chiếu: **825,455 dòng** source (3,865 file) + **410,899 dòng** test (1,871 file) trong `frontend/src`.

---

## 1. `max-lines` — vi phạm chính sách `AGENTS.md`

**Mức độ: 🔴 Critical (policy)**

`AGENTS.md:15` quy định nguyên văn:

> "Never add a `max-lines` disable (`eslint-disable max-lines`, `oxlint-disable max-lines`, or line-specific variants), and never add a per-file `max-lines` bump in `mobile/.oxlintrc.json`."

Kết quả grep toàn `frontend/src`:

| Loại disable | Số lượng |
|---|---|
| Tổng số disable comment (`eslint-disable*` + `oxlint-disable*`) | **438** |
| Trong đó `max-lines` | **240** (217 eslint + 22 oxlint + 1 kết hợp) |

**240/3,865 file (~6.2%) chứa đúng disable mà `AGENTS.md` cấm tuyệt đối.** Đây là phát hiện dễ trích dẫn nhất, rõ ràng nhất của toàn bộ audit — không cần suy diễn, chỉ cần đối chiếu 1 câu quy định với kết quả grep.

Ví dụ cụ thể:
- [App.tsx:1](../../frontend/src/renderer/src/App.tsx#L1) — `/* eslint-disable max-lines */`, **không có comment giải thích**.
- [Terminal.tsx:1](../../frontend/src/renderer/src/components/Terminal.tsx#L1) — tương tự, không giải thích.
- `LinearIssueWorkspace.tsx`, `GitHubItemDialog.tsx`, `GitLabItemDialog.tsx`, `JiraIssueWorkspace.tsx`, `LinearItemDrawer.tsx`, `PullRequestPage.tsx`, `UpdateCard.tsx`, `NewWorkspaceComposerCard.tsx`, `web-runtime-client.ts`, `web-preload-api.ts` — mỗi file có comment `-- Why:` giải thích lý do (thường là "1 file/domain quá lớn để tách hợp lý ngay") — nhưng `AGENTS.md` không có ngoại lệ "có lý do thì được".

**Bối cảnh cần cân nhắc khi quyết định hành động:** rule này rất có thể được thêm **sau khi** phần lớn 240 file này đã tồn tại (một baseline ratchet, xem `config/max-lines-baseline.txt`/`config/scripts/check-max-lines-ratchet.mjs` từng thấy trong lịch sử repo) — nghĩa là đây có thể là nợ kỹ thuật lịch sử được chính sách mới "đóng băng" lại (không tăng thêm), chứ không phải 240 vi phạm mới phát sinh. **Cần xác nhận việc này với người giữ chính sách** trước khi coi đây là 240 việc cần sửa ngay — nhưng dù là baseline cũ, con số + vị trí cụ thể vẫn đáng đưa vào audit vì đây là gap giữa "quy định đang có hiệu lực" và "trạng thái thật của code".

**Gợi ý:**
1. Xác nhận cơ chế ratchet hiện tại (`check-max-lines-ratchet.mjs` nếu còn tồn tại) có đang chặn *tăng thêm* số file vi phạm hay không.
2. Nếu có, audit này coi đây là nợ kỹ thuật đã biết, ưu tiên thấp hơn 2 lỗ hổng bảo mật đã nêu ở [01](./01-security-conformance.md).
3. Nếu không có cơ chế ratchet nào đang chạy, đây là rủi ro đang tăng dần — nên bổ sung ngay.

## 2. Các rule bị disable khác (ngoài `max-lines`)

| Rule | Số lần | Ghi chú |
|---|---|---|
| `react-hooks/exhaustive-deps` | 55 | Rủi ro cao nhất trong nhóm này — dễ gây stale-closure bug |
| `react-doctor/no-adjust-state-on-prop-change` | 36 | |
| `@typescript-eslint/no-explicit-any` | 30 | |
| `@typescript-eslint/consistent-type-imports` | 26 | |
| `no-control-regex` | 23 | |
| `react-hooks/rules-of-hooks` | 14 | |
| Còn lại (`consistent-type-definitions`, `no-initialize-state`, `no-derived-state-effect`, `no-console`, ...) | ≤4 mỗi rule | |

`react-hooks/exhaustive-deps` (55 lần) là rule đáng lưu ý nhất trong nhóm — không nằm trong "never" list của `AGENTS.md` nên không phải policy violation, nhưng đáng để rà lại định kỳ vì đây là loại lỗi khó phát hiện qua test.

## 3. Test coverage gap ở 5 module provider mới

**Mức độ: 🟢 Low (nhưng đáng làm sớm)**

Trong 8 module được phát hiện thiếu doc ở phiên audit trước (nay đã bổ sung vào [F04](../../docs/features/F04-ai-agent-support.md)/[F01](../../docs/features/F01-parallel-worktrees.md)/[F41](../../docs/features/F41-desktop-pet-companion.md)/[F42](../../docs/features/F42-contextual-onboarding-tours.md)):

| Module | File impl | File test |
|---|---|---|
| `src/main/kimi/` | 2 | **0** |
| `src/main/minimax/` | 1 | **0** |
| `src/main/openclaude/` | 1 | **0** |
| `src/main/command-code/` | 2 | **0** |
| `src/main/droid/` | 1 | **0** |
| `renderer/src/components/sparse/` | 2 | **0** |
| `renderer/src/components/pet/` | 7 | 8 ✅ |
| `renderer/src/components/contextual-tours/` | 13 | 9 ✅ |

6/8 module — toàn bộ 7 file `hook-service`/cookie-store/TOML-config trong `main/{kimi,minimax,openclaude,command-code,droid}` cộng 2 file trong `sparse/` — **không có test nào**, dù đều là logic đang chạy thật (install/remove hook, mã hoá cookie, validate path). `pet/` và `contextual-tours/` có coverage tốt, là ngoại lệ tích cực.

**Gợi ý:** ưu tiên viết test cho `getStatus()`/`install()`/`remove()` của 5 `HookService` (đối xứng với test đã có cho `claude/hook-service.test.ts`, `codex/hook-service.test.ts` nếu tồn tại — dùng làm khuôn mẫu) trước khi các provider này được thêm tính năng mới.

## 4. Test bị skip — ✅ Không đáng lo

Chỉ **1** test bị skip thật sự trong 1,871 file test: [components/code-review/annotation-panel.test.tsx:62](../../frontend/src/renderer/src/components/code-review/annotation-panel.test.tsx#L62) — có comment giải thích rõ (Vite không resolve được `date-fns` qua mock-interception plugin), và ghi chú rằng logic đã được review thủ công thay thế. 16 `it.skipIf(...)` khác đều là platform-gating hợp lệ (Windows-only test, WSL-only test), không phải test bị bỏ quên.

## 5. TODO/FIXME — ✅ Thấp

**11 TODO, 0 FIXME** trong ~825k dòng — rất thấp so với quy mô. Không có TODO nào mang giọng điệu "hack tạm/nhớ xoá trước khi release". Phần lớn là ghi chú scope tương lai có ticket reference (`#1693`).

## 6. `any` type usage — ✅ Thấp

**108 lần** (`: any` + `as any`) trên ~825k dòng production — mật độ ~1/7,600 dòng, tín hiệu tốt cho kỷ luật TypeScript strict.
