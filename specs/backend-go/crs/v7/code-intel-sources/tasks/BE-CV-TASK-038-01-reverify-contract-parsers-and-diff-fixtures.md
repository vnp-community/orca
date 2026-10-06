# BE-CV-TASK-038-01: Re-verify chữ ký parser 031/032, cơ chế route HTTP và dựng fixture so sánh hợp đồng

**From Solution:** BE-CV-SOL-038-contract-diff
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/testdata/contract-diff/PROVENANCE.txt` (mới); `testdata/contract-diff/proto/{base,head}/*.proto`, `wscompat/{base,head}/*.go`, `migrations/{base,head}/{postgres,mysql}/*.sql` (mới)
**Depends on:** BE-CV-SOL-031-sql-migration-parser, BE-CV-SOL-032-proto-and-wscompat-contract-catalog (chữ ký thật), BE-CV-SOL-030
**Status:** [ ] TODO

---

## Context

Solution 038 phải gọi bộ phân tích do 031/032 cung cấp nhưng chưa đọc chữ ký (C4); cơ chế đăng ký route HTTP của `api-gateway` chưa khảo sát. Chỉ đọc.

## Việc cần làm

1. Đọc mã/đặc tả đã merge của 032 (`internal/adapter/protoschema`, `gocallgraph`, `internal/domain/contract`) và 031 (`erd.Catalog.Apply`, `sqlmigration`): ghi vào PR chữ ký **thật** của (a) parse một tệp `.proto` ⇒ message/field/rpc/enum có số field, `reserved`, `oneof`, `label`; (b) trích kênh `wscompat` từ nội dung một tệp `channels_*.go` ⇒ `WsChannel{name, kind, args[{index, jsonName, goType, omitempty}], targets}`; (c) áp chuỗi migration ⇒ `Catalog`; điều kiện **hàm thuần theo nội dung** (không đọc đĩa). Chưa merge ⇒ ghi "theo đặc tả" và tạo port `ProtoSchemaParser`, `WsChannelExtractor`, `MigrationCatalogBuilder` ở TASK-038-03 với kiểu tối thiểu.
2. Khảo sát `backend-go/services/api-gateway/internal/adapter/httpgateway/*_routes.go`: ghi cách đăng ký (literal `mux.Handle`/hàm bọc/bảng); kết luận "trích tĩnh được/không"; nếu không ⇒ `route.*` ra ngoài MVP (ghi vào solution §7).
3. Đối chiếu số kênh `r.Register(` trong `channels_*.go` (grep) với con số của CR (~475): ghi số.
4. Fixture: với mỗi quy tắc ở solution 2.D một cặp tối thiểu (base,head): proto (≥ 16 ca: gồm `reserved`, đổi tên, đổi số, đổi kiểu, label, oneof, enum, package); `wscompat` rút từ mẫu `channels_git.go` (đổi tag `worktree`→`worktreeId`, xoá `paths`, thêm trường không `omitempty`, `map[string]any`); migration cả hai dialect (`DROP COLUMN`, `ADD COLUMN … NOT NULL` có/không default, `ALTER … TYPE`/`MODIFY`, `DROP POLICY`, `DISABLE ROW LEVEL SECURITY`, tệp trùng số, thiếu dialect, sửa tệp cũ). `PROVENANCE.txt` ghi từng cặp nhằm kiểm quy tắc nào.

## Kiểm thử

- Không test Go. Kiểm tay: mỗi `.proto` fixture biên dịch được bằng `buf build` **nếu** công cụ có sẵn trên máy tác giả (không bắt buộc); mỗi `.go` fixture parse được bằng `go/parser`.

## Tiêu chí hoàn thành

- [ ] Chữ ký ba bộ phân tích được ghi (hoặc đánh dấu "theo đặc tả").
- [ ] Kết luận về route HTTP.
- [ ] Fixture phủ mọi dòng bảng quy tắc.

## Rủi ro và lưu ý

- Fixture tay có thể bỏ sót ca thật; bổ sung từ corpus repo ở TASK-038-07 (golden trên cặp commit thật, nightly).
