# FE-CV-SOL-056-dataflow-lens: Lens Luồng dữ liệu (sơ đồ tuần tự Mermaid, danh sách bước, sao chép/xuất)

> 🚧 **In Progress.** Trạng thái (cập nhật 2026-10-08): 6/7 task DONE; PARTIAL 0. Chi tiết ở dòng `**Status:**` và "Ghi chú hoàn thiện" của từng task.

**CR:** [CR-CV-056](../../../../../../docs/crs/v7/review-frontend/CR-CV-056-dataflow-lens.md)
**Area:** frontend (`components/review-map`, `hooks`)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (§3.1 `dataFlows`, `dataFlow`; §4.4 `DataFlow`, `DataFlowStep`, `DataFlowSummary`, `SequenceModel`, `DfdModel`, `ComponentRef`, `StoreRef`; §2.3 `SYMBOL_NOT_FOUND`, `RESPONSE_TOO_LARGE`; U9), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-29, PQ-12, PQ-14; §8.2 CR-056).
**TDD tham chiếu:** [v5/05-ui-components §3 Major Screens, §6 UI Library](../../../../tdd/v5/05-ui-components.md), [v5/08-editor-and-files §3 Editor Component](../../../../tdd/v5/08-editor-and-files.md) (Markdown/Mermaid preview), [v5/07-hooks-and-ipc §15 Key Hook Patterns](../../../../tdd/v5/07-hooks-and-ipc.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `components/editor/MermaidBlock.tsx` (props `{content, isDark, htmlLabels?}` :22-24; `mermaid.initialize(getMermaidConfig(isDark, htmlLabels))` :73; `mermaid.render` :74; hiển thị mã khi lỗi :110), `components/editor/mermaid-config.ts` (:9 `securityLevel:'strict'`), `preload/api-types.ts:3216` (`ui.writeClipboardText`), `components/settings/mcp/mcp-audit-csv.ts` (tên, tiền lệ tải tệp), `hooks/` (**không có** `useIsDarkTheme`), `package.json` (`mermaid` ^11.15.0, `dompurify` ^3.4.11). `MAX_TEXTLENGTH = 5e4` ở `mermaid.core.mjs:951` theo CR (chưa đọc lại).

**Correction relative to CR-CV-056:**

| # | CR-056 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | Client dựng Mermaid từ `DataFlow.steps` | `dataFlow {flowId, dialect?, detail?, maxServiceHops? ≤ 8, maxSteps? ≤ 200, includeSequence?, includeDfd?}` ⇒ `{flow, sequence?: SequenceModel, dfd?: DfdModel}`; `SequenceModel {participants[{id,label,kind,group?}], messages[{n,from,to,label,kind,sync,dashedReturn,note?,confidence}]}` | Gọi với `includeSequence:true`; `buildSequenceDiagram` nhận **`SequenceModel`** (backend dựng mô hình, client chỉ in ra Mermaid + thoát văn bản); danh sách bước vẫn dùng `flow.steps` (nối bằng `n`) |
| 2 | `DataFlowSummary {id,label,trigger,stepCount,changedStepCount?}`, tìm kiếm phía client | `DataFlowSummary {id,label,trigger,entryService,entryRpc,serviceHops,completeness}`; `dataFlows` nhận `triggerKind?`, `query?` ≤ 128, `service?`, `limit? ≤ 100`, `pageToken?` | Tìm/lọc **phía server** (debounce 300 ms), phân trang `pageToken`; không có `stepCount`/`changedStepCount` |
| 3 | "Chỉ luồng chạm thay đổi" dùng `changedStepCount` hoặc giao `affectedFlows` | Không có `changedStepCount`; id của `FlowSummary` (GitNexus) so với `DataFlow.id` chưa xác nhận; `DataFlow.relatedProcesses[{processId,...}]` chỉ có sau khi tải một luồng | Công tắc dựa giao theo `id` với `ChangeOverlay.affectedFlows[].id`; nếu không có giao nào, hiển thị "Chưa xác định luồng nào chạm thay đổi" (không khẳng định không có); câu hỏi 1 |
| 4 | Luồng luôn như hoàn chỉnh | `completeness: 'complete'\|'partial'`, `gaps[{afterStep,code,message}]`, `DataFlowStep.unimplemented?`, `confidence`, `origin` (`static-fieldtype\|static-name\|process\|declared`) | Banner "Luồng chưa đầy đủ" + khoảng trống chèn vào danh sách tại `afterStep`; nhãn "Suy luận" cho `origin` ≠ `declared`; không trình bày `partial` như đủ |
| 5 | `ComponentRef/StoreRef {id,name}` | `ComponentRef {container, componentId, name, kind:'component'\|'external'\|'ui'}`, `StoreRef {id, kind, name, schema?}`; `stores[{step, store, table?, op, confidence}]` | Dùng đúng kiểu |
| 6 | Mức chi tiết cố định | `detail?: 'service'\|'component'` | Công tắc "Dịch vụ \| Thành phần" |
| 7 | Không DFD | `includeDfd` có sẵn | MVP vẫn không (`includeDfd:false`); câu hỏi 3 |
| 8 | Giới hạn sơ đồ 60 bước/14 người tham gia/40 000 ký tự | `maxSteps ≤ 200` | Gửi `maxSteps:200` (danh sách), sơ đồ cắt 60 ở client; giữ 14/40 000 |
| 9 | Liên kết "Luồng liên quan" bằng id | `SymbolDetail.flows[{id,label,stepCount,step}]` là process GitNexus | Vào lens bằng `query=label` + thông báo "Đang tìm luồng tương ứng"; không khẳng định cùng id |
| 10 | Chuỗi nhãn | U9: chuỗi từ backend không tin cậy | `escapeMermaidLabel` + `securityLevel:'strict'` + DOMPurify (có sẵn trong `MermaidBlock`) |

## 2. Hợp đồng áp dụng

`dataFlows` (20 s, `read`, `nextPageToken`, `total`), `dataFlow` (20 s). `CODEINTEL_SYMBOL_NOT_FOUND {kind}` ⇒ "Chưa có luồng dữ liệu tương ứng". Không dùng `DfdModel` ở MVP.

## 3. Lệch giữa CR và hợp đồng

Bảng mục 1 (10 dòng). Lệch lớn: dòng 1 (backend cung cấp `SequenceModel`) và dòng 4 (hiển thị `partial`).

## 4. Giải pháp

### 4.1 Cây file

```
components/review-map/DataFlowLens.tsx DataFlowListPane.tsx DataFlowDetailPane.tsx DataFlowToolbar.tsx DataFlowDiagram.tsx
  DataFlowStepList.tsx DataFlowStepDetail.tsx
  data-flow-mermaid.ts data-flow-overlay.ts data-flow-export.ts
hooks/useIsDarkTheme.ts   hooks/useDataFlows.ts   hooks/useDataFlow.ts
```

### 4.2 Danh sách và chi tiết

`useDataFlows({query, triggerKind, service})` (`useCodeIntelPagedQuery`, `limit:100`, debounce 300 ms); `useDataFlow(flowId, {detail})` ⇒ `{flow, sequence}`; huỷ khi đổi nhanh; chọn lưu vào `ReviewUiState.dataFlowId`; id không có ⇒ "Chưa có luồng dữ liệu tương ứng với «{nhãn}»" + danh sách. Từ chip `flows` ⇒ bật công tắc "chạm thay đổi".

### 4.3 Sơ đồ Mermaid

```ts
buildSequenceDiagram(seq: SequenceModel, {changedMessages: ReadonlySet<number>, maxMessages = 60, maxParticipants = 14, maxChars = 40000})
  ⇒ {ok:true, source, renderedMessages, totalMessages} | {ok:false, reason:'too-many-participants'|'too-long'|'empty'}
```

`sequenceDiagram` + `autonumber`; người tham gia theo thứ tự xuất hiện, id an toàn `p1…pn` (không dùng chuỗi dữ liệu làm định danh); mũi tên theo `sync`/`dashedReturn`: đồng bộ `->>`, trả về `-->>`, bất đồng bộ `-)`/`--)`; tin nhắn đã đổi thêm tiền tố chữ `[đổi] ` (không dùng `rect`/màu); `note` ⇒ `Note over`. `escapeMermaidLabel`: bỏ ký tự điều khiển, xuống dòng ⇒ dấu cách, `;`→`#59;`, `#`→`#35;`, `%`→`#37;`, `<`/`>`/`"`/dấu huyền ⇒ thực thể; không sinh `click`/HTML. Vẽ bằng `<MermaidBlock content isDark htmlLabels={false}/>` (không sửa file đó). Thu phóng bằng CSS (`−/100%/+`, 50-200 %), `role="img"` + `aria-label`; danh sách bước là bản tương đương đầy đủ. Sao chép: `window.api.ui.writeClipboardText(source)` (trạng thái nút "Đã sao chép" 2 s, không toast). Xuất `.mmd`/`.svg` bằng Blob + `<a download>` (tên `dataflow-<slug>-<YYYYMMDD-HHmm>.<đuôi>`, không `:`); SVG lấy từ DOM đã qua DOMPurify.

### 4.4 Danh sách bước

Bảng: Bước (`n`), Từ → Tới (`ComponentRef.name`), Loại (icon+chữ), Nhãn (`method`/`symbol.name`), Đồng bộ, Kho (`stores` theo `step`), Độ tin cậy/nguồn (`confidence`, `origin`), Cờ. Hàng đã đổi (`computeOverlayFlags` theo `step.symbol.key` và `touchedTables`) có chấm + chữ "đã đổi" + viền trái `border-l-2`; `unimplemented` ghi rõ; khoảng trống `gaps` chèn hàng "Thiếu dữ liệu sau bước n". Bấm hàng có `symbol` ⇒ `SymbolDetailPanel`; không ⇒ `DataFlowStepDetail`. Phím `useRovingListKeys` (SOL-052-04); ảo hoá khi > 150 bước; bước nằm ngoài phần sơ đồ bị cắt mờ.

## 5. Quyết định thiết kế

Tái dùng `MermaidBlock` nguyên trạng; bước đổi nổi bật ở danh sách (Mermaid không nhận token); danh sách là bản đầy đủ; thoát văn bản ở một hàm + test bơm chỉ thị; không thêm thư viện; liên kết sơ đồ↔hàng chỉ qua số bước; `partial` luôn hiển thị.

## 6. Phụ thuộc chéo khu vực

| Cần | Ở đâu | Cổng |
|---|---|---|
| `dataFlows`, `dataFlow` (+ `SequenceModel`) | `BE-CV-SOL-034-data-flow-model`; `BE-CV-SOL-032-proto-and-wscompat-contract-catalog`; `BE-CV-SOL-040-codeintel-view-channels` | G3 (đợt 4; fake G4 trước) |
| `Process` GitNexus ↔ `DataFlow` | `AG-CV-SOL-002-gitnexus-extraction` (id process); `BE-CV-SOL-034-…` (`relatedProcesses`) | chưa xác nhận |
| Mã hoá lớp phủ, chi tiết symbol | SOL-053; `useRovingListKeys` SOL-052-04 | trước 056-06 |

## 7. Tiêu chí chấp nhận

- [ ] Sơ đồ đúng người tham gia, thứ tự và mũi tên theo `sync`/`dashedReturn`.
- [ ] Nhãn chứa `;`, `#`, `%%{init:…}%%`, xuống dòng, `<script>`, dấu ngoặc kép không đổi cấu trúc (không thêm participant/chỉ thị/thẻ); SVG xuất không có `<script>`.
- [ ] > 60 tin nhắn chỉ vẽ 60 đầu "{r}/{t}"; > 14 người tham gia hoặc > 40 000 ký tự không vẽ, danh sách vẫn đủ.
- [ ] `partial` có banner và `gaps` trong danh sách; `origin` ≠ `declared` có nhãn "Suy luận".
- [ ] Bước đã đổi có chấm, chữ và viền trái trong danh sách, `[đổi]` trong sơ đồ.
- [ ] Sao chép Mermaid và xuất `.mmd/.svg` đúng; tên tệp hợp lệ trên Windows.
- [ ] Tìm kiếm/lọc phía server, phân trang `pageToken`; id lạ không lỗi.
- [ ] Không hex; không sửa `MermaidBlock.tsx`; `translate()` 5 locale.

## 8. Kiểm thử

`data-flow-mermaid.test.ts` (kể cả bơm chỉ thị), `data-flow-overlay.test.ts`, `data-flow-export.test.ts`, `useIsDarkTheme.test.ts`, `useDataFlows.test.tsx`, `DataFlowLens.test.tsx` (mock `MermaidBlock`, `window.api.ui.writeClipboardText`), `DataFlowStepList.test.tsx`, `code-intel-locale-coverage` mở rộng. Kiểm tay: sơ đồ 60 tin nhắn sáng/tối, phóng 200 %, tải `.svg` ở Electron đóng gói. Chưa chạy.

## 9. Rủi ro và điểm chưa kiểm chứng

Cú pháp thoát bằng thực thể `#59;` chưa chạy trên Mermaid 11.15 (test chuỗi không chứng minh hiển thị đúng); giới hạn riêng của sequence chưa kiểm; tải tệp trong Electron đóng gói chưa kiểm; thu phóng CSS làm mờ; `SequenceModel.messages.n` có khớp `steps.n` không (giả định); id `FlowSummary` ≠ `DataFlow`.

## 10. Câu hỏi mở

1. `DataFlowSummary`/`affectedFlows` có thêm `changedStepCount` hoặc `processIds` để nối "Luồng liên quan" chắc chắn?
2. `SequenceModel.messages[].n` ↔ `DataFlowStep.n`.
3. Có hiển thị `DfdModel` (UI→gateway→service→kho)? Đề xuất sau MVP.
4. Mermaid 11 có cú pháp participant `database` ổn định?

## 11. Danh sách task

| Task | Tên | Priority |
|---|---|---|
| [FE-CV-TASK-056-01](../tasks/FE-CV-TASK-056-01-use-is-dark-theme-hook.md) | Hook `useIsDarkTheme` | P1 |
| [FE-CV-TASK-056-02](../tasks/FE-CV-TASK-056-02-data-flow-mermaid-from-sequence-model.md) | `data-flow-mermaid` từ `SequenceModel` | P1 |
| [FE-CV-TASK-056-03](../tasks/FE-CV-TASK-056-03-data-flow-overlay-and-step-model.md) | Mô hình bước và lớp phủ | P1 |
| [FE-CV-TASK-056-04](../tasks/FE-CV-TASK-056-04-data-flow-export-and-clipboard.md) | Sao chép và xuất | P1 |
| [FE-CV-TASK-056-05](../tasks/FE-CV-TASK-056-05-data-flow-list-pane-and-queries.md) | Danh sách luồng và truy vấn | P1 |
| [FE-CV-TASK-056-06](../tasks/FE-CV-TASK-056-06-data-flow-diagram-and-step-list.md) | Sơ đồ và danh sách bước | P1 |
| [FE-CV-TASK-056-07](../tasks/FE-CV-TASK-056-07-data-flow-lens-registration-i18n-e2e.md) | Đăng ký lens, i18n, e2e | P1 |

Thứ tự: 056-01, 056-02, 056-03, 056-04 song song → 056-05 → 056-06 → 056-07.

## 12. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-056-dataflow-lens.md`, `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `/opt/repos/orca/frontend/src/renderer/src/components/editor/{MermaidBlock.tsx,mermaid-config.ts}`, `/opt/repos/orca/frontend/src/preload/api-types.ts`, `/opt/repos/orca/guides/STYLEGUIDE.md`.
