# CR-WFD-006 — Vòng đời chạy: tạm dừng/huỷ thật, retry, timeout, idempotency, giới hạn đồng thời

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-006 |
| **Tên** | Điều khiển lỗi theo từng bước (`retry`, `timeoutMs`, `onError`), tạm dừng và huỷ có hiệu lực thật, trạng thái `waiting`/`cancelled` cho bước, idempotency khi phục hồi, chạy lại từ bước lỗi, giới hạn đồng thời |
| **Loại** | Feature + sửa lỗi nền |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-001 |
| **Mở khoá** | CR-WFD-004 (cổng cần trạng thái chờ và huỷ), CR-WFD-008 |
| **Tác động** | `workflow-service` (`usecase/wave_dispatcher.go`, `execute.go`, `pause_execution.go`, `resume_execution.go`, `cancel_execution.go`, `recover_executions.go`, migration), `infra-fleet-service` (huỷ lệnh relay), `agent` (huỷ tiến trình), proto |

---

## 1. Bối cảnh và vấn đề

Đã đọc code ngày 2026-10-08:

1. **Lỗi là tất cả hoặc không.** Bất kỳ bước nào `failed` làm cả execution `failed`; bước đang chạy dở được chạy nốt, wave sau không dispatch (`wave_dispatcher.go:84-91,141-143`). Không có `continueOnError`: frontend khai báo `continueOnError` và `timeout` (`frontend/src/shared/workflow-types.ts:44-45`) nhưng backend không có tham chiếu nào (chỉ một dòng chú thích chứa chữ "retry" ở `execute.go:225`).
2. **Không có retry hay backoff** ở bất kỳ tầng nào của service.
3. **Không có timeout theo bước** trong dispatcher; README nêu "30 phút mặc định" nhưng không có mã nào thực thi. Giới hạn thật là của agent: 5 phút mặc định, trần 15 phút (agent) / 5 phút (shell); workflow-service không truyền `timeoutMs` (`agent-print-mode-exec.ts:30-32`, `shell-agent-extensions.ts:86-88`).
4. **Tạm dừng chỉ đổi cột.** `PauseExecution` đặt `status=paused`; dispatcher không kiểm tra `paused` (không có tham chiếu trong `wave_dispatcher.go`); `ResumeExecution` đặt lại `running` nhưng không chạy lại dispatcher (`resume_execution.go:28-55`). Kết quả: đang chạy thì tạm dừng không dừng được; đã dừng thì tiếp tục không chạy tiếp.
5. **Huỷ chỉ đổi cột.** Không có hủy ngữ cảnh cho goroutine đang chạy (chưa kiểm chứng ngoài việc không thấy tham chiếu); lệnh relay đang chạy trên dev server tiếp tục cho tới khi xong hoặc hết giờ.
6. **Phục hồi có thể lặp tác dụng phụ.** `RecoverExecutions` khi khởi động chạy lại từ wave đầu chưa thành công (`main.go:291-302`); `dispatch_token` có lưu nhưng **không dùng để chống lặp** (migration 0004).
7. **Không có giới hạn đồng thời.** Pool 10 bước/wave (`wave_dispatcher.go:73`) chỉ giới hạn trong một execution; không giới hạn số execution đồng thời toàn tenant, nên một tenant có thể chiếm hết tài nguyên.
8. **Không chạy lại từ bước lỗi.** Execution `failed` là hết; muốn thử lại phải chạy lại toàn bộ, lặp mọi tác dụng phụ của các bước đã thành công.

## 2. Giải pháp đề xuất

### 2.1 Điều khiển lỗi theo bước

Các trường của bước (README 5.1), kiểm tra lúc lưu (CR-WFD-001):

| Trường | Giá trị | Mặc định |
|---|---|---|
| `timeoutMs` | 1 000 … `maxTimeout` của loại bước | theo loại bước (CR-WFD-005) |
| `retry.max` | 0…5 | 0 |
| `retry.backoffMs` | 100…600 000, nhân đôi mỗi lần, có jitter ±20 % | 1 000 |
| `retry.on` | tập `{transient, timeout, any}` | `transient` |
| `onError` | `fail`, `continue`, `skip_dependents` | `fail` |

- Phân loại lỗi do executor trả: `transient` (mạng, dev server không kết nối, 5xx) khác `permanent` (cấu hình sai, quyền, 4xx, biểu thức). **Chỉ lỗi `transient`/`timeout` được retry theo mặc định**; `retry.on=any` phải khai rõ.
- **Chỉ retry bước `idempotent`** (`Capabilities.idempotent`, CR-WFD-005) hoặc bước khai `retry.acknowledgeSideEffects: true`. Bước không idempotent mà có `retry.max>0` mà thiếu khai báo bị từ chối lúc lưu.
- `onError`:
  - `fail`: execution `failed` như hiện nay.
  - `continue`: bước `failed` nhưng luồng tiếp tục; bước sau đọc được `steps.<id>.status="failed"` (CR-WFD-002).
  - `skip_dependents`: các bước phụ thuộc trực tiếp và gián tiếp thành `skipped(reason=upstream_failed)`; các nhánh độc lập tiếp tục; execution cuối là `failed` nếu có bước `failed` mà không được `continue` che.
- Mỗi lần thử ghi một dòng `step_attempts(execution_id, step_id, attempt, status, error_code, started_at, ended_at)` (bảng mới) để truy vết.

### 2.2 Timeout thật

Dispatcher đặt `context.WithTimeout` quanh `Execute` của executor theo `timeoutMs`, đồng thời **truyền xuống relay** (`timeoutMs` trong params). Hết giờ → hủy ngữ cảnh, yêu cầu agent dừng tiến trình (2.4), bước `failed` mã `WORKFLOW_STEP_TIMEOUT`, là lỗi `timeout` đủ điều kiện retry. Execution có thêm `deadlineMs` tuỳ chọn (tổng thời gian chạy tối đa).

### 2.3 Máy trạng thái

Trạng thái ở [README 5.2](./README.md#52-trạng-thái). Quy tắc chuyển:

| Từ | Sang | Điều kiện |
|---|---|---|
| `pending` | `running` | dispatcher nhận |
| `running` | `waiting` | không còn bước chạy được và có ít nhất một bước `waiting` (CR-WFD-004) |
| `waiting` | `running` | điều kiện chờ được thoả |
| `running`/`waiting` | `paused` | `PauseExecution` |
| `paused` | `running`/`waiting` | `ResumeExecution` |
| không cuối | `cancelled` | `CancelExecution` |
| `running` | `completed`/`failed` | mọi bước ở trạng thái cuối (CR-WFD-002 2.3) |

Chuyển trạng thái dùng **CAS theo `version` của execution** (cột `row_version`) để hai thao tác đồng thời không ghi đè nhau.

### 2.4 Tạm dừng và huỷ thật

- Dispatcher chuyển thành vòng lặp **dựa trên trạng thái DB**, không phải "chạy một mạch rồi thoát": mỗi lần chọn bước tiếp theo đọc trạng thái execution. `paused` → không dispatch bước mới; bước đang chạy **chạy nốt** (không ngắt giữa chừng, vì có thể có tác dụng phụ dở dở), rồi dispatcher nhường. Đây là quyết định có chủ ý: tạm dừng = "không bắt đầu thêm", không phải "đóng băng".
- `ResumeExecution` đặt lại trạng thái và **đánh thức dispatcher** (kênh nội bộ + bộ quét định kỳ làm lưới an toàn), nên tiếp tục thực sự chạy tiếp.
- `CancelExecution`: hủy ngữ cảnh của mọi bước đang chạy, gửi lệnh dừng qua relay cho các bước `Capabilities.cancellable` (`agent.kill` hoặc tương đương; chưa kiểm chứng đã đủ phương thức phía agent), đặt bước còn lại `cancelled`, đóng cổng mở (CR-WFD-004), huỷ execution con (CR-WFD-002). Bước không `cancellable` được chạy nốt rồi `cancelled` ở bước kế tiếp; trạng thái execution là `cancelled` ngay khi nhận yêu cầu, kèm danh sách bước còn chạy để UI hiển thị trung thực.

### 2.5 Idempotency và phục hồi

- Mỗi lần dispatch một bước tạo `dispatch_token` mới (UUID) lưu **trước** khi gọi executor; executor nhận token làm `Idempotency-Key` (nếu loại bước hỗ trợ, ví dụ webhook, `mcp_call`).
- Khi phục hồi sau khởi động lại, bước ở trạng thái `running`:
  - `idempotent` → chạy lại.
  - không `idempotent` → đặt `failed` mã `WORKFLOW_STEP_INTERRUPTED` và áp `onError`; **không tự chạy lại** để tránh lặp tác dụng phụ. Người dùng quyết định chạy lại (2.6).
- Bỏ hành vi "chạy lại từ wave đầu chưa thành công" cho bước không idempotent; chỉ các bước chưa bắt đầu mới được dispatch.

### 2.6 Chạy lại từ bước lỗi (O6)

`RerunExecution{executionId, fromStepIds?}` tạo **execution mới** ghim cùng `template_version` và `inputs`; các bước đã `completed` ở lượt cũ được **sao chép kết quả** (output, artifact tham chiếu) và đánh dấu `reused_from`; chỉ chạy các bước `failed/cancelled/skipped(upstream_failed)` và hậu duệ. Không sửa execution cũ. Bước không idempotent đã chạy dở ở lượt cũ cần xác nhận rõ (`confirmReRunOfInterrupted: true`).

### 2.7 Giới hạn đồng thời

Hạn mức theo tenant (cấu hình, mặc định đề xuất ở CR-WFD-010): số execution `running` tối đa, số bước đồng thời toàn tenant, số execution `waiting` tối đa. Vượt hạn mức: `Execute` trả `WORKFLOW_QUOTA_EXCEEDED` (không xếp hàng ngầm ở MVP); tuỳ chọn hàng đợi là P2.

## 3. Tiêu chí chấp nhận

1. Bước `onError=continue` lỗi: execution vẫn chạy các bước còn lại; trạng thái cuối `completed` với bước đó `failed` hiển thị được.
2. Bước lỗi `transient` với `retry.max=2` thử đúng tối đa 3 lần, khoảng cách tăng theo backoff; mỗi lần có một dòng `step_attempts`.
3. Bước khai `retry.max>0` mà không idempotent và thiếu `acknowledgeSideEffects` bị từ chối lúc lưu.
4. `timeoutMs` nhỏ hơn thời gian bước chạy: bước `failed` với `WORKFLOW_STEP_TIMEOUT`, tiến trình trên dev server bị yêu cầu dừng.
5. `PauseExecution` khi đang chạy: không bước mới nào bắt đầu sau đó; `ResumeExecution` làm luồng tiếp tục không cần can thiệp khác.
6. `CancelExecution` làm bước đang chạy được hủy (loại `cancellable`), mọi bước còn lại `cancelled`, execution `cancelled`.
7. Khởi động lại service giữa lúc bước không idempotent đang chạy: bước `failed` mã `WORKFLOW_STEP_INTERRUPTED`, **không** chạy lại tự động.
8. `RerunExecution` tạo execution mới, sao chép kết quả các bước đã xong và chỉ chạy phần còn lại.
9. Hai thao tác `Pause` và `Cancel` đồng thời dẫn tới một trạng thái cuối nhất quán (CAS), không bị ghi chồng.
10. Vượt hạn mức tenant nhận `WORKFLOW_QUOTA_EXCEEDED`.
11. Test đua (race) có bật `-race`, test hai dialect cho `step_attempts` và `row_version`.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Migration | Thêm `row_version`, `deadline_ms` vào `executions`; thêm `attempt`, `skip_reason`, `reused_from` vào `step_executions`; tạo `step_attempts` |
| Tương thích | Hành vi "một bước lỗi là cả execution lỗi" vẫn là mặc định (`onError=fail`) nên template cũ không đổi nghĩa |
| Rủi ro | Viết lại dispatcher là thay đổi lớn nhất của series; cần test race và test phục hồi trước khi bật; đặt sau cờ `workflow_engine_v2` (CR-WFD-010) |
| Rủi ro | Huỷ phụ thuộc phương thức dừng phía agent (`agent.kill`…), chưa xác nhận bao phủ cả `shell.exec` |
| Rủi ro | Retry bước agent có thể tốn chi phí AI lặp; mặc định `retry.max=0` cho bước agent |

## 5. Chưa kiểm chứng / câu hỏi mở

- Phương thức agent để dừng một `agent.execPrompt` hoặc `shell.exec` đang chạy (có/không, tên).
- Cột `executions` đang giữ snapshot DAG và cơ chế khoá hiện tại (có `SELECT … FOR UPDATE` hay không).
- Tần suất bộ quét an toàn và tải của nó chưa đo.
