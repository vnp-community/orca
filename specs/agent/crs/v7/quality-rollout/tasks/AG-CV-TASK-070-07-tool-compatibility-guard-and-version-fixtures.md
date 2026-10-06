# AG-CV-TASK-070-07: Guard phiên bản/marker và `TestEverySupportedVersionHasFixtures`

**From Solution:** [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/tool-compatibility.test.ts` (mới); hàm phân loại thêm vào module `SUPPORTED_*` của AG-CV-SOL-001 (không tạo module thứ hai)
**Depends on:** 070-02, 070-04, 070-06; AG-CV-SOL-001
**Status:** [ ] TODO

## Context

Agent-rpc §1.3: dải `>=1.6.0 <2` / `>=1.4.0 <2`; marker GitNexus `meta.json.schemaVersion ∈ {5}`, CodeGraph `extractionVersion ∈ {24}`, `schema_versions ∈ [1,8]`; ngoài dải: `supported:false`, `TOOL_UNAVAILABLE reason=unsupported_version`; lỗi lược đồ `schema_version_unsupported`.
Hợp đồng không có `compatibility` tri-state (BE-CV-SOL-070 cũng báo): giữ tri-state nội bộ, ánh xạ `supported = state!=="incompatible"`, `untested` → `warnings:["tool_version_untested"]`, `reindexRecommended` → `warnings:["index_built_with_old_extraction"]` (đề nghị bổ sung hợp đồng).

## Việc cần làm

1. Cài/xác nhận `classifyToolCompatibility(tool, version, markers): "verified"|"untested"|"incompatible"` theo solution 2.6 (so semver tối thiểu viết tay; không thêm phụ thuộc).
2. Test bảng: 1.6.9 verified; 1.6.10 và 1.7.0 untested; 2.0.0 incompatible; 1.5.9 incompatible; marker lạ → incompatible; `reindexRecommended` giữ trạng thái và thêm cảnh báo.
3. `TestEverySupportedVersionHasFixtures`: mỗi phiên bản trong `SUPPORTED_TOOL_VERSIONS` có thư mục fixture + `MANIFEST.json` hợp lệ (hàm nhận danh sách tiêm vào để test "thêm phiên bản giả → đỏ" mà không sửa hằng thật); `MANIFEST.markers` nằm trong `SUPPORTED_SCHEMA_MARKERS`.
4. Shape guard: parser sai hình dạng → `format_drift` kèm `{tool, version, command}` hằng, **không** đầu ra thô; log một lần mỗi `(tool, version, command)` (kiểm bằng logger giả).

## Kiểm thử

- Như mục 3; thêm test `incompatible` làm method đọc trả `CODEINTEL_TOOL_UNAVAILABLE` với `reason` đúng và `data` có `seen`/`supported` (nếu 001 đã có dispatcher; nếu chưa, test ở mức hàm).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Bảng phân loại đúng; test fixture-per-version đỏ khi thiếu; `format_drift` không rò đầu ra.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Dải phiên bản là giả định chưa kiểm; `data.seen`/`supported` là đề xuất chưa có trong hợp đồng.
