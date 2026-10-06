# BE-CV-TASK-021-05: Chờ dev server kết nối lại (cổng theo tenant+dev server) và chính sách retry

**From Solution:** BE-CV-SOL-021-agent-collector
**Priority:** P1
**Service:** `code-intel-service`
**File:** `.../internal/adapter/infrafleetclient/dev_server_reconnect_wait.go` (mới), `.../agent_rpc_caller.go` (sửa) và test
**Depends on:** TASK-021-04
**Status:** [ ] TODO

---

## Context

`RelayByDevServer` trả lỗi ngay khi offline (đã đọc `relay_by_dev_server.go:55-57`); 20 s chờ là việc của collector (PQ-13).

## Việc cần làm

1. `gate` theo `tenantID|devServerID`: một goroutine thăm dò `IsDevServerConnected` (0,5/1/2/2 s ± 20 %), đánh thức mọi người chờ; ngân sách `CODEINTEL_RECONNECT_WAIT` 20 s; huỷ theo ctx.
2. Hết hạn → `CODEINTEL_DEV_SERVER_OFFLINE` (`KindUnavailable`).
3. Retry chỉ lệnh đọc: `Unavailable` ≤ 2 lần; `TOOL_FAILED` 1 lần nếu `retryable`.
4. `Reindex`/`Watch`: không chờ, không retry.

## Kiểm thử

- Đồng hồ giả: kết nối sau 3 s → đúng một `RelayByDevServer` sau khi kết nối; không bao giờ → 20 s ± 1 s `DEV_SERVER_OFFLINE`.
- 50 yêu cầu đồng thời → một chuỗi thăm dò (đếm trên giả); hai tenant không chia sẻ cổng.
- `go test ./internal/adapter/infrafleetclient/ -race -count=3`.

## Tiêu chí hoàn thành

- [ ] Các ca trên xanh.
- [ ] `Reindex` offline lỗi ngay.

## Rủi ro và lưu ý

- Agent thật backoff 1,2,5,15,30 s nên thường hết ngân sách.
