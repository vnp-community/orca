# SOL-QA-05: Chẩn đoán của CLI không còn hiện như lỗi

**Bug:** [BUG-QA-05](../BUG-QA-05-unrecognized-model-log-shown-as-error.md)

## Phần 1 (frontend, chính)

`agent-activity-lines.ts`:
- Nhận dạng dòng stderr dạng `[claude-code:<code>] <json>` → `kind: 'diagnostic'` (kiểu mới trong `AgentActivityKind`), `tone: 'info'`, giữ nguyên văn bản.
- Quy tắc tông: `diagnostic` là `error` chỉ khi phiên kết thúc lỗi (dòng `end` có `is_error` hoặc thoát khác 0) hoặc `<code>` thuộc danh sách lỗi chặn (`auth_failed`, `rate_limit`, …). Mặc định (kể cả `unrecognized_model`) là trung tính (`text-muted-foreground`), thu gọn dưới một dòng "Diagnostics (1)" có thể mở.
- `TaskAgentActivityPanel.tsx`: `lineClass` thêm nhánh `diagnostic`; dòng stderr không có tag vẫn đỏ nếu phiên lỗi, trung tính nếu phiên `Done`.
- Văn bản thân thiện: với `unrecognized_model` thêm gợi ý "Your Claude Code CLI does not recognise model <m>. The run used it anyway; update the CLI to remove this notice." (qua `translate()`, năm locale).

## Phần 2 (agent/vận hành)

- Agent đã biết phiên bản CLI (probe `claude --version` khi kết nối): khi `claude-sonnet-5-5` (hoặc mô hình được cấu hình) mới hơn CLI, ghi một cảnh báo năng lực duy nhất (một lần mỗi kết nối) ở màn Dev server thay vì trên mỗi task.
- Tài liệu: nâng cấp CLI trên dev server (`claude update`) và ghi phiên bản tối thiểu cho mỗi mô hình mặc định.

## Kiểm thử cùng bản sửa

| Tầng | Test |
|---|---|
| FE unit | `agent-activity-diagnostics.test.ts`: dòng `[claude-code:unrecognized_model]` sau phiên `Done` không phải `stderr`/`error`; vẫn là `error` khi phiên lỗi; stderr không tag giữ hành vi cũ |
| FE component | `TaskAgentActivityPanel.test.tsx`: không có phần tử `text-destructive` cho chẩn đoán trên phiên thành công; mở "Diagnostics" thấy văn bản |
| UI live | QA-UI-LOG-01 (bản thật): sau thực thi native thật, mở log task: không có dòng đỏ chứa `unrecognized_model` |
