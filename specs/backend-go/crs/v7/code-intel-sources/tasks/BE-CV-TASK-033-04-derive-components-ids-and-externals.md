# BE-CV-TASK-033-04: Suy component, `id` ổn định, `kind`, `techHint`, external

**From Solution:** BE-CV-SOL-033-c4-component-view
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/c4/component.go`, `stable_ids.go` (mới), `backend-go/services/code-intel-service/internal/usecase/derive_c4_view.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-033-03, BE-CV-TASK-032-07
**Status:** [x] DONE

---

## Context

Solution mục 2.C.

## Việc cần làm

1. Quy tắc thư mục → component (domain, usecase, adapter lá/lồng, config, internal-*, main ẩn).
2. `id` ổn định `<kind-prefix>-<đường dẫn con>`; không phụ thuộc thứ tự.
3. `kind` adapter: grpc-server > grpc-client (dùng `ContractCatalog`) > adapter.
4. `techHint`/external từ bảng import cấu hình được (`config` đọc từ biến `CODEINTEL_C4_TECH_HINTS` hoặc hằng + test); `ext-svc-<service>` từ cạnh RPC; `ext-agent`.
5. Giới hạn 40 external, 200 component; `truncated`.
6. `description` từ package doc với `descriptionSource:"package-doc"`.

## Kiểm thử

`go test ./services/code-intel-service/internal/{domain/c4,usecase}/... -run Derive` (chưa chạy): id ổn định khi đảo thứ tự; `infra-fleet-service` đủ component; `api-gateway` (adapter lá `mcp*`) không panic.

## Tiêu chí hoàn thành

- [x] Mọi component có `origin:"derived"`.
- [x] `main` ẩn mặc định.

## Rủi ro và lưu ý

- Quy tắc cho `api-gateway`/`git-gateway-service` chưa thử.
