# AG-CV-TASK-073-02: Cổng tắt cứng đứng đầu dispatcher `codeintel.*` và `quality.*`

**From Solution:** [AG-CV-SOL-073-agent-kill-switch](../solutions/AG-CV-SOL-073-agent-kill-switch.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/disabled-gate.ts` + `.test.ts` (mới); sửa `dispatchCodeIntelRpc`, `dispatchQualityRpc` (AG-CV-SOL-001/081) thêm một lời gọi
**Depends on:** 073-01; AG-CV-SOL-001, 081
**Status:** [x] DONE

## Context

Solution 2.3: mọi method trừ `codeintel.status` → `-32000` `CODEINTEL_TOOL_UNAVAILABLE`, `data:{code, reason:"codeintel_disabled"}` (reason mới, đề nghị bổ sung §3.2); **không** `-32601` (Go đổi `-32601` thành `CODEINTEL_AGENT_UNSUPPORTED`, sai nghĩa); kiểm trước validate tham số, trước `workspaceRoot`, trước cache.
`makeError(id, code, message, data?)` giữ `data` (đã đọc `agent-rpc-dispatch.ts`).
`status` luôn thành công (§4.1).
Chưa chạy; mã dispatcher của AG-CV-SOL-001/081 chưa tồn tại: test import qua hằng đường dẫn ở đầu tệp.

## Việc cần làm

1. `gateDisabled(rpc, switches)` trả phản hồi lỗi hoặc `null`; `status` được ngoại lệ và dựng phong bì "tắt" (không spawn/fs/git; `tools.*.available:false, supported:false`, `indexes.*.state:"unknown"`, `binding:null`, `warnings:["codeintel_disabled"]`).
2. Cắm vào đầu hai dispatcher; `reindexDisabled` → `reason:"reindex_disabled"` cho `codeintel.reindex`; `qualityDisabled` → `reason:"quality_disabled"` cho `quality.run`.
3. Thông điệp lỗi ≤ 300, không đường dẫn.

## Kiểm thử

- Lặp theo `CODEINTEL_METHODS` và `QUALITY_METHODS`: mỗi method (trừ status) bị chặn; thêm method mà quên cổng → đỏ
- `-32601` không bao giờ; khoá lạ trong params vẫn trả `TOOL_UNAVAILABLE` (không `INVALID_PARAMS`, không oracle)
- spy `child_process`, `fs`, `git` không được gọi
- `status` thành công, hình dạng phong bì chuẩn, `warnings` đúng
- `reindexDisabled` chỉ chặn `reindex`; `qualityDisabled` chỉ chặn `quality.run`
- Test xanh.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel/disabled-gate.test.ts` (5 passed).

## Tiêu chí hoàn thành

- [x] Bảng 2.3 đúng cho mọi method.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Thứ tự cổng trước validate khác hợp đồng §2.1 (validate trước) chỉ khi tắt; chấp nhận.
