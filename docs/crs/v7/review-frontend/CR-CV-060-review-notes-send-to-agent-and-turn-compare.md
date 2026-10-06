# CR-CV-060 — Ghi chú review gắn vào nút đồ thị, gửi theo lô cho agent, so sánh "lượt trước" và "lượt này"

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-060 |
| **Tên** | Khép vòng review: ghi chú gắn vào dòng/nút đồ thị (tái dùng `diff-comments`), gửi theo lô cho agent bằng luồng `NotesSendMenu`, chuyển giữa "lượt trước" và "lượt này" của agent trong cùng worktree |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050 (slice, `codeIntelClient.call` cho `codeIntel.reviewState.get/save`, i18n, `useCodeIntelSupport`), CR-CV-051 (khung, thanh phạm vi, cột phải), CR-CV-052 (`review_states`, tiến độ đã xem, registry phím tắt), CR-CV-053 (panel chi tiết symbol, mở diff đúng dòng), CR-CV-059 (ghi chú gắn vào phát hiện), CR-CV-061 (nguồn sự kiện "agent xong"); backend CR-CV-011 (`review_states`), CR-CV-036 (`ChangeOverlay`) |
| **Mở khoá** | CR-CV-062 (đếm ghi chú chưa gửi trên mobile, tuỳ chọn) |
| **Tác động** | `frontend/src/renderer/src/components/review-map/notes/` và `components/review-map/turns/` (mới), `components/editor/DiffNotesSendMenu.tsx` (chỉ trích hàm dùng chung, không đổi hành vi), `lib/` (module trích ra), `store/slices/code-intel.ts` (khoá `reviewNotes`, `reviewTurns`), `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

[10 §6.5–6.6](../../../research/view-code/10-frontend-review-ux.md) đề xuất: ghi chú gắn vào nút hoặc dòng diff dùng lại `diff-comments`, gửi cho agent bằng `DiffNotesSendMenu`, cho phép gửi một lô; và chuyển giữa "lượt trước" và "lượt này" của agent trong cùng worktree để thấy phần agent vừa sửa theo phản hồi. Cần đọc code thật để biết chính xác cái gì tái dùng được và cái gì **chưa có dữ liệu**.

### 1.1 Cách ghi chú và gửi cho agent hoạt động hiện nay (đã đọc code)

**Mô hình ghi chú.** `DiffComment` (`shared/types.ts`): `id, worktreeId, filePath, source? ('diff'|'markdown'), selectedText?, startLine?, lineNumber, body, createdAt, updatedAt?, sentAt?, scope? ('unstaged'|'staged'|'branch'), oldPath?, diffIdentity?, side:'modified'`. Chú thích trong file nói ghi chú được lưu trên `WorktreeMeta` nên lớp lưu bền hiện có ghi vào `orca-data.json`; với runtime target `environment` (web/nhiều người dùng) còn được đồng bộ với `annotation-service` (`ensureDiffCommentsHydrated` trong `store/slices/diffComments.ts`, `diff-comments-annotation-sync.ts`).

**Gắn vào dòng.** `DiffViewer.tsx` (`handleSubmitComment`) gọi `addDiffComment({worktreeId, filePath: relativePath, source:'diff', startLine, lineNumber, body, side:'modified'})` (`store/slices/diffComments.ts`); chờ lưu xong mới đóng popover, rollback khi lỗi. `lineNumber === 0` nghĩa là **ghi chú cấp tệp** ("Scope: file" khi định dạng). Sửa nội dung xoá `sentAt` (ghi chú trở lại hàng chờ gửi).

**Định dạng lời nhắc.** `shared/diff-comments-format.ts` (`formatDiffComment/formatDiffComments`) sinh khối `File: …`, `Line: n` / `Lines: a-b` / `Scope: file`, `User comment: "…"` (có escape); đây là "hợp đồng giữa ghi chú và agent nhận". Với runtime `environment`, `lib/use-composed-all-notes-prompt.ts` thay lời nhắc của phạm vi "all unsent notes" bằng kết quả RPC `annotation.composeReviewPrompt` (kèm ngữ cảnh ±2 dòng mã), debounce 500 ms, lỗi thì dùng lại `formatDiffComments` phía client (đúng như chú thích trong file).

**Menu gửi.** `components/editor/DiffNotesSendMenu.tsx` bọc `NotesSendMenu` (`components/editor/NotesSendMenu.tsx`, generic theo kiểu ghi chú). Props của `DiffNotesSendMenu`: `worktreeId, groupId, comments, filePath?, showFileScope?, triggerClassName/Label/Count, actionLabel, iconClassName, align`. Hai phạm vi: "This file" (khi `showFileScope`) và "All unsent notes"; nơi gọi hiện có là `CombinedDiffViewer`, `EditorPanelHeader`, `SourceControl` (`components/right-sidebar/SourceControl.tsx` ~dòng 5045). `NotesSendMenu` mở "chế độ chọn đích" qua `openAgentSendPopoverTargetMode` (`store/slices/ui.ts`; `source: 'diff-notes' | 'browser-annotations'`, `launchSource: 'notes_send'`): các hàng agent trong sidebar trở thành bộ chọn đích. Nội dung menu là `ReviewNotesSendMenuContent.tsx`: liệt kê mọi agent đang chạy của worktree (`deriveNotesSendAgentTargets`, chỉ mục `eligible/disabled` kèm lý do) và "New agent" (`QuickLaunchAgentMenuItems`).

**Giao cho agent.** Gửi tới agent có sẵn dùng `sendNotesToActiveAgentSession` (`lib/active-agent-note-send.ts`): tìm terminal đúng host chủ của worktree, `terminal.wait` điều kiện `tui-idle`, rồi gửi bằng dán có bảo vệ (bracketed paste) + Enter (`sendPromptWithGuardedPasteAndEnter`; nhánh cũ dùng `terminal.send` cho runtime SSH cũ). Có toast "Sending notes…/Notes sent." (`runNotesSend`). Khi giao thành công, `DiffNotesSendMenu` gọi `clearDeliveredDiffComments` (**xoá** các ghi chú đã giao khỏi store nếu chưa bị sửa trong lúc gửi) và, chỉ với phạm vi "all unsent" ở runtime `environment`, `annotation.markSent` (hàm `markAnnotationsSentBestEffort`, **riêng tư trong file**).

Hệ quả quan trọng cho CR này: (1) ghi chú gắn vào nút đồ thị nếu lưu dưới dạng `DiffComment` sẽ tự xuất hiện ở số đếm ghi chú của Source Control và đi trong lô "All unsent notes"; (2) ghi chú đã gửi **bị xoá** nên không còn lịch sử "đã gửi gì ở lượt trước" trừ khi CR này lưu một bản ghi riêng.

### 1.2 "Lượt" của agent được lưu ở đâu: **chưa có dữ liệu để so sánh**

- `AgentStatusEntry` (`shared/agent-status-types.ts`) có `state` (`working|blocked|waiting|done`), `prompt`, `stateStartedAt`, `stateHistory[]` (`{state, prompt, startedAt, interrupted?}`, tối đa `AGENT_STATE_HISTORY_MAX = 20`), `interrupted`. Slice `agent-status.ts` ghi rõ `agentStatusByPaneKey` là **thời gian thực, nằm trong bộ nhớ renderer, không lưu bền**; `retainedAgentsByPaneKey` giữ bản chụp của agent đã xong cho tới khi người dùng xác nhận; `last-status.json` (desktop main, `agent-hooks/server.ts`) chỉ lưu **trạng thái cuối cùng theo pane** có TTL, không phải lịch sử lượt.
- Do đó "một lượt" có thể suy ra (chu kỳ `working → done` của một `paneKey`, danh tính `paneKey + stateStartedAt` của lần `done`) nhưng: không bền qua khởi động lại, tối đa ~20 chuyển trạng thái, gắn theo pane chứ không theo worktree, và **không kèm ảnh chụp mã** tại thời điểm xong. Không có chỗ nào hiện lưu "trạng thái worktree ở cuối lượt N".
- Cách gần nhất đã có: mobile tính `diffIdentity` cho từng tệp (`mobile/src/session/mobile-diff-review-queue.ts`: `statusEntryIdentity` băm `scope, area, status, oldPath, path, added, removed, conflictStatus`; `branchEntryIdentity` thêm `branchMergeBase`, `branchHeadOid`) và lưu `MobileDiffReviewState` theo worktree (`Worktree.mobileDiffReview`, `shared/types.ts`) để biết tệp đã đổi từ lần xem trước. Renderer desktop **không** có hàm tương đương (grep `diffIdentity` trong `frontend/src/renderer` ngoài test: không có).
- Nguồn cho danh sách tệp tại thời điểm xong: `GitStatusEntry` (`added/removed` theo staging area, `shared/git-status-types.ts`) và `GitBranchCompareResult` (`entries[]` có `added/removed`, `summary.headOid/mergeBase`, `shared/types.ts`; lưu ở `gitBranchCompareSummaryByWorktree` trong `store/slices/editor.ts`).

Vì vậy CR này **thêm một bản ghi "mốc lượt"** (2.4) và nói rõ phần so sánh chỉ ở mức tệp/symbol, không dựng lại nội dung diff của lượt trước trừ khi hai lượt là commit.

## 2. Giải pháp đề xuất

### 2.1 Ghi chú gắn vào dòng và vào nút đồ thị

Nguyên tắc: **mọi ghi chú review đều là `DiffComment`** (cùng `addDiffComment/updateDiffComment/deleteDiffComment`), nên dùng chung lưu bền, đồng bộ `annotation-service`, đếm ở Source Control, và đường gửi. Phần riêng của đồ thị là **neo (anchor)** và **tiền tố nội dung**.

`notes/review-note-anchor.ts` (hàm thuần):

```ts
type ReviewNoteAnchor =
  | { kind: 'diff-line'; filePath: string; startLine?: number; lineNumber: number }
  | { kind: 'graph-node'; lens: ReviewLensId; nodeKey: string /* SymbolRef.key | table | store | contract id */;
      filePath: string; startLine?: number; endLine?: number; label: string }
  | { kind: 'finding'; findingKey: string; filePath: string; startLine?: number; label: string }
resolveGraphNodeCommentTarget(anchor): { filePath: string; startLine?: number; lineNumber: number } | null
buildGraphNoteBody(anchor, userText): string
```

Quy tắc ánh xạ nút → `DiffComment` (vì `filePath` là bắt buộc):

| Nút | `filePath` | `startLine`/`lineNumber` |
|---|---|---|
| Symbol (`SymbolRef` có `filePath` + dòng) | `SymbolRef.filePath` | `startLine = SymbolRef.startLine`, `lineNumber = SymbolRef.endLine ?? startLine` |
| Bảng ERD | tệp migration đã tạo/đổi bảng (`ErdChange.migrationFile`, CR-CV-057) hoặc tệp tạo bảng | `0` (ghi chú cấp tệp, "Scope: file") |
| Hợp đồng (proto/route/kênh) | `ContractChange.location.filePath` | `location.startLine` nếu có, ngược lại `0` |
| Phát hiện (`Finding`) | `locations[0].filePath` | `locations[0].startLine` hoặc `0` |
| Cụm/service/kho lưu trữ không có tệp rõ ràng | không có | nút "Ghi chú" **bị khoá** kèm lý do "Nút này không gắn với tệp nào; chọn một symbol/tệp bên trong" |

`buildGraphNoteBody` thêm **tiền tố cố định** giúp agent hiểu mà không cần thay đổi `shared/diff-comments-format.ts`: `[Review map · <lens> · <label>] <nội dung>`; ví dụ `[Review map · ERD · infra.dev_servers] Cột status đổi kiểu nhưng repository chưa cập nhật`. Tiền tố không chứa dữ liệu nhạy cảm (đi qua `maskSensitiveText` của CR-CV-058 nếu `label` là chuỗi tự do). Hợp đồng định dạng lời nhắc **giữ nguyên**: chúng ta không thêm trường vào `DiffComment` ở MVP (đổi `shared/types.ts` và định dạng sẽ lan sang annotation-service và mobile, chưa kiểm chứng).

Siêu dữ liệu neo (để bấm ghi chú nhảy về nút và để đếm huy hiệu theo nút) nằm ở **bản đồ neo** `anchorsByCommentId: Record<commentId, ReviewNoteAnchor>` lưu trong `review_states.notes` (README 3.5, O6), tải cùng `GetReviewState`. Nếu bản đồ thiếu mục cho một ghi chú, ghi chú vẫn tồn tại như ghi chú dòng thường (không làm mất dữ liệu). `notes` hiện chưa có lược đồ trong hợp đồng; đề xuất `{ anchors: Record<string, ReviewNoteAnchor>, sentBatches: ReviewSentBatch[] }` (xem "Điều chỉnh hợp đồng").

`diffIdentity` và `scope` của `DiffComment` **không** được đặt cho ghi chú đồ thị ở MVP (ý nghĩa với desktop chưa kiểm chứng; mobile dùng `diffIdentity` để đánh dấu ghi chú cũ, `mobile-diff-review-queue.ts` dòng ~154).

Thành phần (thư mục `components/review-map/notes/`):

```
ReviewNoteButton            (nút "Ghi chú" trong SymbolDetailPanel/ErdTableDetail/ContractChangeDetail/FindingRow; icon MessageSquarePlus)
ReviewNoteComposerPopover   (ui/popover + ui/textarea; Mod+Enter gửi lưu; dùng addDiffComment; tiêu đề hiện tệp:dòng thật sự sẽ gắn)
ReviewNodeNoteBadge         (huy hiệu số ghi chú trên nút xyflow qua selector, mọi lens dùng)
ReviewNotesPanel            (tab "Ghi chú" ở cột phải hoặc drawer; nhóm theo lens/neo; sửa, xoá, nhảy tới neo)
ReviewNotesSendMenu         (2.2)
ReviewSentBatchList         (2.3)
```

Wireframe `ReviewNotesPanel`:

```
Ghi chú review · 5 chưa gửi · 3 đã gửi lượt trước              [Gửi cho agent ▾]
 ▾ ERD · infra.dev_servers (1)
    ✎ Cột status đổi kiểu nhưng repository chưa cập nhật        migrations/…0042.up.sql   [Sửa][Xoá]
 ▾ Ảnh hưởng · DevServerRepository.Update (2)
    ✎ Thiếu kiểm tra tenant_id                                  adapter/postgres/…go:88-96
 ▾ Phát hiện · Vi phạm lớp (1)
 ▾ Không gắn nút (dòng diff thường) (1)
 ─ Đã gửi ở lượt trước (3) ▸  [xem]
```

Bấm ghi chú: chọn nút tương ứng ở lens của neo (`setReviewLens(worktreeId, lens)` và `setReviewSelectedSymbol`, CR-CV-051 2.8; CR-CV-053) và mở diff đúng dòng (cần CR-CV-053; `openDiff` hiện không có tham số dòng).

### 2.2 Gửi theo lô

`notes/ReviewNotesSendMenu.tsx` là bản bọc của **`NotesSendMenu<DiffComment>` (generic, đã export)**, không dùng `DiffNotesSendMenu` trực tiếp vì cần chạy thêm bước lưu lô đã gửi trước khi ghi chú bị xoá. Props: `worktreeId`, `groupId` (nhóm tab hiện tại), `notes: DiffComment[]` (tất cả ghi chú worktree), `selection?` (id đã chọn). Phạm vi (đối tượng `NotesSendMenuScope`):

| Scope id | Nhãn | `notes` | `prompt` |
|---|---|---|---|
| `all` | "Tất cả ghi chú chưa gửi" | mọi ghi chú `!sentAt` | **Dùng đúng** `useComposedAllNotesPrompt` (kết quả `annotation.composeReviewPrompt` ở runtime `environment`, nếu không thì `formatDiffComments`) để giữ cùng lời nhắc với Source Control |
| `lens` | "Ghi chú của <lens hiện tại>" | ghi chú có neo ở lens đang mở | `formatDiffComments` phía client |
| `selection` | "Đã chọn (N)" | các ghi chú đang tích chọn trong `ReviewNotesPanel` | `formatDiffComments` |

Lưu ý khớp hành vi hiện có: phạm vi không phải `all` **không** qua `annotation.composeReviewPrompt` (giống phạm vi "This file" của `DiffNotesSendMenu`, chú thích SOL-FE-ANNOTATE-002) và do đó **không** gọi `annotation.markSent`.

`onDelivered(notes)` thực hiện theo thứ tự: (1) `recordReviewSentBatch(...)` (2.3, không chặn); (2) `clearDeliveredDiffComments(worktreeId, notes)` như hiện nay; (3) `annotation.markSent` cho phạm vi `all` giống `markAnnotationsSentBestEffort`. Bước (3) cần dùng lại hàm hiện **riêng tư** trong `DiffNotesSendMenu.tsx`: trích ra module chung `lib/annotation-mark-sent-best-effort.ts` và để `DiffNotesSendMenu` import lại (không đổi hành vi). **Trước khi sửa `DiffNotesSendMenu`, chạy `gitnexus_impact` theo quy tắc của repo** (hiện có 3 nơi gọi: `CombinedDiffViewer`, `EditorPanelHeader`, `SourceControlInner`).

Đích gửi và bộ chọn agent hoàn toàn do `NotesSendMenu`/`ReviewNotesSendMenuContent` đảm nhiệm (agent đang chạy hợp lệ, hoặc "New agent"); CR này không viết lại, nên hưởng luôn kiểm tra sẵn sàng (`tui-idle`, quyền, lý do bị khoá) và hành vi qua SSH. `AgentSendPopoverTargetMode.source` hiện chỉ cho `'diff-notes' | 'browser-annotations'`: dùng `'diff-notes'` (không thêm giá trị mới ở MVP).

Hộp xem trước (tuỳ chọn trong menu "Xem lời nhắc…", `ReviewSendPreviewDialog.tsx`, `ui/dialog`): hiển thị số ghi chú, lời nhắc sẽ gửi (chỉ đọc), cảnh báo khi lô >50 ghi chú hoặc lời nhắc dài (giới hạn dán của terminal chưa kiểm chứng; đề xuất ngưỡng 50 chưa đo). Gửi vẫn đi qua menu để tái dùng đúng đường đi.

Trạng thái sau khi gửi: toast "Notes sent." hiện có của `ReviewNotesSendMenuContent`; thất bại dùng thông báo của `activeAgentNotesSendFailureMessage` (không viết lại). Các ghi chú đã giao biến mất khỏi danh sách chưa gửi (do `clearDelivered...`) và xuất hiện ở "Đã gửi ở lượt trước" nhờ bản ghi lô.

### 2.3 Bản ghi lô đã gửi (`ReviewSentBatch`)

```ts
type ReviewSentBatch = {
  batchId: string            // UUID
  sentAt: number
  turnId: string | null      // lượt đang xem khi gửi (2.4)
  targetPaneKey: string | null; agentType: string | null    // đích, nếu biết
  notes: { commentId: string; anchor: ReviewNoteAnchor; filePath: string;
           startLine?: number; lineNumber: number; body: string; fileIdentityAtSend?: string }[]
}
```

Lưu trong `review_states.notes.sentBatches[]` (lưu ý: chữ ký `reviewState.save` ở CR-CV-052 hiện chỉ có `readingProgress` và `expectedVersion`, **chưa có `notes`**; cần thêm vào hợp đồng, xem "Điều chỉnh hợp đồng") (giới hạn 10 lô gần nhất, mỗi lô ≤ 100 ghi chú; đề xuất). `fileIdentityAtSend` là `identity` tệp lúc gửi (2.4) dùng để đoán "tệp này có đổi sau khi gửi không". Nội dung `body` là chữ người dùng nhập (có thể chứa dữ liệu nhạy cảm do người dùng gõ): nó vốn đã đi tới annotation-service ở runtime `environment`; không gửi thêm nơi khác.

### 2.4 Mốc lượt và so sánh "lượt trước / lượt này"

**Sự kiện "agent xong".** Từ `agentStatusByPaneKey` (`store/slices/agent-status.ts`): chuyển sang `state === 'done'`; danh tính sự kiện `${paneKey}:${stateStartedAt}`; `interrupted === true` đánh dấu lượt bị ngắt. Cơ chế phát hiện, khử trùng lặp và chọn pane do CR-CV-061 định nghĩa (`agent-turn-completion.ts`); CR này **tiêu thụ** nó, không phát hiện lại.

**Ghi mốc.** `turns/use-review-turn-recorder.ts` (hook ở mức `ReviewWorkspace`, và cả khi tab Review chưa mở nếu cờ bật, vì mốc phải được ghi đúng lúc agent xong): khi có sự kiện xong, chờ trạng thái git ổn định (debounce, đề xuất ~3 s, chưa đo; `useGitStatusPolling` và `git-status-push-signal-refresh.ts` đã có cơ chế làm mới), rồi tạo `ReviewTurnMarker`:

```ts
type ReviewTurnMarker = {
  turnId: string                           // `${paneKey}:${doneAt}`
  worktreeId: string; paneKey: string; agentType: string | null
  startedAt: number                        // lần vào 'working' trước đó (stateHistory) hoặc null
  endedAt: number; interrupted?: boolean
  baseOid: string | null; headOid: string | null
  files: { p: string; o?: string; h: string }[]   // path, oldPath, identity băm (cap 500)
  symbolKeys?: string[]                    // SymbolRef.key đã đổi, từ ChangeOverlay (nếu có, cap 1500)
  overlayAvailable: boolean
}
```

`turn-file-identity.ts` (mới, hàm thuần) lấy cùng ý tưởng của `statusEntryIdentity/branchEntryIdentity` ở mobile: băm `status, oldPath, path, added, removed, area/scope` từ `GitStatusEntry`/`GitBranchChangeEntry` và `headOid/mergeBase`. **Đây là dấu vân tay thô**: hai lần sửa khác nhau cho cùng số dòng thêm/xoá sẽ trùng (bỏ sót thay đổi). Chấp nhận ở MVP và hiển thị nhãn "ước lượng"; nội dung thật cần băm nội dung tệp từ agent (RPC `fs.*`/`git.*` chưa kiểm chứng đủ).

Lưu: `codeIntelClient.call(worktreeId, 'codeIntel.reviewState.save', …)` với `reading_progress.turns[]` giữ tối đa 5 mốc gần nhất (đề xuất; kích thước ≈ 500 tệp × ~60 byte ≈ 30 KB/mốc, chưa đo). Ghi chú hợp đồng: `review_states` khoá theo `(worktree, base_commit, head_commit)` nên mốc của các lần commit khác nhau nằm ở các dòng khác nhau; đề xuất lưu `turns[]` ở một dòng cố định theo worktree (hoặc bảng riêng) — đưa vào "Điều chỉnh hợp đồng".

**Không lưu `prompt` của người dùng lên backend** (có thể chứa dữ liệu nhạy cảm); chỉ hiển thị trích đoạn lấy từ `entry.prompt` đang có trong bộ nhớ (nếu agent còn). Mốc cũ không còn entry thì hiện "Lượt <n> · <agent> · <thời điểm>".

**Giao diện chuyển lượt** (`turns/ReviewTurnSwitcher.tsx`, đặt cạnh bộ chọn phạm vi ở thanh đầu của khung Review, `ui/toggle-group` + `ui/popover`):

```
Lượt: [ Lượt này ▾ ]  ( ● Toàn bộ thay đổi  ○ Chỉ phần đổi từ lượt trước  ○ Xem lượt trước )
        Lượt 3 · Claude · 14:32  ← hiện tại
        Lượt 2 · Claude · 14:10  (đã gửi 3 ghi chú lúc 14:12)
        Lượt 1 · Claude · 13:41
```

Ba chế độ:

1. **Toàn bộ thay đổi** (mặc định): hành vi bình thường của Review (phạm vi O7).
2. **Chỉ phần đổi từ lượt trước**: `turn-compare-model.ts` (hàm thuần) so sánh mốc hiện tại với mốc trước, gán cho từng tệp/symbol: `new_in_turn` (không có ở mốc trước), `changed_in_turn` (identity khác), `unchanged_since` (giống), `reverted_in_turn` (có ở mốc trước, không còn nay). Các lens dùng nhãn này như một lớp phủ (cùng ký hiệu và chú giải nhất quán, 10 §6.3): "Thứ tự đọc" (CR-CV-052) lọc được về `new_in_turn|changed_in_turn`, đồ thị làm mờ nút `unchanged_since`. Symbol cấp độ chi tiết chỉ khi cả hai mốc có `symbolKeys` (`overlayAvailable`); nếu không thì chỉ mức tệp và nói rõ.
3. **Xem lượt trước**: chỉ đọc, hiển thị danh sách tệp/symbol của mốc đó và các lô ghi chú đã gửi lúc ấy. **Không dựng lại nội dung diff** của lượt trước khi thay đổi chưa commit (không có ảnh chụp); nếu hai mốc có `headOid` khác nhau và tồn tại luồng so sánh commit, mở diff theo commit (khả năng này chưa kiểm chứng, phụ thuộc CR-CV-053).

**Phản hồi có được xử lý không** (giá trị chính): với mỗi ghi chú trong lô đã gửi ở lượt trước, so `fileIdentityAtSend` với identity hiện tại của cùng tệp, hiển thị nhãn trên dòng ghi chú: "Tệp đã đổi từ khi gửi" hoặc "Chưa thấy đổi ở tệp này". Đây là **gợi ý, có thể sai** (dấu vân tay thô; agent có thể xử lý ở tệp khác) và không được hiển thị như "đã xử lý"; nút để người dùng tự đánh dấu "Đã xử lý" (cùng cơ chế `dismissFinding` cho phát hiện, còn ghi chú thì chỉ cờ cục bộ trong `ReviewSentBatch`).

**Trường hợp không có dữ liệu**: chưa có mốc trước (lượt đầu, hoặc bật tính năng sau khi agent đã chạy) → bộ chuyển lượt hiện "Chưa có lượt trước để so sánh" và vô hiệu hoá chế độ 2–3; không bịa mốc từ `stateHistory` (không có ảnh chụp mã).

### 2.5 Trạng thái, phím tắt, i18n, hai render target

| Tình huống | Hiển thị |
|---|---|
| Lưu ghi chú | Vô hiệu nút ngay, popover đóng khi `addDiffComment` thành công (giữ bản nháp nếu trả `null`, như `DiffViewer`); độ trễ SSH: hiển thị trạng thái lưu sau ~200 ms |
| Không có agent đang chạy để gửi | Do `ReviewNotesSendMenuContent` (hàng bị khoá kèm lý do; "New agent") |
| Ghi mốc lượt thất bại | Dòng trạng thái inline "Không lưu được mốc lượt, so sánh lượt trước có thể thiếu" kèm "Thử lại"; không chặn gửi ghi chú |
| Dev server offline | Ghi chú vẫn lưu cục bộ (lớp lưu bền hiện có); lưu `review_states` bị hoãn, đánh dấu cũ |

Phím tắt trong khung Review (đăng ký qua registry của CR-CV-052; chỉ hiển thị khi đã cài; `metaKey` trên Mac, `ctrlKey` nơi khác): `n` mở soạn ghi chú cho nút đang chọn (khi tiêu điểm không ở ô nhập), `Mod+Enter` lưu ghi chú trong popover (`isScreenSubmitShortcut`, nhãn `ShortcutKeyCombo`). Chuỗi qua `translate()`, khoá tiền tố `auto.components.review.map.notes.` và `auto.components.review.map.turns.` đủ 5 locale `en/es/ja/ko/zh`. Màu chỉ qua token; huy hiệu ghi chú dùng token accent/muted; reduced-motion tắt animation chuyển lens. Chỉ đi qua `codeIntelClient`/`useCodeIntelQuery` và các hàm store/lib sẵn có nên chạy ở Electron và web; **không** dùng `window.api` trực tiếp trong component.

## 3. Quyết định thiết kế

- **Ghi chú đồ thị là `DiffComment`**, không có kho ghi chú song song: tái dùng lưu bền, đồng bộ, đếm ở Source Control và đường gửi; đổi lại phải chấp nhận ràng buộc "ghi chú phải gắn tệp".
- **Neo ở `review_states.notes` (O6), tiền tố ở nội dung**: không đổi `shared/types.ts`/định dạng lời nhắc nên không ảnh hưởng annotation-service và mobile.
- **Bọc `NotesSendMenu` generic thay vì `DiffNotesSendMenu`**: cần chèn bước lưu lô trước khi ghi chú bị xoá; vẫn tái dùng toàn bộ chọn đích/gửi/kiểm tra sẵn sàng.
- **Mốc lượt ghi lúc agent xong, không suy ngược từ `stateHistory`**: `stateHistory` chỉ trong bộ nhớ và không có ảnh chụp mã.
- **Dấu vân tay thô được dán nhãn "ước lượng"**, không khẳng định "đã xử lý".
- **Không gửi `prompt` của người dùng lên backend** vì có thể chứa dữ liệu nhạy cảm.

## 4. Tiêu chí chấp nhận

- [ ] Từ nút symbol, bảng ERD, thay đổi hợp đồng và phát hiện, người dùng tạo được ghi chú; ghi chú được lưu bằng `addDiffComment` với `filePath/startLine/lineNumber` theo bảng 2.1 và nội dung có tiền tố `[Review map · …]`.
- [ ] Nút không gắn tệp (cụm/service/kho) có nút "Ghi chú" bị khoá kèm lý do; không tạo ghi chú rỗng neo.
- [ ] Ghi chú đồ thị xuất hiện ở số đếm Source Control và đi trong lô "All unsent notes" của `DiffNotesSendMenu`.
- [ ] `ReviewNotesPanel` nhóm theo lens/neo, sửa/xoá được, bấm nhảy tới nút (và diff khi CR-CV-053 hỗ trợ); sửa ghi chú đã gửi đưa nó trở lại hàng chờ.
- [ ] `ReviewNotesSendMenu` có phạm vi `all`, `lens`, `selection`; phạm vi `all` dùng cùng lời nhắc với Source Control (`useComposedAllNotesPrompt`); chọn đích agent và "New agent" dùng nguyên `ReviewNotesSendMenuContent`.
- [ ] Gửi thành công: lưu `ReviewSentBatch` rồi `clearDeliveredDiffComments`; `annotation.markSent` chỉ cho phạm vi `all` ở runtime `environment`; hành vi của `DiffNotesSendMenu` không đổi (có test hồi quy).
- [ ] Agent báo xong → ghi một `ReviewTurnMarker` (trong ≈ debounce) khi cờ bật; khử trùng lặp theo `turnId`; giữ tối đa 5 mốc; không lưu `prompt`.
- [ ] `ReviewTurnSwitcher` có ba chế độ; chế độ 2 gán đúng `new/changed/unchanged/reverted` cho tệp (và symbol khi hai mốc có `symbolKeys`) và lớp phủ nhất quán ở các lens; chế độ 3 chỉ đọc và không giả vờ dựng lại diff.
- [ ] Khi chưa có mốc trước, chế độ 2–3 bị vô hiệu hoá với thông báo "Chưa có lượt trước để so sánh".
- [ ] Nhãn "tệp đã đổi từ khi gửi/chưa thấy đổi" hiển thị như gợi ý, có chú giải "ước lượng".
- [ ] Mọi chuỗi qua `translate()` đủ 5 locale; không hex cứng; `Mod+Enter` đúng nền tảng; không dùng `components/code-review/*`; không thêm thư viện; không thêm `max-lines` disable; chạy ở Electron và web.

## 5. Kiểm thử

Vitest + Testing Library theo mẫu repo (các test slice tham khảo `store/slices/diffComments.test.ts`):

- Hàm thuần: `review-note-anchor` (ánh xạ từng loại nút, `lineNumber` 0 cho cấp tệp, nút không gắn tệp trả `null`, tiền tố, `maskSensitiveText`), `turn-file-identity` (ổn định, khác khi đổi đường dẫn/số dòng/`headOid`), `turn-compare-model` (bốn nhãn, symbol thiếu một phía, giới hạn cap), `review-sent-batch` (trần số lô/ghi chú).
- Component: `ReviewNoteComposerPopover` (lưu, giữ nháp khi lỗi, `Mod+Enter`), `ReviewNotesPanel` (nhóm, sửa đưa `sentAt` về trống, nhảy tới neo), `ReviewNotesSendMenu` (phạm vi đúng, lời nhắc đúng, gọi `recordReviewSentBatch` trước `clearDeliveredDiffComments` bằng giả lập `useAppStore`/`callRuntimeRpc`), `ReviewTurnSwitcher` (vô hiệu khi chưa có mốc trước, ba chế độ).
- Hồi quy: test hiện có quanh `DiffNotesSendMenu`/`NotesSendMenu` (`ReviewNotesSendMenuContent.test.tsx` đã có) vẫn xanh sau khi trích `markAnnotationsSentBestEffort`.
- Slice: mốc lượt bị xoá khi xoá worktree (mẫu `*-worktree-purge-leak.test.ts`); khử trùng lặp sự kiện xong; không ghi mốc khi cờ tắt.
- Phối hợp CR-CV-061: dùng fixture `AgentStatusEntry` như `DashboardAgentRow.test.tsx` cho chuyển `working → done`.
- E2E (cần backend CR-CV-011/036, và một agent thật hoặc giả): gửi lô, thấy lô ở "Đã gửi ở lượt trước", chạy lượt kế tiếp, thấy nhãn so sánh. **Chưa chạy**, đây là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Không có ảnh chụp mã theo lượt**: so sánh chỉ ở mức dấu vân tay thô (có thể trùng khi cùng số dòng thêm/xoá, bỏ sót thay đổi). Muốn so chính xác cần băm nội dung hoặc ref git; chưa kiểm chứng RPC `fs.*`/`git.*` nào cho phép rẻ, và tạo ref trong repo người dùng là xâm lấn (cần xem `guides/reference/git-compatibility.md`).
- **Nhiều agent trong một worktree**: lượt tính theo `paneKey`; chưa chốt "lượt trước" của worktree khi hai agent xen kẽ (đề xuất: ưu tiên cùng pane, nếu không thì mốc liền trước).
- **Mốc chỉ ghi khi renderer đang chạy**: nếu người dùng đóng ứng dụng/tab web lúc agent xong, mốc đó mất; khoảng trống này không tự phục hồi.
- **`review_states.notes` và `reading_progress.turns[]` chưa có lược đồ** trong README 3.5; kích thước 30 KB/mốc và trần 5/10 là ước lượng chưa đo.
- **Ghi chú đã gửi bị xoá bởi `clearDeliveredDiffComments`**: nếu lưu `ReviewSentBatch` thất bại thì lịch sử mất; chấp nhận và báo inline.
- **Trích `markAnnotationsSentBestEffort`** chạm `DiffNotesSendMenu` (dùng ở 3 nơi); cần chạy impact và test hồi quy khi triển khai.
- **`AgentSendPopoverTargetMode.source`** chưa có giá trị riêng cho review; dùng `'diff-notes'` có thể làm số liệu phân loại nhầm (chưa kiểm tra telemetry).
- **`annotation.composeReviewPrompt` chỉ ở runtime `environment`**: ở runtime `local` không có ngữ cảnh ±2 dòng (đã đúng như hiện nay); Review yêu cầu backend nên dự kiến luôn là `environment`, nhưng chưa xác minh với chế độ Electron nối backend.
- Độ dài lời nhắc/giới hạn dán của terminal chưa kiểm chứng.

## 7. Câu hỏi mở

1. Có cho thêm trường `graphRef` tuỳ chọn vào `DiffComment` (và định dạng lời nhắc) để agent nhận được neo có cấu trúc thay vì tiền tố chữ không? Đổi `shared/types.ts` ảnh hưởng annotation-service và mobile.
2. Có cần lời mở đầu cho lô ("Review của <worktree> so với <base>: N ghi chú") hay giữ nguyên định dạng hiện tại?
3. "Lượt" tính theo từng agent hay theo worktree? Nếu theo worktree, mốc tạo khi **mọi** agent đều `done`?
4. Mốc lượt có nên lưu cả ở máy khách (không cần backend) khi dev server offline?
5. Có cho "Đã xử lý" thủ công cho ghi chú đã gửi và đồng bộ với `dismissFinding` khi ghi chú gắn vào phát hiện?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.5 `review_states`, 3.7 kênh `codeIntel.reviewState.*`, O6, O7)
- `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§6.5, §6.6, §6.8)
- `/opt/repos/orca/frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx`, `NotesSendMenu.tsx`, `ReviewNotesSendMenuContent.tsx`, `ReviewNotesSendMenuContent.test.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/diffComments.ts`, `diff-comments-annotation-sync.ts`, `diff-comments-mutations.ts`, `diff-comments-persist-queue.ts`
- `/opt/repos/orca/frontend/src/renderer/src/lib/use-composed-all-notes-prompt.ts`, `diff-comments-format.ts`, `active-agent-note-send.ts`, `notes-send-agent-targets.ts`
- `/opt/repos/orca/frontend/src/shared/diff-comments-format.ts`, `shared/types.ts` (`DiffComment`, `MobileDiffReviewState`, `GitBranchCompareResult`), `shared/git-status-types.ts`, `shared/agent-status-types.ts`
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/agent-status.ts`, `store/slices/ui.ts` (`AgentSendPopoverTargetMode`), `store/slices/editor.ts` (`openDiff`, `gitBranchCompareSummaryByWorktree`)
- `/opt/repos/orca/frontend/src/renderer/src/components/editor/DiffViewer.tsx` (`handleSubmitComment`), `components/right-sidebar/SourceControl.tsx`
- `/opt/repos/orca/desktop/src/main/agent-hooks/server.ts` (`last-status.json`)
- `/opt/repos/orca/mobile/src/session/mobile-diff-review-queue.ts` (`statusEntryIdentity`, `branchEntryIdentity`), `mobile/src/session/mobile-diff-review-state.ts`
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- Mới: `components/review-map/notes/{ReviewNoteButton,ReviewNoteComposerPopover,ReviewNodeNoteBadge,ReviewNotesPanel,ReviewNotesSendMenu,ReviewSendPreviewDialog,ReviewSentBatchList}.tsx`, `review-note-anchor.ts`; `components/review-map/turns/{ReviewTurnSwitcher}.tsx`, `use-review-turn-recorder.ts`, `turn-file-identity.ts`, `turn-compare-model.ts`; `lib/annotation-mark-sent-best-effort.ts`
