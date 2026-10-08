# FE-CV-TASK-058-01: Mô hình hiển thị StorageMap và đánh dấu thành phần bị đổi (hàm thuần)

**From Solution:** [FE-CV-SOL-058](../solutions/FE-CV-SOL-058-storage-lens.md) mục 2.3
**Priority:** P2
**Area:** frontend / renderer (hàm thuần)
**File:** `frontend/src/renderer/src/components/review-map/storage/storage-view-model.ts`, `storage-change-marks.ts` (mới) + `*.test.ts`
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (kiểu `StorageMap`, `Store`, `SourceRef`, `ChangedFile`); FE-CV-TASK-057-01 (`maskSensitiveText`)
**Status:** [x] DONE (verified 2026-10-07: storage/storage-pure.test.ts 15/15 PASS, tsc/oxlint sạch)

## Context

- UI-API §4.4: `Store {id, kind, name, env, owner?, schemas?, deployed, supportedByCode, external, evidence[], confidence, change?}`, `bindings[] {service, store, access, via, configKey?, evidence, confidence, change?}`, `topics[] {name, stream?, publishers[], subscribers[], delivery, payload?, evidence, confidence}`, `redactedCount`, `asOfCommit`, `warnings: string[]`. Không có trường giá trị secret.
- CR dùng `sourceFiles`/`origin`; hợp đồng dùng `evidence`/`confidence` (xem bảng lệch SOL-058).

## Việc cần làm

1. `buildStorageViewModel(map)` → `{nodes, edges}` bốn làn (`service|store|topic|secret`): service suy từ `bindings[].service` + `topics[].publishers|subscribers`; secret = store `kind:'vault'` + binding có `configKey` (nút secret chỉ mang `name`/đường dẫn Vault); mỗi nút có `confidence`, cờ `deployed|supportedByCode|external`.
2. `computeStorageChangeMarks(map, changedFiles)` → `Map<nodeId, 'added'|'modified'|'removed'|'related'>` theo thứ tự: `change` backend → giao `evidence[].path` với `changedFiles[].path` (chuẩn hoá tương đối, không phân biệt `\`) → `related` cho nút nối trực tiếp; trả thêm `{unknown: boolean}` khi **cả hai** nguồn đều thiếu.
3. Mọi chuỗi tự do qua `maskSensitiveText` (mask cờ kèm theo nút); `warnings[]` cũng qua che nhưng không dịch.
4. Enum lạ (`kind`, `confidence`, `access`, `delivery`) → `'unknown'`/`'other'`; không ném.

## Kiểm thử

- Hàm thuần với fixture dựng tay (2 service, postgres+mysql, 1 topic nhiều publisher, 1 vault): làn đúng; secret chỉ tên khoá; `change` backend thắng giao đường dẫn; `related`; thiếu cả hai nguồn ⇒ `unknown`; DSN trong `via` bị che; enum lạ.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/storage/storage-view-model src/renderer/src/components/review-map/storage/storage-change-marks`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần, bất biến, test xanh; không phụ thuộc React/store.
- [ ] Không tạo trường nào chứa giá trị secret; nút secret không có `payload`/`via` thô.
- [ ] Mỗi mark đi kèm ký hiệu (`+ ~ −`/`related`) cho component.

## Rủi ro

- Topic không có `change` từ backend: mark topic là suy luận từ `evidence` (có thể sai khi `evidence` thiếu).

## Ghi chú triển khai (2026-10-07)

- Tạo `storage/storage-view-model.ts`, `storage/storage-change-marks.ts`, `storage/storage-map.fixture.ts`; test chung ba mô-đun (kể cả layout) trong `storage-pure.test.ts`.
- Kho `vault` không vào làn Kho mà thành nút secret (khoá = `configKey` hoặc tên kho); nút secret không mang `via`/`payload`. `computeStorageChangeMarks` nhận view model (không phải `StorageMap` thô) để dùng chung id nút; `unknown` chỉ bật khi có tệp đổi nhưng không có `change` lẫn `evidence`.
