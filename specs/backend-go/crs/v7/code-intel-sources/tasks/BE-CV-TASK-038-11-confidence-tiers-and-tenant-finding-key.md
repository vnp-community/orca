# BE-CV-TASK-038-11: Tầng độ tin cậy, heuristic RLS, khoá toàn cục, `finding_key`

**From Solution:** BE-CV-SOL-038-static-tenant-filter-rule
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/tenantfilter/{confidence_tiers.go, global_key_columns.go, tenant_finding_key.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-038-10; BE-CV-TASK-037-03 (`NewKey`, `Finding`)
**Status:** [x] DONE

---

## Context

Solution 2.C (bảng tầng) và 2.D (`finding_key`). Chống ồn là yêu cầu hạng nhất: bằng chứng RLS không đồng nhất (chỉ `mcp-service` đặt `app.tenant_id` theo CR).

## Việc cần làm

1. `global_key_columns.go`: danh sách cấu hình mặc định `*_hash`, `token`, `code`, `delivery_id` (khớp theo hậu tố/tên cột) + `Options.ExtraGlobalKeys`; hàm `IsGlobalKeyFilter(filterColumns []string) bool` đúng khi **mọi** cột lọc thuộc tập.
2. `confidence_tiers.go`: `Tier(c Candidate, ctx TierContext) (confidence string, severity string, skip bool)`: `skip` khi hàm bao ngoài chứa `outbox`/`relay` (không phân biệt hoa thường) hoặc có `Allow` hợp lệ; `low`/`info` khi `IsGlobalKeyFilter`; `medium`/`info` khi (a) có **truy vấn cùng bảng có `tenant_id` trong cùng `enclosingFunc`** hoặc (b) `ctx.ServiceSetsTenantGUC` (service Postgres có literal `set_config('app.tenant_id'` hoặc `SET LOCAL app.tenant_id` trong `internal/adapter/postgres`); còn lại `high`/`warning`. Thứ tự ưu tiên: skip > low > medium > high. Ghi `params.rlsPossiblyActive=true` khi (b).
3. `tenant_finding_key.go`: `Key(service, dialect, file, enclosingFunc, sqlNormalized string, rule string) string` = `NewKey(rule, service+"::"+dialect+"::"+file+"::"+enclosingFunc+"::"+sqlNormalized)` (hàm của 037); dịch dòng không đổi khoá; đổi SQL đổi khoá.
4. `ToFinding(c Candidate, tier ...) Finding`: `kind` theo `KindOf(rule)` (`missing_tenant_id`), `titleKey` `codeintel.finding.sql.missing-tenant-filter`, `params{table, function, dialect}`, `evidence{path, line}`, `subject` `<service>: <function> → <table>`; **không** chứa SQL thô > 300 ký tự (cắt, đã chuẩn hoá); `scope{service, layer:"adapter"}`.
5. `DetectServiceSetsTenantGUC(files map[string][]byte) bool`: quét literal chuỗi (qua kết quả `ExtractSQL`/`go/parser`, không regex trên text) trong `adapter/postgres`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/tenantfilter/ -run 'Tier|GlobalKey|Key|ToFinding'`: mỗi tầng một ca; thứ tự ưu tiên; `outbox_events` skip; hàm `PublishOutboxRelay` skip; MySQL ⇒ `high`; Postgres có `set_config` ⇒ `medium` + `rlsPossiblyActive`; `WHERE token_hash = ?` ⇒ `low`; khoá ổn định khi đổi dòng; SQL dài ⇒ cắt; không PII.

## Tiêu chí hoàn thành

- [x] Tầng và `severity` đúng bảng 2.C; `finding_key` thoả PQ-06 (≤ 128 ký tự, khớp `ValidKey`).
- [x] Không SQL thô dài trong finding.

## Rủi ro và lưu ý

- Heuristic (a) "cùng hàm" có thể che lỗ thật; bộ vàng (TASK-038-12) đo ảnh hưởng.
