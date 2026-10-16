# BUG-QA-05: Chi tiết thực thi task hiện log lỗi `unrecognized_model` (sonnet-5-5)

**Mức độ:** P2 (gây hiểu nhầm; thực thi vẫn thành công) · **Khu vực:** frontend (panel Agent activity), agent/CLI · **Trạng thái:** đã sửa 2026-10-16 (dòng chẩn đoán CLI không còn hiện như lỗi); test đơn vị xanh, chạy thật native-execute 2/2 không còn lỗi giả

## Triệu chứng

Tab Request → Execution → mở task của #1252 (đã `completed`): ở cuối nhật ký agent, sau dòng "Done · 6 turns · 13.8s", xuất hiện dòng đỏ:

`! [claude-code:unrecognized_model] {"model":"claude-sonnet-5-5","query_source":"sdk"}`

## Tái hiện (2026-10-16)

Request #1252 → Execution → "Details" của task `Add add(a, b) …` → panel "Agent activity". Các dòng trước đó bình thường (Session started (claude-sonnet-5-5), Read/Edit/Bash đều `ok`, đoạn tóm tắt của agent). Dòng cuối là stderr.

## Nguyên nhân gốc

1. **Nguồn cảnh báo** (suy ra từ log, chưa đọc mã nguồn CLI): Claude Code CLI `2.1.266` trên dev server chưa có mã mô hình `claude-sonnet-5-5` trong danh mục của nó, nên ghi một chẩn đoán lên stderr (`[claude-code:unrecognized_model]`, `query_source: "sdk"`). Phiên vẫn chạy bằng mô hình được chọn (log "Session started (claude-sonnet-5-5)", mọi lệnh `ok`, kết thúc `Done`), không có lỗi thực thi. Không có mã nào trong `agent/src` tự sinh thẻ `[claude-code:…]` (đã grep), nên nó đến từ CLI.
2. **Giao diện coi mọi dòng stderr là lỗi.** `frontend/.../task/agent-activity/agent-activity-lines.ts` (`parseAgentOutput`, dòng ~85-95) gán `kind: 'stderr'`; `TaskAgentActivityPanel.tsx` `lineClass` tô `text-destructive` cho `stderr`. Không phân biệt chẩn đoán của CLI với lỗi, không xét kết quả cuối của phiên (`Done`).

## Giải pháp và task

[solutions/SOL-QA-05-agent-diagnostic-lines.md](./solutions/SOL-QA-05-agent-diagnostic-lines.md); T-QA-05-xx; test: vitest `agent-activity-diagnostics.test.ts` (đang `it.fails`, bỏ `.fails` khi sửa) và UI QA-UI-LOG-01.
