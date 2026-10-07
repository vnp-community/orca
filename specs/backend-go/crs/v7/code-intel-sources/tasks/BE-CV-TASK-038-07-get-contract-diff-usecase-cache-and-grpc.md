# BE-CV-TASK-038-07: Use case `GetContractDiff`, giới hạn, cache, handler gRPC và golden

**From Solution:** BE-CV-SOL-038-contract-diff
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/get_contract_diff.go`, `get_contract_diff_test.go`; `internal/adapter/grpc/contract_diff_handler.go`, `contract_diff_handler_test.go`; `testdata/contract-diff/golden/*.json` (mới); `cmd/server/main.go` (sửa: đăng ký)
**Depends on:** BE-CV-TASK-038-03, 038-04, 038-05, 038-06; BE-CV-SOL-012/013/022
**Status:** [x] DONE

---

## Context

Solution 2.F. Quyền kiểm trước cache. Đầu ra làm giàu cho `BE-CV-SOL-036-*` qua port `TouchedContractSource` (hiện thực ở đây).

## Việc cần làm

1. `get_contract_diff.go`: cờ ⇒ `read` ⇒ `selector` ⇒ `ChangedFiles` ⇒ `SelectContractFiles` ⇒ `LoadVersions` ⇒ parse hai phía (`ProtoSchemaParser`, `WsChannelExtractor`, `MigrationCatalogBuilder`) ⇒ `DiffProto`, `DiffWsChannels`, `DiffRoutes` (nếu extractor), `ClassifyStatements`/`BuildTableImpacts` ⇒ `ContractDiff`. Parse lỗi một tệp ⇒ `ContractChange{ruleId:"parse-error", compatibility:"unknown", files:[path]}`; `Skipped` ⇒ `ruleId:"skipped"`.
2. `consumers` cho thay đổi `proto-rpc|proto-message` từ `RpcEdge` của catalog 032 (client gọi RPC đó); không có catalog ⇒ `[]`.
3. Giới hạn: ≤ 2 000 `ContractChange`, ≤ 500 `SqlChange`, payload ≤ 2 MiB; `summary` đếm trước cắt; cắt giữ thứ tự `breaking > risky > unknown > compatible`.
4. Cache `graph_snapshots(view="contractDiff")` khoá `(tenant, binding, view, head_commit, params_hash)`, `params_hash` gồm `mergeBase|kinds|detail|ContentHash` các tệp; cây bẩn ⇒ bộ nhớ TTL 30 s; singleflight.
5. `TouchedContractSource`: hàm trả `[]TouchedContract` (`kind` `proto-rpc|ws-channel|route|file`, `compatibility`, `breaking = compatibility=="breaking"`) và cờ `MigrationDestructive` cho 036.
6. Handler: ánh xạ domain⇄proto; lỗi `CODEINTEL_*`; `ResultMeta` phẳng; `CODEINTEL_TIMEOUT` có hậu tố.
7. Golden: cặp commit thật nhỏ trong repo Orca (chạy thủ công/nightly, không chặn PR): ghi `unknown` ratio của `ws.*`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/usecase/ -run ContractDiff -race` với cổng đọc/giả bộ parse: tệp mới/xoá/đổi tên; lỗi parse một tệp; `Skipped`; 301 tệp ⇒ `truncated`; `summary` trước cắt; **không có lời gọi `git.*` trực tiếp, không `.env`** (cổng giả ghi yêu cầu); cache hai tenant cùng `params_hash` không đọc chéo; quyền sai không chạm cổng.
- `go test ./services/code-intel-service/internal/adapter/grpc/ -run ContractDiff`.
- Hai dialect cho snapshot: ma trận của `BE-CV-SOL-022` (ghi rõ nếu chưa có).

## Tiêu chí hoàn thành

- [x] Toàn bộ §9 solution 038-contract-diff đạt trên fixture.
- [x] `buf breaking` xanh; kênh `codeIntel.contractDiff` chưa đăng ký ở gateway (việc của `BE-CV-SOL-040-codeintel-view-channels`).

## Rủi ro và lưu ý

- Hiệu năng: `GetContractDiff` điển hình < 5 s là ngân sách tạm (CR-071), chưa đo.
- Nếu 032 chưa merge, nhánh `ws.*`/`consumers` dùng fake và PR ghi rõ.
