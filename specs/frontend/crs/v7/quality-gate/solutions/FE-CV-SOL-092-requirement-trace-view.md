# FE-CV-SOL-092-requirement-trace-view: Lens "Yêu cầu" (truy vết yêu cầu ↔ thay đổi ↔ test)

> Trạng thái (2026-10-07): 7/7 task DONE, 0 PARTIAL, 0 BLOCKED, 0 TODO. Xem mục "Ghi chú triển khai" của từng task; code thật lệch spec ở các điểm đã ghi.

**CR:** [CR-CV-092](../../../../../../docs/crs/v7/quality-gate/CR-CV-092-requirement-traceability.md) (phần frontend; dữ liệu do `BE-CV-SOL-092-requirement-trace`). Priority P2.
**Area:** frontend (`components/review-map/requirements/`, `hooks`)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) 2.3, 2.4 (`quality.trace.confirm|link` ≤ 8 KiB), 3.2 (`quality.trace`, `quality.trace.confirm`, `quality.trace.link`), 4.6 (`ReviewNoteAnchor.lens` có `'requirements'`), 4.7 (`RequirementTrace`, `Requirement`, `RequirementEvidence`), 5 (push); PQ-01, PQ-04, PQ-13, PQ-27, mục 7.1, 8.3.
**TDD tham chiếu:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md), [v5/15-task-graph-ui](../../../../tdd/v5/15-task-graph-ui.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `shared/task-types.ts` (`OrcaTask`: `id`, `projectId?`, `title`, `taskNumber?`…), `hooks/useTasks.ts` (`useTasks(projectId)`, tải bằng `task.list`, lọc theo `projectId` ở client), `components/ui/` (có `command.tsx`, `popover.tsx`, `badge.tsx`, `toggle-group.tsx`; **không có** picker task nào: grep `TaskPicker|TaskSelect|LinkTask` chỉ trúng `TaskPage.tsx`), `shared/types.ts:482-540` (`Worktree` có `projectId?`, **không có** `taskId` ở frontend), `guides/STYLEGUIDE.md`.

**Correction relative to CR-CV-092 (hợp đồng và mã thật thắng):**

| # | CR ghi | Thực tế / hợp đồng | Quyết định |
|---|---|---|---|
| 1 | Hiển thị do CR-CV-087 vẽ | Hợp đồng 8.2 giao `FE-CV-SOL-092-requirement-trace-view` cho CR này; `'requirements'` đã có trong `ReviewNoteAnchor.lens` (4.6) | Solution này sở hữu lens `requirements`, panel và hook |
| 2 | `GetRequirementTrace` | Kênh `codeIntel.quality.trace` `{projectId, worktreeId, base?, includeInferred? (true), taskId?, turnKey?}` → `{trace, evaluatedAt}` | Dùng đúng |
| 3 | `ConfirmRequirementEvidence` / `LinkWorktreeTask` | `quality.trace.confirm {requirementKey, evidenceKind, evidenceRef, linkKind:'confirm'\|'reject', scope?}` và `quality.trace.link {taskId}` (rỗng = gỡ), cả hai trả `{trace}` | Dùng đúng; `scope` hợp đồng không liệt kê giá trị: theo backend `worktree\|repo`, mặc định `worktree` (chưa kiểm chứng ở hợp đồng) |
| 4 | Cờ `quality_gate_enabled` hay chỉ `code_intel_enabled` (Q4) | PQ-01 và backend: `quality_gate_enabled` → `CODEINTEL_QUALITY_GATE_DISABLED` | Lens chỉ đăng ký khi `flags.quality` |
| 5 | Nội dung trace có thể đệm | Backend **không đệm** trace có chữ tiêu chí (quyền theo người) | Frontend chỉ giữ trong bộ nhớ theo phiên, không `localStorage`/IndexedDB |
| 6 | Nút "Liên kết với task…" | Chưa có picker; `task.list` chỉ nhận `projectId`, `pageToken`, `pageSize` (đã đọc ở v6 SOL-018) | Dựng picker bằng `Command` + `Popover` trên `useTasks(projectId)` (tải theo trang); không thêm thư viện |
| 7 | Worktree → task | Backend tự phân giải `linkConfidence`; frontend không có `Worktree.taskId` | Không suy diễn ở frontend; hiển thị `linkConfidence` và `requirement:null` khi `none` |
| 8 | Nhãn "Suy luận" | Hợp đồng enum `explicit\|derived\|inferred` | `inferred` luôn nhãn "Suy luận", icon riêng, **không** dùng màu/biểu tượng "có bằng chứng" |

**Chưa kiểm chứng:** tên API của FE-CV-SOL-050/051/053 (`pendingDiffReveal`, `REVIEW_LENS_DEFINITIONS`); quy mô dữ liệu thật của `requirements` (≤ 50) và `unlinkedChanges` (≤ 100).

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/components/review-map/requirements/
  requirement-trace-view-model.ts        (mới) hàm thuần
  use-requirement-trace.ts               (mới) hook đọc + hành động
  RequirementTracePanel.tsx              (mới) panel lens
  RequirementRow.tsx                     (mới) một yêu cầu + bằng chứng
  RequirementEvidenceList.tsx            (mới)
  UnlinkedChangesList.tsx                (mới)
  WorktreeTaskLinkPicker.tsx             (mới) Command + Popover
  (mỗi file có .test.ts(x))
components/review-map/registry: đăng ký lens 'requirements' (sửa nhỏ ở khung của FE-CV-SOL-051)
i18n/locales/*.json (sửa) + i18n/requirement-trace-locale-coverage.test.ts (mới)
```

### 2.2 Chữ ký

```ts
export type RequirementRowViewModel = {
  key: string; text: string                         // text là chuỗi tự do từ task: hiển thị văn bản thuần, cắt hiển thị 300
  origin: Requirement['origin']; state: Requirement['state']
  stateLabelKey: string                             // 'has_evidence' -> "Có dấu vết (chưa xác nhận)" ... KHÔNG "đã đáp ứng"
  stateIcon: 'trace'|'partial'|'none'|'manual'|'unknown'
  evidence: { ref: string; label: string; kind: RequirementEvidence['kind']
              confidence: RequirementEvidence['confidence']; inferred: boolean; matchedBy: RequirementEvidence['matchedBy'] }[]
  suggestions: RequirementRowViewModel['evidence']  // bằng chứng inferred (gợi ý, không tính vào state)
}
export type RequirementTraceViewModel = {
  linkConfidence: RequirementTrace['linkConfidence']
  groups: { noEvidence: RequirementRowViewModel[]; partial: ...[]; manualPending: ...[]; hasEvidence: ...[]; unknown: ...[] }  // "Chưa thấy bằng chứng" đứng đầu
  unlinked: RequirementTrace['unlinkedChanges']; warnings: string[]; summary: RequirementTrace['summary']
  canLinkTask: boolean
}
export function buildRequirementTraceViewModel(trace: RequirementTrace | null, ctx: { showInferred: boolean }): RequirementTraceViewModel

export function useRequirementTrace(args: { projectId: string|null; worktreeId: string|null; base?: string }): {
  state: 'idle'|'loading'|'ready'|'error'|'disabled'
  view: RequirementTraceViewModel | null; error: CodeIntelErrorKind | null
  showInferred: boolean; setShowInferred(v: boolean): void
  confirm(req: string, ev: {kind: string; ref: string}): Promise<void>
  reject(req: string, ev: {kind: string; ref: string}): Promise<void>
  linkTask(taskId: string): Promise<void>; unlinkTask(): Promise<void>
  refetch(): void
}
```

### 2.3 Hành vi

- `quality.trace` gọi khi lens hiển thị (không nền), kèm `includeInferred=showInferred`; làm mới theo push `quality.finished`/`quality.gateChanged` và `codeIntel.changed` (đánh dấu cũ, không tải lại giữa lúc tương tác); `CODEINTEL_TIMEOUT` `inProgress` → thử lại ≤ 90 s.
- `confirm/reject/linkTask/unlinkTask` thay `trace` bằng `{trace}` trả về (cập nhật ngay, không lạc quan); khoá nút ngay, spinner sau ~200 ms. Lỗi `forbidden` → hành động ghi chuyển chỉ đọc; `conflict`/`validation` → inline.
- **Quy tắc chữ (F10):** nhãn trạng thái: `has_evidence` "Có dấu vết (chưa xác nhận)"; `partial` "Có dấu vết một phần"; `no_evidence` "Chưa thấy bằng chứng"; `manual_pending` "Cần xác nhận thủ công"; `unknown` "Chưa đủ dữ liệu để kết luận" (khác hẳn `no_evidence`; lý do ở `warnings`); bằng chứng `inferred` ghi "Suy luận" và nằm trong mục "Gợi ý", không tính vào đếm. **Cấm** "đã đáp ứng", "đã hoàn thành", "đạt yêu cầu". Tóm tắt dùng số đếm theo từng trạng thái, không phần trăm đơn.
- `linkConfidence:'none'` → trạng thái rỗng có hành động "Liên kết với task…" (không bịa task); `title_only` kèm cảnh báo `no_structured_criteria` giải thích "chưa có tiêu chí có cấu trúc".
- Nhảy tới bằng chứng: gọi `pendingDiffReveal` của FE-CV-SOL-053-impact-lens-and-symbol-detail với `ref` tệp/symbol; `ref` dạng `runId` mở lens `quality`.
- Ghi chú Review (FE-CV-SOL-060) có thể neo `lens:'requirements'` với `nodeKey = requirement.key`.

```
┌ Yêu cầu · Task #TG-42 "Thêm lọc theo tuần"       Liên kết: suy luận ┐
│ [☐ Hiện gợi ý suy luận]   [Đổi task…]                              │
│ ── Chưa thấy bằng chứng (2) ──                                      │
│  ○ Lọc theo tuần trả đúng ngày đầu tuần         (checklist)         │
│      Gợi ý (suy luận): week-filter.ts   [Xác nhận] [Bỏ]             │
│ ── Có dấu vết một phần (1) ──                                       │
│  ◐ Có test cho lọc rỗng   thay đổi: filter.ts · chưa thấy test      │
│ ── Cần xác nhận thủ công (1) ──                                     │
│  ◌ Giao diện đúng thiết kế            [Tôi đã kiểm tra]             │
│ ── Thay đổi chưa gắn yêu cầu (3) ──  a.ts · b.ts · c.ts            │
└─────────────────────────────────────────────────────────────────────┘
```

Màu/kiểu: token `muted-foreground`, `border`, `accent` (hàng hover), không dùng `destructive` cho "chưa thấy bằng chứng" (không phải lỗi); icon `lucide-react` khác hình theo trạng thái (không chỉ màu); danh sách ≤ 50 mục dùng `ScrollArea`; `Command` + `Popover` cho picker (STYLEGUIDE: chọn một từ danh sách có tìm kiếm). Mọi chuỗi qua `translate()`; `text` và `label` từ backend là văn bản thuần (U9).

### 2.4 Picker task

`WorktreeTaskLinkPicker`: `Popover` + `Command` có ô tìm; nguồn `useTasks(projectId)` (đã tải theo dự án; lọc `type`/`status` không cần); hiển thị `#TG-{taskNumber} {title}`; chọn → `linkTask(task.id)`; "Gỡ liên kết" → `unlinkTask()` (`taskId` rỗng). Ngữ nghĩa quyền: nếu backend từ chối vì thiếu quyền đọc task thì thông báo inline chung, không lộ tồn tại task.

## 3. Quyết định thiết kế

- Nhãn không bao giờ khẳng định đáp ứng; `unknown ≠ no_evidence`; `inferred` chỉ là gợi ý.
- Không lưu nội dung trace ở bộ nhớ bền của trình duyệt (dữ liệu nhạy cảm, quyền theo người).
- Tách view-model thuần để kiểm thử nhãn không cần React.
- Không thêm thư viện; không gọi `task-service` trực tiếp ngoài `useTasks` có sẵn (task-service chỉ được dùng cho picker).

## 4. Phụ thuộc chéo khu vực

| Cần | Nơi |
|---|---|
| Ba kênh `quality.trace*` | `BE-CV-SOL-092-requirement-trace`, `BE-CV-SOL-040-codeintel-quality-channels`; trước G3 dùng fake backend G4 (fixture: `none`, `explicit`, `title_only`, `unknown` do index cũ, bằng chứng `inferred`) |
| Dữ liệu overlay/test/run | `BE-CV-SOL-036-*`, `BE-CV-SOL-085-*`, `BE-CV-SOL-082-quality-run-storage-and-ingest` |
| Tiêu chí có cấu trúc (v6) | CR-REQ-027/029 qua backend; frontend không đọc trực tiếp |
| Khung lens, đăng ký lens, cờ | `FE-CV-SOL-051-review-workspace-shell`, `FE-CV-TASK-085-01`, `FE-CV-SOL-050-*` |
| Nhảy tới diff | `FE-CV-SOL-053-impact-lens-and-symbol-detail` (`pendingDiffReveal`) |
| Ghi chú neo lens | `FE-CV-SOL-060-review-notes-and-turn-compare` |
| Telemetry lens | `FE-CV-SOL-095-review-telemetry` (`lens: 'requirements'`) |
| AG | Không có |

## 5. Tiêu chí chấp nhận

- [ ] Không có chuỗi nào chứa "đã đáp ứng", "đã hoàn thành", "đạt yêu cầu" (test quét khoá locale en).
- [ ] Yêu cầu chỉ có bằng chứng `inferred` nằm ở "Chưa thấy bằng chứng" kèm mục "Gợi ý"; không tăng đếm "có dấu vết".
- [ ] `unknown` hiện "Chưa đủ dữ liệu để kết luận" với icon khác `no_evidence`.
- [ ] `confirm`/`reject` cập nhật trace trả về; `reject` làm gợi ý biến mất; nút khoá ngay.
- [ ] `linkConfidence:'none'` hiện hành động liên kết; picker liên kết/gỡ gọi `quality.trace.link` đúng `taskId`.
- [ ] Lens không đăng ký khi `flags.quality=false`; 0 lời gọi.
- [ ] Không ghi trace vào lưu trữ trình duyệt.
- [ ] `text` chứa `<script>`/markdown hiển thị đúng dạng chữ.

## 6. Kiểm thử (Vitest + Testing Library)

View-model (đủ năm `state`, `origin`, `inferred`, gom nhóm, đếm), hook (retry `inProgress`, hành động thay trace, cờ tắt), panel/row/picker (`// @vitest-environment happy-dom`; bàn phím trong `Command`; Esc), test quét khoá locale cấm từ. Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/requirements` (chưa chạy).

## 7. Rủi ro và điểm chưa kiểm chứng

- Giá trị thực phụ thuộc v6 (AC có cấu trúc); nhánh Task thường chỉ có `title_only` (CR mục 6).
- `Command` với hàng nghìn task: dùng `useTasks` đã tải theo trang; có thể cần tìm phía server (chưa có kênh).
- Khoá `task:…#hash` đổi khi sửa chữ tiêu chí → xác nhận cũ mất (chấp nhận).
- `scope` của `confirm` chưa nêu trong hợp đồng.

## 8. Câu hỏi mở

1. Giá trị `scope` của `quality.trace.confirm` (`worktree|repo`?) và mặc định.
2. Có hiển thị `summary` của trace ở thanh tóm tắt Review không (CR-092 Q6: không đưa vào cổng).
3. Picker cần tìm phía server khi dự án có quá nhiều task?

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-092-requirement-traceability.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/frontend/src/renderer/src/hooks/useTasks.ts`, `/opt/repos/orca/frontend/src/shared/task-types.ts`.
