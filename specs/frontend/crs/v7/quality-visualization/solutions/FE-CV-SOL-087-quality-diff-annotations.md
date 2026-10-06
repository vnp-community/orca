# FE-CV-SOL-087-quality-diff-annotations: Chú thích phát hiện trên diff Monaco, danh sách phát hiện kiểm tra, miễn trừ

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06 từ khảo sát code `frontend/src`; chưa chạy test hay ứng dụng. Solution 2/3 của CR-CV-087 ([1](./FE-CV-SOL-087-quality-scorecard-and-state.md), [3](./FE-CV-SOL-087-quality-trend-coverage-hotspot.md)).

**CR:** [CR-CV-087](../../../../../../docs/crs/v7/quality-visualization/CR-CV-087-quality-frontend-scorecard-and-annotations.md) mục 1.1, 2.6, 2.7
**Area:** frontend (`components/editor/`, `components/editor/quality-annotations/`, `components/review-map/quality/findings/`)
**Làm sau:** FE-CV-SOL-087-quality-scorecard-and-state (kiểu, state, hook), FE-CV-SOL-088 (SeverityBadge), FE-CV-SOL-053 (`pendingDiffReveal`, mở diff đúng dòng), FE-CV-SOL-059 (dock Phát hiện).
**TDD tham chiếu:** [v5/08-editor-and-files](../../../../tdd/v5/08-editor-and-files.md) (chỉ bối cảnh; TDD nói Orca không embed Monaco trực tiếp, **mâu thuẫn với code**: `DiffViewer.tsx` dùng `@monaco-editor/react`), [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/02-state-management](../../../../tdd/v5/02-state-management.md).

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

| Mục hợp đồng | Dùng cho |
|---|---|
| `CONTRACT-codeintel-ui-api.md` §3.2 `quality.findings {runId?, severities?, categories?, file?, inScope?, limit≤500, pageToken?}` → `{findings, totalCount, truncated, outsideScopeCount, nextPageToken}` | danh sách, chỉ mục theo tệp |
| §3.2 `quality.waive {subjectKind:'finding', subjectKey=fingerprint, action:'waive'|'revoke', reason 1..1000, expiresAt ≤ 30 ngày, scope?}` → `{waiver}`; quyền `quality_waive`; `CODEINTEL_WAIVER_EXPIRY_INVALID {maxDays}` | miễn trừ / bỏ miễn trừ |
| §4.7 `QualityFinding`: `fingerprint`, `fpVersion`, `ruleId`, `severity`, `category`, `file`, `line`, `endLine`, `column`, `endColumn`, `message`, `tool`, `toolVersion`, `fixHint?`, `stepId`, `inScope`, `waiver?`; `QualityRun.headCommit/dirty/workTreeChangedDuringRun` | marker, bảo vệ lệch dòng |
| PQ-05 (waiver tách hẳn dismissal; `Dismiss` không miễn cổng), PQ-06 (hai nguồn, một dock, không trộn), PQ-26 (`ruleId` regex `^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`, hiển thị nguyên văn mã lạ) | dock |
| U9, §4.7 cấm overclaim | message/fixHint văn bản thuần |
| §2.4 giới hạn `reason` ≤ 500 chuỗi chung; `reason` waive 1..1000 | form |

**Lệch giữa CR và hợp đồng:**

| # | CR-087 | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `QualityFinding` không có cột; chú thích cả dòng | Có `column`, `endColumn` | Marker chính xác cột khi `column ≥ 1` và `endColumn ≥ column`; ngược lại cả dòng. Quy ước 0/1-based của cột **không nêu** (câu hỏi mở 1) → giả định 1-based như `line`, ghi chú "chưa kiểm chứng" |
| 2 | "Chỉ tệp đã đổi" lọc bằng `ChangeOverlay.changedFiles` | `quality.findings.inScope` + `QualityFinding.inScope` + `outsideScopeCount` | Lọc phía server bằng `inScope` |
| 3 | Phân trang `offset`; waive `{fingerprint, note}`; 90 ngày | `pageToken`; `subjectKind/subjectKey`; ≤ 30 ngày; có `revoke` | Theo hợp đồng; chọn hạn 7/14/30 ngày hoặc ngày cụ thể ≤ 30; "Bỏ miễn trừ" thật |
| 4 | Dòng đã miễn trừ do backend gắn (đề xuất) | `QualityFinding.waiver?` có sẵn | Dùng |
| 5 | Chỉ `DiffViewer.tsx` | `useDiffCommentDecorator` thật được gọi ở **ba** nơi: `DiffViewer.tsx:110`, `DiffSectionItem.tsx:176` (diff gộp/PR), `MonacoEditor.tsx:229`; `DiffViewer` dùng ở `EditorContent.tsx:964`, `ChangesModeView.tsx:87`, `ExternalFileChangeCompareDialog.tsx:145` (không `worktreeId`) | Hook marker dùng được cho cả `DiffViewer` và `DiffSectionItem` (cùng `modifiedEditor`); không gắn `MonacoEditor` ở MVP; **chỉ vẽ khi phía modified khớp nội dung worktree** (quy tắc đủ điều kiện, mục 2.3) |
| 6 | "Chú thích xoá khi nội dung đổi" dựa trên `headCommit` | `QualityRun.dirty`, `workTreeChangedDuringRun` | Dùng cả hai làm cảnh báo mềm |
| 7 | Phím tắt `w`, `n`, `j/k` | Chưa có registry phím chốt | Chỉ cài phím khi đã thống nhất với FE-CV-SOL-052/059; mặc định chỉ `Enter` và nút |

**Phụ thuộc chéo khu vực:** `BE-CV-SOL-040-codeintel-quality-channels`, `BE-CV-SOL-082-quality-run-storage-and-ingest` (fingerprint, `inScope`), `BE-CV-SOL-085-waivers-and-trend` (waive/revoke; hết hạn tự trả lại phát hiện), `AG-CV-SOL-082-quality-parsers-and-fingerprint`; FE: `FE-CV-SOL-053-impact-lens-and-symbol-detail` (lõi mở diff đúng dòng; đề nghị tách `openReviewDiffAtPath(worktreeId, relativePath, line, scope)`), `FE-CV-SOL-059-contract-lens-and-findings` (`FindingsToolbar`, ô nguồn), `FE-CV-SOL-060-review-notes-and-turn-compare` (luồng ghi chú; neo `findingKey`/`fingerprint`), `FE-CV-SOL-052` (đăng ký phím).

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc: `components/editor/DiffViewer.tsx` (props :36-49; `modifiedEditor` state :82; `useDiffCommentDecorator` :110; `handleMount` :318 đặt `setModifiedEditor`; `DiffEditor` :444-478 với `options` **không** có `glyphMargin`, có `renderOverviewRuler: true`, `keepCurrentModifiedModel`), `components/editor/DiffSectionItem.tsx` (props `section`, `worktreeId`; `useDiffCommentDecorator` :176 với `filePath: section.path`), `components/editor/EditorContent.tsx` (:964 `<DiffViewer ... relativePath={activeFile.relativePath} worktreeId={activeFile.worktreeId}>`), `ChangesModeView.tsx` (:87), `ExternalFileChangeCompareDialog.tsx` (:145, không `worktreeId`), `store/slices/editor.ts` (:100 `DiffSource`, :232 `diffSource?`), `components/editor/MonacoEditor.tsx` (:735-760 `createDecorationsCollection`), `components/editor/check-annotation-open.ts` + `check-annotation-path.ts` (`resolveAnnotationPathInsideWorktree`, `getOpenableAnnotationLine`), `components/editor/CheckRunAnnotations.tsx` (sibling: CI check annotations), `components/diff-comments/useDiffCommentDecorator.tsx` (733 dòng, `eslint-disable max-lines` :1-3 từ trước; import `* as monaco from 'monaco-editor'`), `lib/monaco-setup` (`DiffViewer` import `{ monaco }` từ `@/lib/monaco-setup`).

**Xác nhận:** grep `setModelMarkers` rỗng trong `frontend/src` (cơ chế mới với repo); `glyphMargin` chỉ có ở `IpynbViewer.tsx:415` (đặt `false`); view zone của `useDiffCommentDecorator` đẩy nội dung (không phù hợp trăm phát hiện).

**`DiffSource`** (`editor.ts:100`): `unstaged|staged|branch|commit|combined-all|combined-uncommitted|combined-branch|combined-commit`. **Chưa kiểm chứng** phía `modified` của từng loại: `unstaged` = nội dung worktree (đủ điều kiện hợp lý); `staged` = index; `commit` = một commit; `branch` = head của so sánh nhánh. Task 087-09 phải đọc nơi dựng `dc.modifiedContent` rồi chốt bảng đủ điều kiện.

**Correction relative to CR-087:** bảng "Lệch" #1-#7.

## 2. Giải pháp

### 2.1 Cây file

```
components/editor/quality-annotations/                       (mới)
  quality-marker-model.ts           buildQualityMarkers(findings, {lineCount, lineMaxColumn}) -> {markers, glyphs}
  quality-annotation-eligibility.ts isAnnotationEligible({diffSource, run, fileDirty, compareHeadOid})
  useQualityFindingMarkers.ts       hook: setModelMarkers (owner 'orca-quality') + decorations glyph
  quality-annotation-notice.ts      trạng thái "đã ẩn vì nội dung đổi / có thể lệch"
components/editor/DiffViewer.tsx, DiffSectionItem.tsx         (sửa: một lời gọi hook mỗi file)
assets/main.css                                               (sửa: lớp .orca-quality-glyph-{error,warning,info})
components/review-map/quality/findings/                       (mới)
  QualityFindingsList.tsx  QualityFindingRow.tsx  QualityFindingsToolbar.tsx  QualityWaivePopover.tsx
  quality-finding-filter.ts  quality-finding-sort.ts  quality-waive-expiry-options.ts
hooks/useQualityFindings.ts, useQualityFindingsForFile.ts, useQualityWaive.ts              (mới)
```

### 2.2 Marker và glyph

`buildQualityMarkers` thuần: ánh xạ `error→MarkerSeverity.Error`, `warning→Warning`, `info→Info`, `unknown→Hint` (dùng enum `monaco.MarkerSeverity`, không số); vùng `[line,endLine]` kẹp `[1,lineCount]` (`endLine<line` → `line`); cột chỉ khi hợp lệ (`1 ≤ column ≤ lineMaxColumn`, `endColumn ≥ column`) nếu không cả dòng; mức tệp (`line≤0`) đặt dòng 1; `message` = `message` + (`fixHint` nếu có), `source = tool`, `code = ruleId`; owner cố định `'orca-quality'`. Glyph: `glyphMarginClassName: 'orca-quality-glyph-{error|warning|info}'` với hình khác nhau bằng `clip-path` (bát giác/tam giác/tròn), màu `var(--quality-*)`; `glyphMarginHoverMessage` văn bản thuần. `glyphMargin` bật bằng `updateOptions` chỉ khi có phát hiện (hành vi mặc định ở diff editor **chưa kiểm chứng**).

### 2.3 Hook và vòng đời

`useQualityFindingMarkers({editor, worktreeId, relativePath, diffSource, enabled})`: chỉ phía `modified`; chỉ khi `useQualitySupport()==='enabled'`, `annotationsOn`, có dữ liệu cho tệp (`useQualityFindingsForFile`), và `isAnnotationEligible`; `monaco.editor.setModelMarkers(model, 'orca-quality', markers)` + `createDecorationsCollection`. **Xoá toàn bộ ngay khi `onDidChangeModelContent`** (marker không tự dời theo chỉnh sửa) và hiển thị ghi chú "Chú thích kiểm tra đã ẩn vì nội dung đã đổi; chạy lại để cập nhật"; dọn khi gỡ hook, đổi model (`modelKey`), tắt công tắc. Cảnh báo mềm (chip trong Tooltip, vẫn vẽ) khi `run.dirty` hoặc `workTreeChangedDuringRun`; không vẽ khi `run.headCommit` khác HEAD hiện tại. Bấm glyph chọn phát hiện ở dock (`setQualityUi({selectedFingerprint})`) qua `editor.onMouseDown` (đích `GUTTER_GLYPH_MARGIN`, **chưa kiểm chứng** trong `DiffEditor`); không dựng popover riêng. F8/Shift+F8 mặc định Monaco cho marker: chưa kiểm chứng trong `DiffEditor`, không hiển thị chip phím. Không đặt màu overview ruler riêng (Monaco không hiểu `var()`); không sửa `useDiffCommentDecorator`.

`useQualityFindingsForFile(worktreeId, relativePath)`: dùng danh sách đã nạp nếu đầy đủ (không `truncated`, không `nextPageToken`); nếu không, gọi `quality.findings {file, runId, limit:500}` theo tệp (cache nhỏ ≤ 16 tệp/worktree, thuộc `codeIntelQualityByWorktree`); mảng ổn định.

### 2.4 Danh sách phát hiện kiểm tra

Gắn vào dock của FE-CV-SOL-059 khi nguồn = "Kiểm tra" (`ToggleGroup` "Cấu trúc | Kiểm tra" ở `FindingsToolbar`); số đếm hai nguồn **tách**, không trộn. Ảo hoá `@tanstack/react-virtual` > 50 hàng (`getItemKey = fingerprint`, `estimateSize` 44); nạp bằng `pageToken` (limit 500), trần 5 000, "Đang hiển thị X/Y" + "Tải thêm". Lọc phía server: `severities`, `categories`, `inScope`, `file`; lọc cục bộ: tìm chữ trong `message|ruleId|file`, "hiện đã miễn trừ". Sắp: mức → category → file → line → `fingerprint`.

```
Lọc: [Mức: ⊗Lỗi ▲Cảnh báo ○Thông tin] [Loại ▾] [☐ Chỉ trong phạm vi đã đổi] [☐ Hiện đã miễn trừ N] 🔍
 ⊗ oxlint/no-unused-vars  'x' khai báo nhưng không dùng     src/a/b.ts:42      oxlint 1.2  [Xem diff][Miễn trừ ▾]
 ▲ tsc/TS2322             Kiểu 'string' không gán được...    src/a/c.ts:7-9      tsc 5.x     …
   Gợi ý sửa: … (fixHint, thu gọn)
 Đang hiển thị 500/1 203 · 12 phát hiện ngoài phạm vi (không tính vào cổng)         [Tải thêm]
```

"Xem diff" gọi lõi mở diff theo đường dẫn của FE-CV-SOL-053 (`openReviewDiffAtPath`; nếu chưa có, dùng `openAnnotationLocation` như `CheckRunAnnotations`); ngoài thay đổi → "Mở tệp"; đường dẫn qua `resolveAnnotationPathInsideWorktree`. `ruleId` lạ hiển thị nguyên văn. Phát hiện mức tệp (`line≤0`) không số dòng.

### 2.5 Miễn trừ

`QualityWaivePopover`: `reason` bắt buộc (1..1000; đếm ký tự), `expiresAt` bắt buộc (7/14/30 ngày hoặc ngày ≤ 30 ngày), `scope` không hiển thị (mặc định backend); khoá nút ngay, cập nhật lạc quan, hoàn nguyên và báo inline khi lỗi (`forbidden`, `offline`, `conflict`, `validation` + `maxDays`); `Mod+Enter` (`isScreenSubmitShortcut`; chip `ShortcutKeyCombo`); dòng đã miễn trừ ẩn mặc định, hiển thị người/hạn/lý do khi bật, có "Bỏ miễn trừ" (`action:'revoke'`). Miễn trừ **không** làm đổi số trên scorecard cho tới khi tải lại `gate` (backend quyết định), nên sau thành công gọi `invalidateQuality` + `loadQualityGate`; không tự tính lại cổng ở client.

## 3. Quyết định thiết kế

- Marker (chính) + glyph hình học; không view zone; không sửa file 733 dòng.
- Đủ điều kiện theo `DiffSource` vì số dòng là của worktree, không của mọi phía modified.
- Gắn cả `DiffViewer` và `DiffSectionItem` (diff gộp là nơi Review dùng).
- Lọc `inScope` ở server; phân trang bằng token.
- Hai nguồn trong một dock, không trộn (PQ-06).

## 4. Tiêu chí chấp nhận

- [ ] Marker owner `'orca-quality'` với `source`/`code`; glyph hình dạng khác nhau theo mức; chỉ phía modified; chỉ khi đủ điều kiện; xoá khi nội dung đổi hoặc tắt; dọn khi gỡ hook; không phát sinh khi chưa có lần chạy/`annotationsOn` tắt; không đổi `useDiffCommentDecorator`.
- [ ] Danh sách: lọc, sắp ổn định, ảo hoá > 50, nạp theo trang, trần 5 000 "X/Y", `outsideScopeCount` hiển thị; "Xem diff" đúng dòng; đường dẫn thoát worktree bị chặn.
- [ ] Miễn trừ bắt buộc lý do + hạn ≤ 30 ngày; lạc quan + hoàn nguyên; revoke hoạt động; `Mod+Enter` theo nền tảng.
- [ ] Dock có điều khiển nguồn; số đếm tách biệt.
- [ ] Không hex; `translate()` đủ 5 locale; Electron và web; không `max-lines` disable; không `components/code-review/*`; không dependency mới.

## 5. Kiểm thử

Vitest + Testing Library; `pnpm --filter orca-frontend test -- <đường dẫn>`. **Chưa chạy.** Thuần: `quality-marker-model` (ánh xạ mức, kẹp dòng, cột hợp lệ/không, mức tệp, `endLine<line`, `unknown→Hint`), `quality-annotation-eligibility` (mọi `DiffSource`), filter/sort, expiry options. Hook với editor/model giả (`setModelMarkers`, `createDecorationsCollection`, `onDidChangeModelContent`): đặt, xoá, dọn, khoá owner, lệch HEAD, `dirty`. `DiffViewer`/`DiffSectionItem` test hiện có không vỡ. Component: list (số hàng DOM < tổng; "X/Y"), row, toolbar nguồn, waive popover (bắt buộc lý do/hạn, lạc quan, hoàn nguyên, revoke). Kiểm tay: F8, glyph sáng/tối, `ExternalFileChangeCompareDialog` không có chú thích.

## 6. Rủi ro và điểm chưa kiểm chứng

1. `glyphMargin` mặc định, F8, `onMouseDown` glyph, hiển thị marker với `modifiedModelPath` + `keepCurrentModifiedModel` trong `DiffEditor`: chưa kiểm chứng; rút lui: decoration thuần (`className` nền dòng).
2. Chi phí `setModelMarkers` với ~1 000 marker chưa đo.
3. Ngữ nghĩa phía modified của từng `DiffSource` (2.3).
4. Hover marker Monaco hiển thị `message` dạng văn bản thuần: chưa kiểm chứng.
5. Hai nguồn có thể mô tả cùng vấn đề (`architecture`/`convention` so `layer_violation`): hiển thị cả hai kèm nhãn nguồn, không khử trùng.

## 7. Câu hỏi mở

1. Hợp đồng: cột 0- hay 1-based; `column=0` nghĩa là gì?
2. Phím tắt dock (`j/k`, `w`) cần registry chung của 052/059.
3. `scope` của waiver (`repo|binding`) có cần chọn trong UI?
4. Marker có nên hiện ở diff `staged`/`commit` bằng cách ánh xạ dòng (cần diff giữa worktree và phía modified): ngoài phạm vi.

## 8. Tasks

[FE-CV-TASK-087-09](../tasks/FE-CV-TASK-087-09-quality-marker-model-and-annotation-eligibility.md) đến [087-14](../tasks/FE-CV-TASK-087-14-quality-waive-and-revoke.md).
