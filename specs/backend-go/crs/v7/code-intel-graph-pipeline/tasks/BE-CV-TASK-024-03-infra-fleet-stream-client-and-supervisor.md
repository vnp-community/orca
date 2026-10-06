# BE-CV-TASK-024-03: Client luồng infra-fleet (tái kết nối) và `Supervisor`

**From Solution:** BE-CV-SOL-024-event-distribution
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/adapter/infrafleetclient/code_intel_event_stream.go`, `.../internal/usecase/{code_intel_stream_supervisor.go,binding_stream_targets.go}` (mới) và test
**Depends on:** TASK-024-01, BE-CV-TASK-021-03, BE-CV-SOL-023
**Status:** [ ] TODO

---

## Context

SOL-024 2.B. Mẫu backoff: lịch reconnect của agent (1,2,5,15,30 s).

## Việc cần làm

1. Vòng mở/đọc/mở lại với backoff + jitter ± 20 %; reset khi nhận tin đầu hoặc sống ≥ 10 s; `Unauthenticated/NotFound` dừng hẳn.
2. `Supervisor` 60 s (`CODEINTEL_STREAM_RECONCILE`) + báo từ SOL-012; mở thiếu, đóng thừa, đóng khi tenant tắt cờ.
3. Sau mỗi mở thành công: phát `resync` nội bộ rồi gọi `Watch(true)` + `Status` từng binding (C9).
4. Tắt êm bằng `WaitGroup`; metrics `codeintel_event_stream_reconnects_total{reason}`, `..._connected` (nhãn giới hạn).

## Kiểm thử

- `go test ./internal/adapter/infrafleetclient/ ./internal/usecase/ -run 'EventStream|Supervisor' -race` (đồng hồ giả, infra-fleet giả).
- Ca: backoff đúng dãy; binding xoá → luồng đóng ≤ 60 s; gọi `Watch` sau nối lại; tenant tắt → đóng.

## Tiêu chí hoàn thành

- [ ] Các ca xanh. - [ ] Không rò goroutine.

## Rủi ro và lưu ý

- N replica × M dev server goroutine chưa đo.
