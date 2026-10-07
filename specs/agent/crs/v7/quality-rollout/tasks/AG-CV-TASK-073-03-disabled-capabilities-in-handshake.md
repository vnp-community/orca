# AG-CV-TASK-073-03: Bỏ capability `codeintel*`/`quality` khỏi handshake khi tắt

**From Solution:** [AG-CV-SOL-073-agent-kill-switch](../solutions/AG-CV-SOL-073-agent-kill-switch.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** sửa `agent/src/relay/agent-session-capabilities.ts` (`buildCapabilities`) hoặc điểm AG-CV-SOL-001 thêm capability; `agent/src/relay/codeintel/disabled-capabilities.test.ts` (mới)
**Depends on:** 073-01; AG-CV-SOL-001, 081
**Status:** [x] DONE

## Context

`buildCapabilities(config, log)` trả mảng chuỗi tự do; `STATIC_CAPABILITIES_FALLBACK` không có `codeintel*` (đã đọc). Hợp đồng §1.3: thêm `codeintel`, `codeintel.gitnexus`, `codeintel.codegraph` khi có binary; `quality` khi có profile `ready`. Tắt cứng → không thêm (đề xuất; hợp đồng im lặng, solution D4). `tools[]` giữ nguyên.
Chưa chạy; mã dispatcher của AG-CV-SOL-001/081 chưa tồn tại: test import qua hằng đường dẫn ở đầu tệp.

## Việc cần làm

1. Truyền `RuntimeSwitches` vào nơi thêm capability; khi `codeintelDisabled` bỏ cả bốn; khi `reindexDisabled`/`qualityDisabled` vẫn quảng cáo (method tồn tại; từ chối theo `reason`).
2. Không đổi chữ ký hiện có bằng cách bắt buộc tham số mới (tham số tuỳ chọn mặc định đọc từ env) để test cũ giữ nguyên.

## Kiểm thử

- tắt → không có `codeintel*`, `quality`; `tools[]` không đổi
- bật (binary giả) → có lại
- handshake giả (`agent-session-handshake`) mang đúng `capabilities`
- STATIC fallback không thêm gì
- Test xanh; các test `buildCapabilities` hiện có xanh không sửa.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel/disabled-capabilities.test.ts src/relay/agent-session-capabilities-codeintel.test.ts` (7 passed).

## Tiêu chí hoàn thành

- [x] Backend không thấy capability khi tắt.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Backend có thể cache capability (CR-023); hiệu lực sau lần kết nối lại.
