# Execution Contract: Hợp đồng thực thi và cổng sẵn sàng (v6)

> Điều kiện để một task gửi đi là AI làm được ngay và làm đúng: `TaskSpec` có cấu trúc, gói prompt render xác định, cổng sẵn sàng trước mỗi `Execute`, kết quả có cấu trúc, kiểm chứng độc lập sau chạy, phân loại lỗi. Bối cảnh, quyết định D1 đến D5 và hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-029](./CR-REQ-029-execution-contract-and-readiness-gate.md) | Prompt nối chuỗi (`buildExecutePrompt`), không có phạm vi, tiêu chí, Check, kết quả có cấu trúc; run `exit 0` là thành công; mọi lỗi bị đếm như nhau | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-027 (task_specs, AC-n) ─┐
CR-REQ-028 (Clarification)  ───┼─▶ CR-REQ-029 ─▶ CR-REQ-030 (đánh giá thực tế dùng files_changed đã kiểm chứng)
CR-REQ-033 (khối kết quả, changes, capabilities) ─┤
CR-REQ-035 (common/secretscan) ─┘      │
CR-REQ-011, 012, 013, 014 ─────────────┘  (CR này sửa điểm móc của 012, 013, 014: xem mục 9 của CR)
```

CR-REQ-029 chia được thành ba lớp độc lập về mặt giao hàng, theo thứ tự nên làm:
1. `TaskSpec` v2 và bộ render `ExecutionPacket` (không đổi hành vi chạy khi cờ tắt).
2. `ReadinessGate` và `TaskReadinessReport` (chỉ đọc, thử trước ở chế độ chạy khô qua `CheckReadiness`).
3. Phân tích `ExecutionResult` ở `task-service` và `VerifyExecution` ở `request-service` (cần CR-REQ-033 trên dev server).

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| E1 | `TaskSpec` là Task schema v2, cộng thêm vào v1 của CR-REQ-027; lưu ở `task_specs` của CR-REQ-027, không thêm bảng spec | Một nơi lưu spec; spec bị khoá sau duyệt Plan |
| E2 | Packet render ở `request-service` bằng hàm thuần có `digest` và `template_version`, gửi qua trường `prompt` có sẵn của `Execute` | Chỉ request-service có Request, Solution, Plan; không sửa `buildExecutePrompt` cho task cũ |
| E3 | Task có spec luôn chạy Engine 1 (`direct_agent`) | Ghi đè prompt chỉ chạy được ở `direct_agent` (`execute_task.go:179`) |
| E4 | Khối kết quả theo CR-REQ-033 (`ORCA_RESULT_BEGIN/END <nonce>`); schema nội dung do CR này định nghĩa, kiểm ở backend | Một định dạng, có nonce chống chèn |
| E5 | Cổng sẵn sàng chỉ đọc, ba tầng cấu trúc, ngữ nghĩa, môi trường; kết quả `ready`, `needs_info`, `spec_defect`, `env_defect` | Lỗi spec hay môi trường không được đốt lần thử của agent |
| E6 | Orca không tin lời agent: chạy lại Check, kiểm phạm vi bằng `git diff` từ `base_sha`, quét bí mật bằng `common/secretscan` | `trustPreset=full` cho agent ghi tuỳ ý; chỉ kiểm sau chạy phát hiện được |
| E7 | `Failure.class` ∈ `retryable|needs_info|spec_defect|env_defect|agent_defect`; chỉ `retryable` và `agent_defect` tính lần thử | Định tuyến đúng chỗ cần sửa |
| E8 | Toàn bộ sau cờ `REQUEST_EXECUTION_CONTRACT_ENABLED` (mặc định tắt) | Bật dần cùng `request_flow_enabled` |

## Phạm vi ngoài feature này

Định nghĩa Task schema v1 và `AC-n` (CR-REQ-027), Clarification (CR-REQ-028), chế độ chỉ đọc và khối kết quả phía agent (CR-REQ-033), `secretscan` (CR-REQ-035), điểm rủi ro (CR-REQ-030), giao diện (CR-REQ-036).

## Điểm lệch và điểm cần xác nhận khi viết feature này

- CR-REQ-011 mục 2.7 và CR-REQ-013 mục 2.1 giả định task có `depends_on` đi Engine 2; với task có spec phải đổi thành Engine 1 (xem E3).
- CR-REQ-014 coi `tests_modified` là lời khai của agent; CR này thay bằng số đo của Orca cho task có spec.
- `ReadinessReport` ở CR-REQ-028 là cấp Request; cấp task đặt tên `TaskReadinessReport` để khỏi trùng.
- Chưa kiểm chứng: agent trả khối kết quả ổn định; chạy lại Check trong cùng môi trường; độ trễ cổng; `ResolveConnection(worktree_id)` có trả đường dẫn worktree.
