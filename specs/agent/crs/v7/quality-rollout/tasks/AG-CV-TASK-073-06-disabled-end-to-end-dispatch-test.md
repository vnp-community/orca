# AG-CV-TASK-073-06: Test tích hợp dispatcher thật: tắt rồi bật

**From Solution:** [AG-CV-SOL-073-agent-kill-switch](../solutions/AG-CV-SOL-073-agent-kill-switch.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/disabled-dispatch.integration.test.ts` (mới)
**Depends on:** 073-02, 073-03, 073-04
**Status:** [ ] TODO

## Context

Khuôn: `agent-rpc-dispatch-misc.test.ts` (`MockWs`, `createWireState`, `vi.mock`). BE e2e (BE-CV-SOL-073) dùng agent giả; test này bảo đảm agent **thật** đúng hợp đồng khi tắt, cung cấp tệp mẫu lỗi cho BE (`errors/tool-unavailable-codeintel-disabled.json` — điều phối với BE-CV-SOL-070 trước khi thêm vào tệp vàng).
Chưa chạy; mã dispatcher của AG-CV-SOL-001/081 chưa tồn tại: test import qua hằng đường dẫn ở đầu tệp.

## Việc cần làm

1. Dựng `RpcDispatcher` thật với `ORCA_CODEINTEL_DISABLED=1`; gửi `codeintel.symbol`, `codeintel.reindex`, `quality.run`, `codeintel.status`; kiểm khung gửi qua `ws.send`.
2. Dựng lại không biến → method chạy (CLI giả) và status bình thường.
3. Kiểm handshake giả có/không capability.

## Kiểm thử

- các kịch bản trên; lỗi đúng `error.code -32000`, `error.data.reason`

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Tắt/bật hoạt động trên dispatcher thật.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Cần CLI giả (AG-CV-TASK-071-04) cho nhánh "bật".
