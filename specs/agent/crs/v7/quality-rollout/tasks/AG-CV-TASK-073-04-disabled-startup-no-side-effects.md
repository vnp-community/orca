# AG-CV-TASK-073-04: Khởi động khi tắt: không watcher, không quét nhật ký, một dòng log

**From Solution:** [AG-CV-SOL-073-agent-kill-switch](../solutions/AG-CV-SOL-073-agent-kill-switch.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/disabled-startup.test.ts` (mới); sửa điểm khởi tạo của AG-CV-SOL-004 (watcher, journal) và notification sink (AG-CV-SOL-001)
**Depends on:** 073-01, 073-02; AG-CV-SOL-004
**Status:** [ ] TODO

## Context

Hợp đồng §4.10-4.13, §6: watcher/thông báo/journal `~/.orca/codeintel/jobs/` chỉ có khi dùng. Khi tắt phải không tạo gì: không thư mục `0700`, không `fs.watch`, không đọc registry, không thông báo.
Log một dòng `warn`; không in giá trị biến.
Chưa chạy; mã dispatcher của AG-CV-SOL-001/081 chưa tồn tại: test import qua hằng đường dẫn ở đầu tệp.

## Việc cần làm

1. Cổng khởi tạo: nếu `codeintelDisabled` thì bỏ qua dựng watcher, notification sink, khôi phục journal, dò công cụ.
2. `log.warn("codeintel disabled by ORCA_CODEINTEL_DISABLED")` một lần.

## Kiểm thử

- spy `fs.watch`, `mkdir`, `readFile` registry → không gọi
- sink không phát thông báo
- log đúng một dòng, không chứa giá trị env
- bật lại (dựng mới) → khởi tạo bình thường
- Test xanh.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Không tác dụng phụ khi tắt.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Phụ thuộc cấu trúc khởi tạo của 004 (chưa có).
