# FE-CV-SOL-058: Lens Lưu trữ (service → kho dữ liệu → topic → secret, chỉ đọc)

> 🚧 **In Progress.** Trạng thái (cập nhật 2026-10-08): 4/5 task DONE; PARTIAL 0. Chi tiết ở dòng `**Status:**` và "Ghi chú hoàn thiện" của từng task.

**CR:** [CR-CV-058](../../../../../../docs/crs/v7/review-frontend/CR-CV-058-storage-lens.md)
**Area:** frontend (`frontend/src/renderer/src/components/review-map/storage/`, hook, khoá slice)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (U1, U3, U5, U6, U7, U9; §3.1 kênh `storage`; §4.4 `StorageMap`/`Store`/`SourceRef`; §4.3 `ChangeOverlay.changedFiles`; §2.3 lỗi; §6 cờ), [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (**PQ-02, PQ-04, PQ-12, PQ-29, PQ-32**; §7 G3/G4; §8.2; §8.3; §9 O-1, O-16). **TDD:** [v5/02-state-management](../../../../tdd/v5/02-state-management.md), [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md).

## 1. Trạng thái hiện tại (re-verify)

**Đã xác minh trong phiên này:** `components/review-map/` chưa có; `@xyflow/react`, `@tanstack/react-virtual` có trong `frontend/package.json`; không có `maskSensitiveText` (SOL-057 task 057-01 tạo `components/review-map/sensitive-text-masking.ts`); `components/ui/` có `badge`, `toggle-group`, `tooltip`, `popover`, `scroll-area`, `collapsible`.
**Theo CR-CV-058 (chưa kiểm lại):** chưa có UI hiển thị DSN/secret nào; quy tắc che secret là hợp đồng series (README v7 mục 6); 08 §6 cho độ tự động "thấp–trung bình".

### Lệch giữa CR và hợp đồng (hợp đồng thắng)

| # | CR-058 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `sourceFiles: string[]` trên Store/Binding/Topic | `evidence: SourceRef[]` (`{path, line?, kind}`) trên cả ba (§4.4) và `Store.change?`, `bindings[].change?` | Dùng `evidence[].path` ∩ `changedFiles`; ưu tiên `change` của backend |
| 2 | `origin: declared|inferred` | `confidence: 'declared'|'derived'|'inferred'` | Nhãn theo `confidence`; `derived` = "suy ra từ cấu hình" |
| 3 | `env: dev|prod` | `env?: 'dev'|'prod'|'legacy'` + `includeLegacy?` | Toggle dev/prod + công tắc "Hiện legacy" |
| 4 | Topic `{publisher, subscribers[]}` | `topics[]: {name, stream?, publishers[], subscribers[], delivery, payload?, evidence, confidence}` (không có `change`) | Nhiều publisher; `delivery` hiển thị nhãn; thay đổi topic suy từ `evidence` ∩ `changedFiles` |
| 5 | Secret: `Store.kind='vault'` + `configKey` | Giữ (kind gồm `'vault'`); "Không có trường giá trị secret"; `redactedCount`, `warnings: string[]` | Làn Secret = store `vault` + binding có `configKey`; hiển thị `redactedCount` ("N giá trị đã che ở backend") |
| 6 | `Store.owner?` | `owner?: {name}`; `deployed`, `supportedByCode`, `external` | Nhãn "chưa triển khai", "code chưa hỗ trợ", "ngoài hệ thống" |
| 7 | `warnings` (prod chưa rõ) | `StorageMap.warnings: string[]` (chuỗi tự do) | Hiển thị văn bản thuần (U9); không dịch |
| 8 | Hook `use-code-intel-storage.ts` | README nhóm: `useCodeIntel*.ts` | `useCodeIntelStorage.ts` |
| 9 | `maskSensitiveText` ở `storage/storage-secret-masking.ts` | Dùng chung; tạo ở SOL-057-01 | `components/review-map/sensitive-text-masking.ts` (không tạo bản riêng) |
| 10 | Khoá i18n `auto.components.review.map.storage.` | README nhóm `auto.components.reviewMap.<Thành phần>.<tên>` | Theo README nhóm |

## 2. Giải pháp

### 2.1 Cây file

```
components/review-map/storage/
  storage-view-model.ts        (mới) StorageMap → nodes/edges 4 làn + cờ origin/deployed
  storage-layout.ts            (mới) bốn làn cố định, một lượt barycenter
  storage-change-marks.ts      (mới) computeStorageChangeMarks(map, changedFiles)
  StorageLens.tsx, StorageToolbar.tsx, StorageCanvas.tsx, StorageNodeDetail.tsx,
  StorageLegend.tsx, StorageInferenceNotice.tsx, StorageSecretNode.tsx   (mới)
hooks/useCodeIntelStorage.ts   (mới)
store/slices/code-intel.ts     (sửa nhỏ: khoá `storageEnv`, `selectedStorageNodeId`; vào CODE_INTEL_WORKTREE_KEYED_STATE_KEYS)
```
Đăng ký lens `id:'storage'` ở `REVIEW_LENS_DEFINITIONS` (SOL-051) và **chỉ khi** backend hỗ trợ: gọi `storage` lỗi `CODEINTEL_UNAVAILABLE`/`AGENT_UNSUPPORTED` (kind `unsupported`) ⇒ lens ẩn khỏi `ReviewLensTabs`, không hiện tab chết (CR-058 mục 6).

### 2.2 Dữ liệu và hook

`useCodeIntelStorage({worktreeId, env, includeLegacy})` bọc `useCodeIntelQuery(worktreeId, 'storage', {env, includeLegacy})` → `{status, stage?, envelope: CodeIntelEnvelope<StorageMap> | null, error, reload}`. Bỏ phản hồi cũ khi đổi `env`; push `changed` chỉ đánh dấu cũ (chip), không tải lại giữa lúc tương tác. Phong bì có `stale`, `truncated`, `sources[].commit`; `StorageMap.asOfCommit` hiển thị ở toolbar.

### 2.3 Bố cục bốn làn và đánh dấu thay đổi

```
 Service            Kho dữ liệu                 Topic                    Secret
┌───────────┐ rw  ┌─────────────────────┐   ┌──────────────────┐     ┌───────────────────┐
│ infra-    │────▶│ postgres (infra)    │   │ orca.infra.agent │     │ vault_ssh_role    │
│ fleet     │ ro  ├─────────────────────┤   └──────────────────┘     │ (tên khoá; giá trị│
│           │────▶│ mysql   (infra) [+] │      ▲ pub / sub ▼         │  không hiển thị)  │
└───────────┘     └─────────────────────┘                            └───────────────────┘
```
- `storage-layout.ts`: x cố định theo làn, sắp theo `name` (tất định) rồi một lượt barycenter theo hàng xóm làn trái. Cạnh: binding (service→store; nhãn `rw|ro`), topic (publisher→topic, topic→subscriber), `configKey` (service→secret, nét đứt). Không thêm thư viện (O5); `onlyRenderVisibleElements` bật.
- `computeStorageChangeMarks(map, changedFiles) → Map<nodeId, 'added'|'modified'|'removed'|'related'>`: (1) dùng `change` của backend nếu có; (2) không thì giao `evidence[].path` với `changedFiles[].path`; (3) `related` cho nút nối trực tiếp với nút bị đổi. Thiếu cả hai nguồn: banner "Chưa xác định thành phần nào bị đổi (backend chưa cung cấp bằng chứng nguồn)", không tô gì. Ký hiệu `+ ~ −` + nhãn chữ; màu theo cùng quy ước `--git-decoration-*` như ERD (SOL-057); `related` viền muted.
- Bộ lọc "Chỉ thành phần bị đổi + liên quan" mặc định bật khi có nút bị đổi; không có nút đổi ⇒ "Thay đổi này không chạm cấu hình lưu trữ **trong phạm vi index**" (không "an toàn").

### 2.4 Che secret nhiều lớp

Backend là lớp chính (hợp đồng: không có trường giá trị secret; chuỗi tự do đã che). Frontend: **mọi** chuỗi tự do của `StorageMap` (`name`, `configKey`, `via`, `payload`, `stream`, `warnings[]`, `evidence[].path`) qua `maskSensitiveText` trước khi render; khi `masked` ⇒ icon `ShieldAlert` + tooltip "Giá trị nhạy cảm đã bị che ở giao diện — báo người triển khai backend" và `console.warn` **không** kèm nội dung. `StorageSecretNode` chỉ hiển thị tên khoá + đường dẫn Vault, nhãn "giá trị không bao giờ hiển thị"; **không** có nút hiện/sao chép giá trị, chỉ "Sao chép tên khoá". Không đưa nội dung StorageMap vào `localStorage`, tiêu đề tab hay cửa sổ; cache chỉ ở slice trong bộ nhớ, dọn khi xoá worktree. Ghi chú gửi agent (SOL-060) chỉ chèn id/tên nút đã che, không chèn `payload`.

### 2.5 Chi tiết nút và liên kết

`StorageNodeDetail` (cắm vào panel chi tiết của SOL-053): `kind`, `env`, `owner`, `schemas`, binding vào/ra (`rw|ro`, `via`), publisher/subscriber, `confidence` (nhãn "suy luận" khi `inferred`), cờ `deployed|supportedByCode|external`, `evidence[]` (mỗi mục "Xem diff" nếu thuộc `changedFiles`, "Mở tệp" thường). Kho `postgres|mysql` có `owner.name`: nút "Mở ERD của service" gọi action chuyển lens + `setErdService(worktreeId, owner.name)` (SOL-057 task 057-04). Topic: liệt kê service pub/sub; bấm → chọn nút service trong canvas.

### 2.6 Trạng thái, phím, i18n, a11y

Tải theo ngưỡng 100 ms/1 s/3 s (qua SSH trì hoãn ~200 ms, khoá điều khiển ngay). Không tìm thấy cấu hình: rỗng inline (không lỗi). `env=prod` thiếu dữ liệu: hiển thị `warnings[]` từ backend (không cứng chữ trong UI). `stale`: banner chung của khung. Lỗi `CODEINTEL_*`: inline + "Thử lại" (không toast); `offline`: dùng trạng thái của khung (SOL-051). Phím cục bộ: `f` vừa khung, `Esc` bỏ chọn (không phím bổ trợ, bỏ qua khi `isEditableTarget`). Khoá `auto.components.reviewMap.Storage*` đủ en/es/ja/ko/zh; chỉ biến CSS; reduced-motion tắt animation khung nhìn; canvas chỉ đọc (`nodesConnectable={false}`, `nodesDraggable` tuỳ chọn tắt). Nút `tabIndex=0` + `aria-label`; có Danh sách văn bản thay thế (bảng service→kho→quyền) để trình đọc màn hình đọc được (nhỏ, không ảo hoá vì chỉ vài chục nút; chưa đo).

## 3. Quyết định thiết kế

- **Chỉ đọc hoàn toàn**: không có thao tác ghi cấu hình (ngoài phạm vi, rủi ro secret).
- **Bốn làn cố định** thay vì thuật toán đồ thị: đủ cho vài chục nút, không thêm thư viện.
- **Che hai lớp, không có đường "hiện giá trị"**; mô-đun che dùng chung (SOL-057-01).
- **Nhãn suy luận thay vì vẽ như sự thật** (08 §6): `confidence` luôn hiển thị.
- **Không khẳng định an toàn** khi không thấy thay đổi.

## 4. Phụ thuộc chéo khu vực

| Cần | Solution đối ứng | Ghi chú |
|---|---|---|
| `StorageMap` (compose/config/adapter/migration → stores, bindings, topics) | `BE-CV-SOL-035-storage-map` (P2) | Chưa có ⇒ fake backend G4; lens ẩn khi `unsupported` |
| `changedFiles`, `touchedTables` | `BE-CV-SOL-036-change-overlay-pipeline` | Cho đánh dấu thay đổi |
| Kênh `codeIntel.storage` | `BE-CV-SOL-040-codeintel-view-channels` (cổng **G3**: `BE-CV-SOL-040-codeintel-channel-foundation`) | |
| Che secret/giới hạn đường dẫn | `BE-CV-SOL-072-security-tests-service-gateway`; `AG-CV-SOL-072-security-tests-agent` (kiểm thử) | Phối hợp kiểm thử canary |
| Bridge, store, fake backend G4 | `FE-CV-SOL-050-*`; `FE-CV-SOL-073-flag-gating-and-web-e2e` (task 073-02) | **Bắt đầu bằng fake backend G4** |
| Khung/panel chi tiết/liên kết ERD | `FE-CV-SOL-051-…`, `FE-CV-SOL-053-…`, `FE-CV-SOL-057-erd-lens` | |

Thứ tự: 050 → 051 → 053 → 057 (task 057-01 che chuỗi) → **058** (đợt 6).

## 5. Tiêu chí chấp nhận

- [ ] Lens `storage` chỉ hiện khi `effective.codeIntelEnabled` và backend hỗ trợ; Electron và web cùng mã.
- [ ] Bốn làn service → kho → topic → secret với cạnh `rw|ro`, publish/subscribe, `configKey`.
- [ ] Chuyển dev/prod (+ công tắc legacy); kho có nhãn `kind` (icon + chữ), `schemas`, `deployed|supportedByCode|external`.
- [ ] Thành phần bị đổi `+ ~ −` kèm nhãn chữ; bộ lọc "chỉ đổi + liên quan"; thiếu dữ liệu ⇒ banner nêu rõ.
- [ ] `confidence='inferred'|'derived'` có nhãn; không câu "an toàn/không thay đổi" ngoài "không phát hiện trong phạm vi index".
- [ ] Không nơi nào hiển thị giá trị secret; không nút hiện/sao chép giá trị; chuỗi DSN/token/khoá riêng bị che với icon cảnh báo; test chứng minh bằng dữ liệu giả có secret không xuất hiện trong DOM, `localStorage`, `console`.
- [ ] "Mở ERD của service" chuyển lens + service; topic liệt kê publisher/subscriber bấm được.
- [ ] Tải/rỗng/lỗi/offline/stale đúng 2.6; lỗi inline có "Thử lại", không toast.
- [ ] Không hex, `translate()` đủ 5 locale, không `components/code-review/*`, không thêm thư viện, không `max-lines` disable.

## 6. Kiểm thử (Vitest + Testing Library)

Hàm thuần: `storage-layout` (tất định, làn đúng, không chồng nút), `storage-change-marks` (ưu tiên `change` backend; giao `evidence`∩`changedFiles`; `related`; thiếu dữ liệu ⇒ rỗng + cờ), `storage-view-model` (topic nhiều publisher; secret từ `configKey`). Mô-đun che (đã có test ở 057-01) thêm bảng ca riêng cho StorageMap (DSN trong `via`, token trong `payload`). Component: `StorageSecretNode` không render gì ngoài tên khoá/đường dẫn kể cả khi dữ liệu giả nhét giá trị vào `payload`; `StorageNodeDetail` (nút ERD, "Xem diff" chỉ khi tệp đổi); `StorageLens` (rỗng, prod thiếu dữ liệu + `warnings`, lỗi + thử lại, ẩn khi `unsupported`). Hook: bỏ phản hồi cũ khi đổi `env`. Bảo mật (phối hợp CR-072): fixture secret giả qua toàn bộ UI không xuất hiện trong DOM/`localStorage`/spy `console` — **chưa chạy**, kế hoạch.

## 7. Rủi ro và điểm chưa kiểm chứng

- `StorageMap` ở backend có độ tự động thấp–trung bình; `env=prod` có thể gần như trống (08 §6/09, chưa kiểm).
- Danh sách mẫu che là đề xuất; che thừa/thiếu; O-16 (chủ sở hữu bộ che chung) chưa chốt.
- Chưa đọc cấu hình thật của các service và `deploy/*/docker-compose.yml`; mọi ví dụ lấy từ CR/08 §6.
- Số nút chưa đo (giả định vài chục).
- `Topic` không có `change` ở backend: dấu thay đổi của topic là suy từ `evidence` (có thể sai khi `evidence` thiếu).
- Cần `projectId` (O-1).

## 8. Câu hỏi mở

1. Có cần hiển thị `env=prod` ở MVP khi topology prod chưa rõ?
2. Đường dẫn Vault tự nó có coi là nhạy cảm theo chính sách nào không (cho phép "Sao chép tên khoá"/đường dẫn)?
3. `Topic` nên có `change` từ backend (như `Store`/`binding`)? Hiện UI suy từ `evidence`.
4. Mẫu che chung nằm ở đâu để hai phía dùng một nguồn (proto, hay mỗi phía một bản có test đối chiếu — O-16)?

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-058-storage-lens.md`, `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/frontend/package.json`, `/opt/repos/orca/frontend/src/renderer/src/components/ui/`, `/opt/repos/orca/frontend/src/renderer/src/i18n/no-top-level-translate.test.ts`.

## 10. Ghi chú triển khai (2026-10-07)

- Mã ở `components/review-map/storage/`, `hooks/useCodeIntelStorage.ts`; khoá UI (`storageEnv`, `selectedStorageNodeId`) ở `store/slices/review-ui.ts`. Lens `storage` đã đăng ký `load`. Dùng lại `sensitive-text-masking.ts` của 057-01.
- Khi backend trả `unsupported`/`disabled`, lens hiện thông báo "không khả dụng" thay vì ẩn tab (ẩn tab cần cơ chế khả dụng theo runtime trong khung, chưa có).
- Thêm `StorageTextView` (bảng văn bản thay thế). Evidence không đổi chỉ hiện đường dẫn (khung chưa có action mở tệp thường).
- Việc còn lại: ẩn tab khi `unsupported`; e2e web (073-03).
