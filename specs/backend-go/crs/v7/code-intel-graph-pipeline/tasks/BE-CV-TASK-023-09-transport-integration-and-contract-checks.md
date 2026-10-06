# BE-CV-TASK-023-09: Kiểm thử tích hợp in-process, so khớp hợp đồng và hồi quy infra-fleet

**From Solution:** BE-CV-SOL-023-infra-fleet-codeintel-transport
**Priority:** P1
**Service:** `infra-fleet-service` · `proto` · CI
**File:** `backend-go/services/infra-fleet-service/internal/adapter/grpc/codeintel_transport_integration_test.go` (mới, `//go:build integration`), `.../internal/adapter/devserveragent/codeintel_golden_notifications_test.go` (mới), workflow infra-fleet hiện có trong `.github/workflows/` (sửa bước `buf`, nếu chưa gọi `buf` trực tiếp)
**Depends on:** TASK-023-02 … TASK-023-08
**Status:** [ ] TODO

---

## Context

CR-021 mục 5 yêu cầu một test tích hợp: infra-fleet thật in-process + `DevServerAgentClient` giả trả lỗi JSON-RPC `CODEINTEL_INDEX_MISSING`; mã phải tới client gRPC. Test đó **phải đỏ** trước SOL-023 (chứng minh vấn đề) và xanh sau. Ở đây dựng phần phía infra-fleet; phía collector là TASK-021-09.

## Việc cần làm

1. Test tích hợp: dựng `Server` thật với `RelayByDevServer`, `StreamCodeIntelEvents`, `GetAgentCapabilities` (dùng `devserveragent.Client` thật + `fakeAgent` WS như `client_test.go`), client gRPC thật qua `bufconn`; kiểm đường đầy đủ lỗi (mã + trailer) và sự kiện (agent phát `codeintel.indexChanged` → client stream nhận).
2. Test "tệp vàng" thông báo: nạp mẫu JSON thông báo của agent contract §6.1–6.4 (sao chép nguyên văn vào `testdata/codeintel_notifications/*.json`; khi `AG-CV-SOL-070` có `testdata/agent-results` thì đổi sang đọc từ đó) và khẳng định ánh xạ từng trường tới `CodeIntelEvent`.
3. Test phản chiếu tệp proto: `CodeIntelEvent` có đúng 20 field với số 1–20 như hợp đồng §2.3; `percent` là `optional`.
4. Chạy hồi quy: `cd backend-go/services/infra-fleet-service && go test ./... -race` so với baseline TASK-023-01; `go vet ./...`.
5. CI: bảo đảm bước `buf lint` + `buf breaking` được gọi trực tiếp cho `backend-go/proto/orca/infrafleet/**` (không `make proto-lint`).
6. Cập nhật mục "Known gaps"/README của `infra-fleet-service` (nếu có) nêu: timeout `codeintel.*`, trailer lỗi, ba RPC mới, bảng ánh xạ; không tạo file `.md` mới nếu README đã có.

## Kiểm thử

- `go test -tags=integration ./internal/adapter/grpc/ -run CodeIntelTransport -race`.
- `go test ./internal/adapter/devserveragent/ -run CodeIntelGolden -race`.
- `git stash`-style kiểm chứng đỏ: chạy ca "mã lỗi tới client" trên nhánh `main` chưa có thay đổi → thấy `INFRA_AGENT_EXEC_FAILED` (ghi vào PR; chỉ chạy cục bộ).

## Tiêu chí hoàn thành

- [ ] Test tích hợp xanh; ca mã lỗi đỏ trên `main` và xanh sau thay đổi (ghi vào PR).
- [ ] Số test đỏ so với baseline không tăng.
- [ ] CI gọi `buf` trực tiếp.
- [ ] Không có `max-lines` disable mới.

## Rủi ro và lưu ý

- Test tích hợp cần cổng cục bộ cho `fakeAgent`; dùng `httptest`/cổng 0 để tránh va chạm.
- Mẫu JSON của agent chưa được agent thật phát ra (agent contract §0: mọi mẫu chưa chạy); đây là kiểm thử hợp đồng, không phải bằng chứng hành vi agent.
