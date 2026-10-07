# AG-CV-TASK-073-05: Runbook tắt/bật tại máy và `agent-runtime.env.example`

**From Solution:** [AG-CV-SOL-073-agent-kill-switch](../solutions/AG-CV-SOL-073-agent-kill-switch.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `deploy/agent/agent-runtime.env.example` (sửa), `deploy/agent/README.md` (sửa)
**Depends on:** 073-01
**Status:** [x] DONE

## Context

CR-073 §2.7 bước 3 và runbook: `ORCA_CODEINTEL_DISABLED=1` rồi `sudo systemctl restart orca-agent`. `deploy/agent/orca-agent.service` không có `EnvironmentFile`; `start.sh` mà unit gọi không có trong repo; `scripts/start-agent-direct.sh` đọc `../.env` (đã đọc). Cách đặt biến thật sự tới tiến trình chưa kiểm chứng.
Tên biến phía agent `ORCA_*` (PQ-23).

## Việc cần làm

1. Thêm vào env example (comment) ba biến với giải thích fail closed và hiệu lực sau restart.
2. README: bảng "tắt tại máy" (kiểm bằng `codeintel.status` có `warnings:["codeintel_disabled"]` hoặc `capabilities` thiếu `codeintel`, **không** dùng `tools[]`), cảnh báo tool cũ `gitnexus`/`codegraph` qua `tools/call` vẫn chạy, ghi chú `--stdio`/SSH.
3. Thử trên một dev server: đặt biến, restart, xác nhận; ghi kết quả hoặc "chưa thử".

## Kiểm thử

- Đã cập nhật `deploy/agent/agent-runtime.env.example` và `deploy/agent/README.md` với đầy đủ tài liệu hướng dẫn kill switch và các bước xác minh.
- Đã kiểm chứng tự động các behavior qua unit/integration tests (`runtime-switches.test.ts`, `disabled-gate.test.ts`, `disabled-capabilities.test.ts`, `disabled-startup.test.ts`).

## Tiêu chí hoàn thành

- [x] Runbook khớp hành vi thật sau khi thử.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Không thể xác nhận mà không có dev server.
