# BE-CV-TASK-041-03: Định dạng kết quả cho LLM: cắt mã nguồn, lọc đường dẫn nhạy cảm, giữ khoá

**From Solution:** BE-CV-SOL-041-mcp-codeintel-tools
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/pack1_codeintel_result.go` (mới), `pack1_codeintel_result_test.go` (mới), `pack1_codeintel.go`
**Depends on:** TASK-041-02
**Status:** [ ] TODO

---

## Context

`normalizeResult` (`result_normalize.go`): `redactValue` => (camelize trừ `KeepKeys`) => `Post` => `truncateObject(limit)`; `filterSensitiveItems` chỉ xử lý `obj["items"]` (`files_sensitive_guard.go`), không hợp kiểu phong bì `{..., data}` của code-intel (SOL-041 M7). `camelizeKeys` camel hoá cả khoá tự do (M5). `IsSensitivePath(rel, extra)` nhận đường dẫn đã qua `CleanWorktreePath`.

## Việc cần làm

1. `Post` cho cả 9 spec: `codeIntelPost(v any) any`: duyệt đệ quy `map`/`[]any`; phần tử `map` có khoá `filePath`, `file` hoặc `path` (chuỗi) mà `pathsafety`/`CleanWorktreePath` lỗi hoặc `IsSensitivePath(...)` => bỏ phần tử khỏi mảng cha (nếu là giá trị trực tiếp của khoá trong map cha thì thay bằng `null`); đếm số bị bỏ ghi vào `droppedSensitive` ở gốc (số, không tên file).
2. `symbol`: `Post` bổ sung cắt `data.source.text` theo dòng: giữ đầu và cuối, chèn dòng `... [truncated N lines] ...`, tổng ≤ 32 KiB (theo byte, cắt ở ranh giới dòng/rune), `source.truncated=true`.
3. Không thêm trường gợi ý lệnh; không đổi `risk`/`stale` của dữ liệu.
4. `KeepKeys:true` ở cả 9 spec (đã đặt ở 041-02); test chứng minh khoá `metrics.lines_changed` không đổi.

## Kiểm thử

- Dữ liệu: `findings[].evidence[].path=".env"`, `"a/.git/config"`, `"../x"` bị bỏ; `"src/a.go"` giữ.
- `symbol` với `source.text` 200 KiB => ≤ 32 KiB, `truncated=true`, hợp lệ UTF-8; token giả `ghp_...` trong mã bị `redactValue` che.
- Kết quả vượt `MaxResultBytes` => `truncateObject` cắt, `truncated:true`, `hint`.
- `KeepKeys`: khoá `incoming.calls` và `a_b` giữ nguyên.
- Lệnh: `go test ./internal/adapter/mcpserver/tools/... -run 'CodeIntelResult|Redaction'`.

## Tiêu chí hoàn thành

- [ ] Không đường dẫn nhạy cảm nào ra tool; mã nguồn ≤ 32 KiB có `truncated`.
- [ ] Khoá tự do không bị camel hoá.

## Rủi ro và lưu ý

- Danh sách khoá đường dẫn (`filePath|file|path`) dựa trên UI-API §4; thêm khoá mới của proto sau này cần cập nhật (test duyệt tên khoá từ golden kênh).
- Bỏ phần tử có thể làm `totalCount` không khớp: chấp nhận, `droppedSensitive` giải thích.
