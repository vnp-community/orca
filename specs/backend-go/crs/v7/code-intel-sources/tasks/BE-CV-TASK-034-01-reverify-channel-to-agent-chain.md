# BE-CV-TASK-034-01: Re-verify chuỗi kênh → RPC → handler → use case → cổng → adapter và chọn luồng golden

**From Solution:** BE-CV-SOL-034-data-flow-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/testdata/dataflow/CHAIN_NOTES.md` (mới), `.../testdata/dataflow/mini-flow/` (mới)
**Depends on:** BE-CV-TASK-032-07, BE-CV-TASK-033-01
**Status:** [ ] TODO

---

## Context

CR §2.5 chuỗi `accounts.selectClaude` mới đọc từng mảnh. Cần đọc hết và chọn 3 luồng golden.

## Việc cần làm

1. Đọc và ghi dòng: `channels_accounts.go` (đăng ký 4 kênh), `infra-fleet-service/internal/adapter/grpc/server.go` (`RelayByDevServer` → `relayByDevServer.Execute`), `usecase/relay_by_dev_server.go`, `ports.go` (`DevServerRepository`, `DevServerAgentClient`), `adapter/{postgres,mysql}/repository.go` (hàm `Get`, `FROM infra.dev_servers`), `devserveragent/client.go` (`Exec`, `IsConnected`).
2. Xác định hàm cụ thể chứa `SELECT … FROM infra.dev_servers` (CR: chưa xác định) cho cả hai dialect.
3. Chọn 3 luồng golden: `accounts.selectClaude`; một luồng qua ≥ 2 service (tìm kênh gọi handler có client sang service khác); một luồng tới RPC `Unimplemented` (nếu có kênh gọi `ListFleetDefinitions` hoặc tương tự; nếu không, dùng fixture tổng hợp).
4. Dựng `mini-flow/` (cây Go rút gọn: wscompat giả → grpc server → usecase → port → hai adapter).
5. Ghi mọi nơi chuỗi vỡ (closure, goroutine, hàm trung gian) → sẽ thành `partial`.

## Kiểm thử

Không test mã; review `CHAIN_NOTES.md`.

## Tiêu chí hoàn thành

- [ ] Chuỗi 5 bước có file:dòng thật cho cả hai dialect.
- [ ] 3 luồng golden đã chọn, có lý do.

## Rủi ro và lưu ý

- Nếu không có kênh nào tới RPC `Unimplemented`, dùng fixture; ghi rõ.
