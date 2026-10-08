# FE-CV-SOL-060: Ghi chú review gắn nút đồ thị, gửi theo lô cho agent, so sánh lượt trước/lượt này

> 🚧 **In Progress.** Trạng thái (cập nhật 2026-10-08): 7/8 task DONE; PARTIAL 1 (060-08). Chi tiết ở dòng `**Status:**` và "Ghi chú hoàn thiện" của từng task.

**CR:** [CR-CV-060](../../../../../../docs/crs/v7/review-frontend/CR-CV-060-review-notes-send-to-agent-and-turn-compare.md)
**Area:** frontend (`components/review-map/notes/`, `components/review-map/turns/`, `lib/`, slice)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (U1, U3, U5; §3.1 `reviewState.get|save`; §4.6 `ReviewNoteAnchor`, `ReviewSentBatch`, `ReviewTurnMarker`, `ReviewState`; §2.3 `CODEINTEL_VERSION_CONFLICT`, `CODEINTEL_PAYLOAD_TOO_LARGE`; §2.4 giới hạn `args[0]` ≤ 256 KiB), [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (**PQ-02, PQ-04, PQ-14, PQ-22, PQ-31, PQ-35**; §7; §8.2; §9 O-1, O-2, O-8). **TDD:** [v5/02-state-management](../../../../tdd/v5/02-state-management.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md).

## 1. Trạng thái hiện tại (re-verify)

**Đã xác minh trong phiên này:**
- `components/editor/DiffNotesSendMenu.tsx`: `markAnnotationsSentBestEffort` là hàm **riêng tư** (dòng 16), gọi ở dòng 136; props gồm `worktreeId, groupId, comments, filePath?, showFileScope?, …`; GitNexus: chỉ `DiffNotesSendMenu` gọi nó.
- `components/editor/NotesSendMenu.tsx` export `NotesSendMenu<TNote>` generic, `NotesSendMenuScope<TNote> {id, label, notes, prompt}`, tham số `source = 'diff-notes'`; `store/slices/ui.ts:138` `AgentSendPopoverTargetMode.source: 'diff-notes' | 'browser-annotations'`.
- `store/slices/diffComments.ts` có `clearDeliveredDiffComments` (dòng 229); `lib/use-composed-all-notes-prompt.ts`, `shared/diff-comments-format.ts` tồn tại; `lib/annotation-mark-sent-best-effort.ts` **chưa có**.
- `store/slices/agent-status.ts`: `agentStatusByPaneKey` (dòng 101), `retainedAgentsByPaneKey` (115); `shared/agent-status-types.ts` có `stateStartedAt` (dòng 112); `shared/git-status-types.ts`; `store/slices/editor.ts:696` `gitBranchCompareSummaryByWorktree`; `mobile/src/session/mobile-diff-review-queue.ts` chứa mẫu identity. `components/review-map/` chưa có.
**Theo CR-CV-060 (chưa kiểm lại):** hành vi chi tiết của `NotesSendMenu`/`sendNotesToActiveAgentSession`; `clearDeliveredDiffComments` xoá ghi chú đã giao.

### Lệch giữa CR và hợp đồng (hợp đồng thắng)

| # | CR-060 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `notes` / `turns[]` trong `reading_progress` | PQ-22: `notes = {anchors, sentBatches}`; `turnMarkers` là trường riêng, **chỉ ở dòng mức worktree** (`baseCommit=''`, `headCommit=''`), ≤ 5 phần tử | Hai dòng `ReviewState`: dòng `(base,head)` cho notes/progress, dòng `('','')` cho markers; hai lần `save` độc lập |
| 2 | `reviewState.save` chỉ `readingProgress` + `expectedVersion` (SOL của 052) | `save` nhận `notes?`, `turnMarkers?`, `status?`; `readingProgress` bắt buộc | Dùng bộ ghi của FE-CV-SOL-052 (đồng bộ một hàng đợi/dòng); task 060-05 chỉ thêm patch `notes`/`turnMarkers` |
| 3 | `anchorsByCommentId` | `notes.anchors: Record<string, ReviewNoteAnchor>` (khoá commentId) | Giữ tên hợp đồng |
| 4 | `ReviewTurnMarker.startedAt: number` | `startedAt: number | null`, `interrupted?`, `overlayAvailable`, `symbolKeys?`, `files {p,o?,h}[]` | Theo hợp đồng |
| 5 | Giới hạn lô 10/100 (đề xuất) | `notes` ≤ 256 KiB, ≤ 500 mục; `turnMarkers` ≤ 5; `reviewState.save` ≤ 256 KiB (cần `SetReadLimit` 320 KiB, O-2) | Cắt `sentBatches` theo tổng mục ≤ 500 và dung lượng; `PAYLOAD_TOO_LARGE` ⇒ cắt batch cũ nhất rồi thử lại 1 lần |
| 6 | "Không gửi `prompt` lên backend" | PQ-22: **Không lưu prompt người dùng**; `agent_turns` là kho provenance riêng (FE-CV-SOL-089, `quality.turn.record`) | Marker không chứa prompt; không gọi `quality.turn.record` ở đây |
| 7 | Tín hiệu "agent xong" tự phát hiện | PQ-35: chỉ nguồn renderer; `codeIntel.hintAgentTurnFinished` là P1, chưa là kênh v7 | Dùng selector của FE-CV-SOL-061 (task 061-01); không gọi hint |
| 8 | Khoá i18n `auto.components.review.map.notes|turns.` | README nhóm `auto.components.reviewMap.<Thành phần>.<tên>` | Theo README nhóm |
| 9 | `turnId` | `${paneKey}:${doneAt}` = `agent_turns.client_turn_id` | Cùng định danh với FE-CV-SOL-089 |
| 10 | `body` ghi chú lưu trong `ReviewSentBatch` | Không có quy định che | Lưu bản đã `maskSensitiveText`, cắt 2000 ký tự (chỉ để hiển thị lịch sử; lời nhắc gửi đi vẫn là bản gốc) |

## 2. Giải pháp

### 2.1 Cây file

```
components/review-map/notes/
  review-note-anchor.ts            (mới) resolveGraphNodeCommentTarget, buildGraphNoteBody
  review-sent-batch.ts             (mới) buildSentBatch, trimSentBatches (trần 500 mục/dung lượng)
  ReviewNoteButton.tsx, ReviewNoteComposerPopover.tsx, ReviewNodeNoteBadge.tsx,
  ReviewNotesPanel.tsx, ReviewNotesSendMenu.tsx, ReviewSendPreviewDialog.tsx, ReviewSentBatchList.tsx  (mới)
components/review-map/turns/
  turn-file-identity.ts, turn-compare-model.ts, useReviewTurnRecorder.ts, ReviewTurnSwitcher.tsx  (mới)
lib/annotation-mark-sent-best-effort.ts   (mới, trích từ DiffNotesSendMenu.tsx; sửa DiffNotesSendMenu chỉ để import lại)
hooks/useReviewNotesPersistence.ts        (mới) nối bộ ghi reviewState của SOL-052
store/slices/code-intel.ts                (sửa nhỏ: `reviewNotesState`, `reviewTurnsState` vào CODE_INTEL_WORKTREE_KEYED_STATE_KEYS)
```

### 2.2 Ghi chú = `DiffComment`

Mọi ghi chú review là `DiffComment` (tái dùng `addDiffComment/updateDiffComment/deleteDiffComment`, lưu bền, đồng bộ annotation-service, đếm ở Source Control, đường gửi). Phần riêng là **neo** (`ReviewNoteAnchor` của hợp đồng §4.6, lens ∈ `impact|architecture|dataflow|erd|storage|structure|contract|quality|requirements`) lưu ở `ReviewState.notes.anchors` và **tiền tố nội dung** `[Review map · <lens> · <label>] <nội dung>`. Không đổi `shared/types.ts`/`diff-comments-format.ts` (lan sang annotation-service và mobile, chưa kiểm chứng).

| Nút | `filePath` | `startLine`/`lineNumber` |
|---|---|---|
| Symbol | `SymbolRef.filePath` | `startLine`, `lineNumber = endLine ?? startLine` |
| Bảng ERD | `ErdChange.migrationFile` hoặc `ErdTable.firstMigration` | `0` (cấp tệp) |
| Thay đổi hợp đồng | `ContractChange.files[0]` | `0` (hoặc `evidence[0].line`) |
| Phát hiện (`kind:'finding'`) | `evidence[0].path` | `evidence[0].line` hoặc `0` |
| Cụm/service/kho/secret không có tệp | — | nút "Ghi chú" **khoá** + lý do |

Neo thiếu trong `anchors` ⇒ ghi chú vẫn là ghi chú dòng thường. `diffIdentity`/`scope` không đặt. `label` tự do qua `maskSensitiveText` trước khi vào tiền tố.

### 2.3 Gửi theo lô

`ReviewNotesSendMenu` bọc `NotesSendMenu<DiffComment>` với ba scope: `all` (dùng đúng `useComposedAllNotesPrompt`), `lens`, `selection` (dùng `formatDiffComments` client-side; **không** `annotation.markSent`). `onDelivered` theo thứ tự: (1) `recordReviewSentBatch` (không chặn) (2) `clearDeliveredDiffComments` (3) `markAnnotationsSentBestEffort` chỉ scope `all` ở runtime `environment`. Hàm (3) trích sang `lib/annotation-mark-sent-best-effort.ts`; **trước khi sửa `DiffNotesSendMenu` phải chạy `gitnexus_impact` (báo blast radius) và có test hồi quy**. `source` giữ `'diff-notes'`. `ReviewSendPreviewDialog` (`ui/dialog`) xem lời nhắc chỉ đọc + cảnh báo lô > 50 ghi chú (ngưỡng chưa đo).

### 2.4 Lưu `ReviewState` (notes, sentBatches, turnMarkers)

- `useReviewNotesPersistence`: đọc `reviewState.get {baseCommit, headCommit}` (khi chưa có ⇒ `version:0`), nhận patch `{anchors, sentBatches}` và đẩy vào bộ ghi `reviewState.save` của SOL-052 (tuần tự theo dòng, `expectedVersion` = version đã đọc). `CODEINTEL_VERSION_CONFLICT` (kèm `currentVersion`): tải lại, **gộp** (anchors hợp theo commentId — bản có `at` mới thắng; `sentBatches` hợp theo `batchId`; `turnMarkers` hợp theo `turnId`, giữ 5 mới nhất), thử lại một lần, thất bại ⇒ banner inline "Chưa lưu được".
- Dòng mức worktree `('','')` chứa `turnMarkers`; dòng `(base,head)` chứa notes/progress. `readingProgress` bắt buộc khi save: luôn gửi bản hiện có từ bộ ghi 052.
- Offline/dev server mất kết nối: ghi chú vẫn lưu cục bộ (lớp lưu hiện có); `reviewState.save` hoãn, đánh dấu cũ.

### 2.5 Mốc lượt và so sánh

- **Sự kiện xong:** `selectAgentTurnCompletions` (FE-CV-SOL-061, 061-01) cho `AgentTurnCompletion` với `id = "${paneKey}:${doneAt}"`; FE-CV-SOL-089 dùng cùng sự kiện để `quality.turn.record` (provenance, khác marker) — dùng chung một khử trùng lặp, không hai debounce.
- `useReviewTurnRecorder` (mức `ReviewWorkspace`, chạy cả khi tab Review chưa mở nếu cờ bật): chờ git ổn định (debounce ~3 s, chưa đo, tái dùng cơ chế làm mới git status hiện có) rồi tạo `ReviewTurnMarker` (`files {p,o?,h}` cap 500, `symbolKeys` cap 1500 từ `ChangeOverlay` nếu có, `overlayAvailable`), lưu ≤ 5 marker ở dòng worktree. **Không lưu prompt.**
- `turn-file-identity.ts`: băm thô `status, oldPath, path, added, removed, area/scope` + `headOid/mergeBase` (cùng ý tưởng mobile `statusEntryIdentity`); nhãn "ước lượng" vì trùng khi cùng số dòng.
- `turn-compare-model.ts` → nhãn `new_in_turn|changed_in_turn|unchanged_since|reverted_in_turn` mức tệp, mức symbol khi cả hai marker có `symbolKeys`.
- `ReviewTurnSwitcher`: ba chế độ ("Toàn bộ thay đổi", "Chỉ phần đổi từ lượt trước", "Xem lượt trước" chỉ đọc, **không dựng lại diff**); chưa có marker trước ⇒ vô hiệu hoá + "Chưa có lượt trước để so sánh". Lớp phủ dùng mã hoá của SOL-053 (`review-overlay-model.ts`) — thêm nhãn lượt, không màu mới.
- Nhãn trên ghi chú đã gửi: "Tệp đã đổi từ khi gửi"/"Chưa thấy đổi ở tệp này" (gợi ý, có thể sai; không viết "đã xử lý"); người dùng tự đánh dấu cục bộ.

### 2.6 Trạng thái, phím, i18n

Lưu ghi chú: khoá nút ngay, hiện trạng thái sau ~200 ms (SSH). Không agent để gửi: do `ReviewNotesSendMenuContent`. Ghi marker thất bại: dòng inline + "Thử lại", không chặn gửi. Phím (registry SOL-052; chỉ chip khi đã cài): `n` soạn ghi chú nút đang chọn (tiêu điểm không ở ô nhập), `Mod+Enter` lưu (`isScreenSubmitShortcut`, `metaKey` Mac / `ctrlKey` nơi khác). Khoá `auto.components.reviewMap.ReviewNote*|ReviewTurn*|ReviewSent*` đủ en/es/ja/ko/zh. Chỉ token CSS; reduced-motion tắt animation. Không dùng `window.api` trong component.

## 3. Quyết định thiết kế

- **Ghi chú đồ thị là `DiffComment`**, không kho song song; chấp nhận ràng buộc "phải gắn tệp".
- **Neo ở `review_states.notes`, tiền tố ở nội dung**: không đổi kiểu dùng chung.
- **Bọc `NotesSendMenu` generic** để chèn bước lưu lô trước khi ghi chú bị xoá.
- **Marker ghi lúc agent xong**, không suy ngược từ `stateHistory` (trong bộ nhớ, không có ảnh chụp mã).
- **Marker ≠ `agent_turns`** (PQ-22): marker nhẹ cho so sánh UI; provenance là FE-CV-SOL-089.
- **Nhãn "ước lượng", không khẳng định "đã xử lý"**.

## 4. Phụ thuộc chéo khu vực

| Cần | Solution đối ứng | Ghi chú |
|---|---|---|
| Bảng `review_states` + RPC | `BE-CV-SOL-011-data-model-and-migrations`; `BE-CV-SOL-040-codeintel-write-and-stream-channels` (`reviewState.get|save`) | Cần `SetReadLimit` 320 KiB (`BE-CV-SOL-040-codeintel-channel-foundation`, O-2); trước đó dùng fake backend G4 |
| `symbolKeys`, `ChangeOverlay` | `BE-CV-SOL-036-change-overlay-pipeline` | `overlayAvailable=false` khi thiếu |
| Provenance lượt (khác marker) | `BE-CV-SOL-089-agent-turn-provenance`; FE: `FE-CV-SOL-089-agent-turn-recorder` | Dùng chung `turnId` |
| Bộ ghi reviewState, phím, panel | `FE-CV-SOL-052-reading-order-and-progress`, `FE-CV-SOL-053-impact-lens-and-symbol-detail`, `FE-CV-SOL-051-review-workspace-shell` | |
| Sự kiện "agent xong" | `FE-CV-SOL-061-review-entry-points` (061-01) | |
| Neo từ lens | `FE-CV-SOL-057/058/059` | `ReviewNoteButton` cắm vào panel chi tiết của từng lens |
| Fake backend G4 | `FE-CV-SOL-073-flag-gating-and-web-e2e` (073-02) | **Bắt đầu bằng fake backend** |
| Agent | `AG-*`: — | |

## 5. Tiêu chí chấp nhận

- [ ] Ghi chú từ symbol/ERD/hợp đồng/phát hiện lưu bằng `addDiffComment` với bảng 2.2 và tiền tố; nút không gắn tệp bị khoá có lý do.
- [ ] Ghi chú xuất hiện ở số đếm Source Control và trong lô "All unsent notes".
- [ ] `ReviewNotesPanel` nhóm theo lens/neo, sửa/xoá, nhảy tới neo; sửa ghi chú đã gửi đưa về hàng chờ.
- [ ] `ReviewNotesSendMenu` có `all|lens|selection`; `all` cùng lời nhắc với Source Control; chỉ `all` gọi `markSent`.
- [ ] Gửi thành công: lưu `ReviewSentBatch` → `clearDeliveredDiffComments`; `DiffNotesSendMenu` không đổi hành vi (test hồi quy, impact đã chạy).
- [ ] `reviewState.save` dùng `expectedVersion`; xung đột được gộp và thử lại; `notes ≤ 500 mục/256 KiB`, `turnMarkers ≤ 5`; markers chỉ ở dòng `('','')`.
- [ ] Agent xong ⇒ một marker (khử trùng lặp theo `turnId`), không prompt; chưa có marker trước ⇒ chế độ 2–3 vô hiệu hoá.
- [ ] Nhãn so sánh có chú giải "ước lượng"; không "đã xử lý" tự động.
- [ ] `translate()` đủ 5 locale; không hex; không `components/code-review/*`; không thêm thư viện; không `max-lines` disable; Electron và web.

## 6. Kiểm thử (Vitest + Testing Library)

Hàm thuần: `review-note-anchor`, `review-sent-batch` (trần), `turn-file-identity`, `turn-compare-model` (bốn nhãn, symbol thiếu một phía). Component: composer (lưu, giữ nháp khi lỗi, `Mod+Enter`), `ReviewNotesPanel`, `ReviewNotesSendMenu` (thứ tự `record` trước `clear`; giả lập `useAppStore`/`callRuntimeRpc`), `ReviewTurnSwitcher`. Hồi quy: `NotesSendMenu.test.tsx`, `ReviewNotesSendMenuContent.test.tsx`, test `DiffNotesSendMenu`. Hook: gộp xung đột, `PAYLOAD_TOO_LARGE`. Slice: dọn khi xoá worktree; không ghi marker khi cờ tắt. E2E trên fake backend (gửi lô, thấy "Đã gửi ở lượt trước", lượt kế có nhãn) — **chưa chạy**.

## 7. Rủi ro và điểm chưa kiểm chứng

- Không có ảnh chụp mã theo lượt: so sánh chỉ ở dấu vân tay thô.
- Nhiều agent trong một worktree: "lượt trước" chưa chốt (đề xuất cùng pane, nếu không thì marker liền trước).
- Marker chỉ ghi khi renderer chạy; mất khi đóng ứng dụng lúc agent xong.
- Hai bên (052 và 060) cùng ghi một dòng `ReviewState`: sai thứ tự gây xung đột; phải qua một bộ ghi.
- `SetReadLimit`/O-2 chưa chốt: `save` lớn có thể bị cắt kết nối.
- Trích `markAnnotationsSentBestEffort` chạm `DiffNotesSendMenu` (3 nơi gọi theo CR).
- `body` ghi chú đưa lên backend (đã mask+cắt) — cần xác nhận chính sách; telemetry của `source:'diff-notes'` chưa kiểm.

## 8. Câu hỏi mở

1. Có thêm `graphRef` có cấu trúc vào `DiffComment`/định dạng lời nhắc (ảnh hưởng annotation-service, mobile)?
2. "Lượt" theo agent hay theo worktree?
3. Có lưu marker ở máy khách khi dev server offline?
4. Lưu `body` ghi chú lên `review_states` có chấp nhận (đã mask) không?

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-060-review-notes-send-to-agent-and-turn-compare.md`, `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§6.5–6.6), `/opt/repos/orca/frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx`, `NotesSendMenu.tsx`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/diffComments.ts`, `agent-status.ts`, `ui.ts`, `/opt/repos/orca/mobile/src/session/mobile-diff-review-queue.ts`.

## 14. Ghi chú triển khai (2026-10-07)

**Sai lệch so với spec**

- Đã chạy `gitnexus impact`: `markAnnotationsSentBestEffort` LOW (1 caller trực tiếp), `DiffNotesSendMenu` HIGH (3 caller trực tiếp), `clearDeliveredDiffComments` LOW và không đổi. `DiffNotesSendMenu.tsx` chỉ xoá hàm riêng tư và import `lib/annotation-mark-sent-best-effort.ts`; thêm test hồi quy `components/editor/DiffNotesSendMenu.test.tsx`.
- Không thêm `reviewNotesState/reviewTurnsState` vào slice `code-intel`: dòng (base,head) đi qua `patchReviewState` của slice `review-progress` (writer 052); dòng `('','')` (turnMarkers) đi qua `turns/review-turn-marker-row.ts` (tuần tự theo worktree). Hook `useReviewNotesPersistence` gộp lại notes bị slice 052 ghi đè khi xung đột.
- `CODEINTEL_PAYLOAD_TOO_LARGE` ánh xạ thành `too-large` (sửa 1 dòng `review-shell-data.ts`); hook thu nhỏ còn 250 mục rồi thử lại một lần.
- Lô đã gửi: `turnId/targetPaneKey/agentType = null` vì `NotesSendMenu.onDelivered` không báo pane đích. Thứ tự: ghi lô → `clearDeliveredDiffComments` → `markAnnotationsSent` (chỉ scope all).
- "Agent xong" dùng `detectAgentTurnCompletions` có sẵn (SOL-061-01 chưa có). `useReviewTurnRecorder` chưa được mount (khung của agent khác).
- `ReviewNotesPanel`, `ReviewNodeNoteBadge`, `ReviewTurnSwitcher` chưa được gắn vào khung/lens; `ReviewNoteButton` đã gắn ở chi tiết Hợp đồng (059) và dòng Phát hiện. Lớp phủ lượt: switcher phát `onCompare(TurnCompareResult)`, chưa nối `review-overlay-model.ts`.
- Neo `ReviewNoteAnchor` không có `at` ⇒ gộp neo theo commentId: local thắng. Phím `n` cài cục bộ (prop `hotkey`), chưa qua registry 052.
- Khoá i18n: `auto.components.reviewMap.ReviewNote.*`, `.ReviewSent.*`, `.ReviewTurn.*`; test phủ khoá `i18n/contract-findings-notes-locale-coverage.test.ts`. Chưa có e2e `review-notes.web.e2e.ts`.
