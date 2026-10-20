---
doc_id: SOL-CR-001
doc_type: solution
title: Solution CR-001
version: "1"
status: active
---

# Change spec CR-001: Chuyển tiền hẹn lịch và định kỳ (MiniPay)

Nguồn: `cr.md`, `srs.md` (DOC-SRS-CR001), `impact.json`, `tdd_patch.json`, KG `tdd-kg` (get_context_pack MOD-Scheduler, SCN-ScheduledRun, NFR-013; trace_requirement FR-045; get_integration_contract INT-01, INT-02) và mã nguồn `minipay/`, `tests/`, `scripts/`, `contracts/`.
Quy ước: "(TDD: X)" là căn cứ từ nút KG X. "(Đề xuất solution)" là điều TDD chưa quy định; mục đó nằm trong §7 để Tech Lead xác nhận.

## 1. Phạm vi và yêu cầu

CR-001 thêm module lập lịch và hai bảng dữ liệu mới. Luồng chuyển tiền hiện có (idempotency, hạn mức, Fraud Check, sổ cái, outbox, audit) được tái sử dụng, không có đường ghi sổ thứ hai.

| Mã | Yêu cầu (rút gọn) | Ưu tiên | Hiện thực chính |
| --- | --- | --- | --- |
| FR-040 | Tạo lịch một lần trong tương lai | Must | MOD-API, MOD-Scheduler (DI-SchedAPI, DI-ScheduledRepo) |
| FR-041 | Tạo lịch DAILY/WEEKLY/MONTHLY có `endDate` hoặc `maxRuns` | Must | như trên |
| FR-042 | Tra cứu lịch, danh sách lịch, các lần chạy | Must | DI-SchedAPI, DI-ScheduledRepo |
| FR-043 | Hủy lịch | Must | DI-SchedAPI, DI-ScheduledRepo |
| FR-044 | Tạm dừng / tiếp tục lịch định kỳ | Should | DI-SchedAPI, DI-ScheduledRepo |
| FR-045 | Đến hạn tự chuyển tiền bằng luồng hiện có; Idempotency-Key = mã lịch + thời điểm dự kiến | Must | DI-Scheduler, DI-TransferCore, MECH-RunClaim |
| FR-046 | Thất bại theo BR-13: thử lại, FAILED, tự PAUSED sau 3 FAILED liên tiếp | Must | DI-Scheduler, MECH-ScheduleRetry |
| FR-047 | SMS kết quả mỗi lần chạy qua outbox | Should | DI-SchedNotify, MSG-INT02-scheduled-run |
| FR-048 | Audit tạo/hủy/tạm dừng/tiếp tục và mọi kết quả lần chạy | Must | DI-SchedAudit |
| FR-049 | Chạy bù lần quá hạn ≤ 24 giờ, quá hạn thì MISSED | Should | DI-Scheduler, MECH-CatchUp |
| NFR-010 | 99% lần chạy bắt đầu trong 60 giây | Must | AD-05, MOD-Scheduler, SCN-ScheduledRun |
| NFR-011 | 0 giao dịch trùng cho một lần chạy | Must | MECH-RunClaim |
| NFR-012 | Quá hạn trong 24 giờ được xử lý trong 5 chu kỳ quét đầu | Must | MECH-CatchUp |
| NFR-013 | 100 lần chạy cùng lúc, p95 API ≤ 500 ms | Must | MECH-SchedulerThrottle, MECH-TimeoutBudget |
| NFR-014 | 100% lần chạy có correlationId riêng, đi theo giao dịch, audit, Fraud Check | Must | MECH-Correlation |

Ràng buộc: CON-010 (scheduler cùng tiến trình và cùng CSDL). Giả định đã xác nhận trong patch: ASM-010 (quét 30 giây đủ), ASM-011 (giới hạn 20 lần chạy mỗi chu kỳ, 10 rps; ASM-001 vẫn Open). Câu hỏi đã chốt: OQ-010 (thiếu số dư: thử lại 2 lần, cách 1 giờ), OQ-011 (20 lịch mỗi tài khoản), OQ-012 (chạy bù 24 giờ).
Ngoài phạm vi: giữ chỗ số dư, sửa lịch, đa tiền tệ, giao diện web/mobile, direct debit.

## 2. Hiện trạng codebase

| Tệp | Ký hiệu hiện có | Liên quan |
| --- | --- | --- |
| `minipay/api.py` | `create_app(db_path, fraud)`, middleware `correlation` (X-Correlation-Id), handler `MiniPayError`, `caller()`, route `/v1/transfers`, `/v1/accounts/{aid}/transfers`, `/admin/outbox/run`, `_default()` | Thêm 6 route (DI-SchedAPI) và khởi động vòng lặp scheduler |
| `minipay/service.py` | `transfer(con, caller, body, idem_key, cid, fraud)` (validate → idempotency → limit/balance → Fraud Check → ghi sổ + outbox + audit), `audit()`, `authenticate()`, `balance_of()`, `now()`, `history()`, `_view()` | Tách lõi `execute_transfer` (DI-TransferCore); thêm `audit_schedule_event` (DI-SchedAudit) |
| `minipay/db.py` | `SCHEMA` (`CREATE TABLE IF NOT EXISTS`), `connect()`, `init_db()`, `tx()` (BEGIN IMMEDIATE) | Thêm 2 bảng vào `SCHEMA`; không đổi bảng cũ |
| `minipay/errors.py` | `ERRORS` map mã → HTTP, `err()` | Thêm 5 mã `SCHEDULE_*` |
| `minipay/config.py` | hằng số `[TDD]`, `TZ_OFFSET_HOURS=7`, `QUEUE_TOPIC`, `FRAUD_*` | Thêm hằng số scheduler |
| `minipay/notify.py` | `relay_outbox()` (key theo `fromAccountId`), `consume()` (khử trùng `processed_events`, gửi SMS tối đa `SMS_MAX_ATTEMPTS`), `sms_to_log()` | Thêm nhánh mẫu SMS lần chạy (DI-SchedNotify) |
| `minipay/fraud_client.py` | `FraudClient.check()` (INT-01: timeout 0,3/0,8/1,0 s, retry 1, breaker 5 lần/30 s) | Không đổi (INT-01 giữ contract) |
| `contracts/transfer-completed.event.schema.json` | schema `additionalProperties:false` | Giữ nguyên; tạo schema mới cho sự kiện lịch |
| `tests/test_minipay.py` | `FakeFraud`, fixture `env(tmp_path)` + `acct()`, helper `tr()` | Mẫu cho `tests/test_scheduler.py` |
| `scripts/benchmark.py` | `SlowFraud`, ThreadPoolExecutor, ghi `tdd_kg/data/benchmark_result.json` | Mẫu cho `scripts/benchmark_scheduler.py` |

Quan sát ảnh hưởng thiết kế:
- Nhánh `reject()` trong `service.transfer` lưu bản ghi `REJECTED` cùng `(from_account, idempotency_key)`. Gọi lại với cùng khóa sẽ phát lại đúng lỗi cũ (`_stored_error`). Điều này ảnh hưởng tới cách đặt khóa khi thử lại (xem D-3 và OQ mới Q-S1).
- `now()` trả chuỗi ISO UTC (`+00:00`), nên so sánh thời điểm bằng chuỗi hoạt động nếu mọi giá trị lưu cùng định dạng.
- `notify.consume` hiện luôn gửi SMS tới người nhận (`toAccountId`) với nội dung "+amount"; sự kiện lịch phải gửi cho người gửi (FR-047).
- Chưa có `scheduler.py`, `tests/test_scheduler.py`, `scripts/benchmark_scheduler.py`, `contracts/scheduled-run.event.schema.json` (đều là tệp mới do patch khai báo).

## 3. Thiết kế thay đổi

### 3.1 Quyết định kiến trúc (AD-05)

AD-05: scheduler là vòng lặp nền trong tiến trình MiniPay, quét mỗi 30 giây, chiếm lần chạy bằng `UPDATE` có điều kiện. Căn cứ: CON-010, NFR-010 (30 giây < 60 giây, ASM-010), NFR-011. Phương án bị loại: ALT-DelayedQueue (thêm hạ tầng, vi phạm CON-010), ALT-OSCron (tách khỏi tiến trình/CSDL, khó chống chạy trùng), ALT-APScheduler (thêm phụ thuộc không cần thiết).

### 3.2 Dữ liệu (DE-ScheduledTransfer, DE-ScheduledRun) — tệp `minipay/db.py`

Thêm vào `SCHEMA` (DI-ScheduledRepo sở hữu truy cập). Bảng cũ giữ nguyên; giao dịch sinh ra vẫn nằm trong `transfers` (TDD: DE-ScheduledRun —relatesTo→ DE-Transfer, 1:0..1).

`scheduled_transfers` (DE-ScheduledTransfer, lưu 5 năm):

| Cột | Ghi chú |
| --- | --- |
| `id` TEXT PK | `SCH-` + 12 ký tự hex hoa |
| `from_account`, `to_account`, `amount`, `note` | như `transfers` |
| `recurrence` | ONCE / DAILY / WEEKLY / MONTHLY |
| `first_run_at` | UTC ISO; mỏ neo tính các lần sau |
| `end_date` | ngày (UTC+7), có thể NULL |
| `max_runs` | NULL hoặc 1..60 |
| `next_index` | số lần chạy đã sinh (chỉ số lần chạy kế tiếp) |
| `status` | ACTIVE / PAUSED / COMPLETED / CANCELLED |
| `consecutive_failures` | đếm FAILED liên tiếp |
| `idempotency_key`, `request_hash` | `UNIQUE(from_account, idempotency_key)` cho POST tạo lịch |
| `correlation_id`, `created_at` | |

`scheduled_runs` (DE-ScheduledRun, lưu 5 năm):

| Cột | Ghi chú |
| --- | --- |
| `id` TEXT PK | `RUN-` + 12 ký tự hex hoa (cũng là `eventId` của sự kiện) |
| `schedule_id`, `scheduled_at` | `UNIQUE(schedule_id, scheduled_at)`: không sinh trùng lần chạy |
| `status` | PENDING / RUNNING / SUCCEEDED / FAILED / SKIPPED / MISSED / CANCELLED |
| `attempts` | số lần đã chiếm; tăng trong chính câu `UPDATE` chiếm |
| `next_attempt_at` | NULL hoặc thời điểm thử lại |
| `transfer_id`, `reason_code` | kết quả |
| `correlation_id` | sinh riêng mỗi lần chiếm (NFR-014) |
| `started_at`, `finished_at` | |

Chỉ mục: `scheduled_runs(status, scheduled_at)` và `scheduled_transfers(from_account, status)`.

### 3.3 Cấu hình — `minipay/config.py`

| Hằng số | Giá trị | Căn cứ |
| --- | --- | --- |
| `SCHED_SCAN_INTERVAL_S` | 30 | AD-05, ASM-010 |
| `SCHED_MAX_RUNS_PER_SCAN` | 20 | MECH-SchedulerThrottle, ASM-011 |
| `SCHED_RATE_LIMIT_RPS` | 10 | MECH-SchedulerThrottle, ASM-011 |
| `SCHED_RETRY_FRAUD` | (3 lần, 300 s) | MECH-ScheduleRetry, BR-13 |
| `SCHED_RETRY_FUNDS` | (2 lần, 3600 s) | MECH-ScheduleRetry, OQ-010 |
| `SCHED_AUTO_PAUSE_AFTER` | 3 | FR-046 |
| `SCHED_CATCHUP_WINDOW_S` | 86400 | MECH-CatchUp, OQ-012 |
| `SCHED_MAX_ACTIVE_PER_ACCOUNT` | 20 | BR-15, OQ-011 |
| `SCHED_MIN_LEAD_S` / `SCHED_MAX_LEAD_DAYS` | 300 / 90 | BR-10 |
| `SCHED_MAX_RUNS_LIMIT` / `SCHED_MAX_SPAN_DAYS` | 60 / 366 | BR-11 |
| `SCHED_RUN_LEASE_S` | 120 | Đề xuất solution (Q-S2): hết hạn thì RUNNING bị chiếm lại |

### 3.4 Mã lỗi — `minipay/errors.py`

Thêm vào `ERRORS`: `SCHEDULE_NOT_FOUND` 404, `SCHEDULE_TIME_INVALID` 422, `SCHEDULE_LIMIT_REACHED` 422, `SCHEDULE_NOT_CANCELLABLE` 409, `SCHEDULE_STATE_INVALID` 409 (SRS bổ sung). Mã cũ (INVALID_REQUEST, UNAUTHORIZED, FORBIDDEN, ACCOUNT_NOT_FOUND, IDEMPOTENCY_CONFLICT, SAME_ACCOUNT, LIMIT_EXCEEDED…) dùng lại.

### 3.5 Lõi chuyển tiền dùng chung — DI-TransferCore (`minipay/service.py::execute_transfer`)

Tách `transfer()` thành hai phần, hành vi API không đổi (36 test hiện có phải vẫn đạt):
- `transfer(con, caller, body, idem_key, cid, fraud)` giữ: kiểm tra `idem_key`, trường bắt buộc, `frm != caller → FORBIDDEN`; sau đó gọi `execute_transfer`.
- `execute_transfer(con, body, idem_key, cid, fraud) -> (view, replayed)`: từ bước tính `req_hash` trở đi (idempotency, SAME_ACCOUNT, ACCOUNT_NOT_FOUND, ACCOUNT_INACTIVE, hạn mức/số dư, Fraud Check ngoài giao dịch DB, ghi sổ + outbox `TransferCompleted` + audit). Không kiểm tra API key (scheduler gọi nội bộ).
- Căn cứ: MOD-Transfer, DI-Transfer, FR-045, impact.json (MOD-Transfer update).

BR-16: `history()`/`get_transfer()` bổ sung trường `scheduleId` (LEFT JOIN `scheduled_runs.transfer_id`), chỉ thêm trường, không đổi trường cũ (Đề xuất solution, Q-S6).

### 3.6 Scheduler — DI-Scheduler, DI-ScheduledRepo (tệp mới `minipay/scheduler.py`)

DI-ScheduledRepo (`minipay/scheduler.py::create_schedule` và các hàm kho):
- `create_schedule(con, caller, body, idem_key, cid) -> (view, replayed)`.
- `get_schedule(con, sid, caller)`, `list_schedules(con, acc_id, caller, status, limit, offset)`, `list_runs(con, sid, caller, limit, offset)`.
- `cancel_schedule(con, sid, caller, cid)`, `set_status(con, sid, caller, status, cid)` (PAUSED/ACTIVE).
- `occurrence(first_run_at, recurrence, index) -> datetime`: tính trong UTC+7 từ mỏ neo `first_run_at`, không cộng dồn từ lần trước. DAILY +index ngày, WEEKLY +7×index ngày, MONTHLY +index tháng với ngày = `min(ngày mỏ neo, ngày cuối tháng)` (BR-12, CR1-R5). Nhờ vậy 31/01 → 28/02 (29/02 năm nhuận) → 31/03.
- `schedule_exhausted(schedule, index, when)`: `max_runs` đạt hoặc `occurrence` có ngày (UTC+7) sau `end_date` (BR-11, điều kiện nào đủ trước thì kết thúc).

DI-Scheduler (`minipay/scheduler.py::scan_and_run`):
- `scan_and_run(con, fraud, clock=utcnow, sleep=time.sleep, limit=C.SCHED_MAX_RUNS_PER_SCAN) -> int`: một chu kỳ quét, trả số lần chạy đã xử lý. `clock` và `sleep` tiêm được để test dùng đồng hồ giả (TC-S09).
- `start_scheduler(db_path, fraud, interval=C.SCHED_SCAN_INTERVAL_S) -> SchedulerHandle`: luồng daemon, mỗi vòng mở kết nối riêng, gọi `scan_and_run`, ngủ `interval`, có `stop()`. Bắt mọi ngoại lệ trong vòng lặp để luồng không chết (ghi log).
- `recover_stale_runs(con, clock)`: chuyển `RUNNING` có `started_at` cũ hơn `SCHED_RUN_LEASE_S` về `PENDING` (giữ nguyên `attempts`).

### 3.7 Cơ chế

- MECH-RunClaim: chiếm bằng một câu lệnh có điều kiện, `rowcount == 1` mới được chạy:
  `UPDATE scheduled_runs SET status='RUNNING', attempts=attempts+1, started_at=?, correlation_id=? WHERE id=? AND status='PENDING' AND COALESCE(next_attempt_at, scheduled_at) <= ?`. Hai scheduler cùng quét: một bên `rowcount=0` và bỏ qua (NFR-011, RISK-005, CR1-R1).
- Khóa idempotency của lần chạy (FR-045): lần chiếm đầu `attempts=1` dùng `"{scheduleId}:{scheduledAt}"` (scheduledAt là chuỗi UTC ISO của lần chạy); lần thử lại thứ n≥2 dùng `"{scheduleId}:{scheduledAt}#{n}"`. Lý do: lần bị REJECTED đã lưu bản ghi với khóa cũ và sẽ phát lại lỗi cũ mãi nếu dùng lại khóa. Mỗi khóa vẫn được cố định bởi `attempts` đã ghi trong lúc chiếm, nên chạy lại sau khởi động lại cùng `attempts` sẽ phát lại giao dịch đã có thay vì tạo mới (TC-S11). (Q-S1.)
- MECH-ScheduleRetry (BR-13, OQ-010): bảng phân loại ở §5.1. Chọn thử lại thì lần chạy về `PENDING` với `next_attempt_at = now + khoảng cách`; hết lượt hoặc lỗi vĩnh viễn thì `FAILED` kèm `reason_code`; sau `SCHED_AUTO_PAUSE_AFTER` lần FAILED liên tiếp (đếm ở lịch) thì lịch định kỳ `PAUSED`. `SUCCEEDED` đặt lại bộ đếm về 0.
- MECH-CatchUp (OQ-012, NFR-012): không có đường chạy riêng. Lần `PENDING` quá hạn nhưng ≤ 24 giờ so với `scheduled_at` vẫn nằm trong truy vấn "đến hạn" và được xử lý, tối đa 20 lần mỗi chu kỳ (nên 100 lần quá hạn xong trong 5 chu kỳ, khớp NFR-012). Đầu mỗi chu kỳ, lần `PENDING` quá hạn > 24 giờ chuyển `MISSED`, phát `ScheduledRunFailed` với `reasonCode=MISSED`, không tính vào bộ đếm FAILED liên tiếp.
- MECH-SchedulerThrottle (NFR-013, ASM-011): `LIMIT 20` mỗi chu kỳ và giãn tối thiểu 0,1 giây giữa hai lần gọi `execute_transfer` (10 rps) bằng `sleep` tiêm được. Truy vấn sắp theo `COALESCE(next_attempt_at, scheduled_at)` tăng dần.
- MECH-Correlation (NFR-014): mỗi lần chiếm sinh `uuid4` làm `correlation_id` của lần chạy; truyền vào `execute_transfer` (nên có trong `transfers.correlation_id`, `audit_logs.correlation_id`, tham số `fraud.check(cid, …)` và payload sự kiện). Lời gọi tạo lịch dùng correlationId của yêu cầu HTTP.
- MECH-TimeoutBudget (NFR-013): mỗi lần `execute_transfer` bị chặn bởi timeout INT-01 hiện có (tối đa 2 lời gọi × 1000 ms); không thêm timeout mới.

### 3.8 API — DI-SchedAPI (`minipay/api.py::scheduled_transfers`)

Thêm sáu route trong `create_app`, dùng lại `db()`, `caller()`, handler lỗi và middleware correlation; mỗi route đóng kết nối trong `finally` như route hiện có. `create_app(db_path, fraud, run_scheduler=False)`: khi `run_scheduler=True` gọi `start_scheduler` và dừng khi tắt ứng dụng; `_default()` truyền `True`. Test hiện có không bị ảnh hưởng vì mặc định `False` (Đề xuất solution, Q-S5).

| Route | Hàm | Yêu cầu |
| --- | --- | --- |
| `POST /v1/scheduled-transfers` (201, phát lại 200) | `create_schedule` | FR-040, FR-041 |
| `GET /v1/scheduled-transfers/{scheduleId}` | `get_schedule` | FR-042 |
| `GET /v1/accounts/{accountId}/scheduled-transfers` | `list_schedules` | FR-042 |
| `GET /v1/scheduled-transfers/{scheduleId}/runs` | `list_runs` | FR-042 |
| `DELETE /v1/scheduled-transfers/{scheduleId}` | `cancel_schedule` | FR-043 |
| `PATCH /v1/scheduled-transfers/{scheduleId}` | `set_status` | FR-044 |

Quy tắc kiểm tra khi tạo lịch (thứ tự; BR-10, BR-11, BR-14, BR-15):
1. Thiếu `Idempotency-Key`, thiếu `fromAccountId`/`toAccountId`/`firstRunAt`/`recurrence`, `amount` không phải số nguyên dương, `recurrence` ngoài enum, `firstRunAt` không parse được, định kỳ thiếu cả `endDate` lẫn `maxRuns`, `maxRuns` ngoài 1..60 → `INVALID_REQUEST` (400).
2. `fromAccountId != caller` → `FORBIDDEN`. `from == to` → `SAME_ACCOUNT`. Tài khoản đích không có → `ACCOUNT_NOT_FOUND`.
3. `amount > 50.000.000` → `LIMIT_EXCEEDED` (BR-14, BR-04).
4. `firstRunAt` ngoài [now+5 phút, now+90 ngày], `endDate` trước ngày của lần chạy đầu, hoặc `endDate` quá 366 ngày kể từ lần chạy đầu → `SCHEDULE_TIME_INVALID`. `firstRunAt` không có offset được hiểu là UTC+7 (giả định SRS bổ sung).
5. Trong `tx(con)`: đếm lịch ACTIVE/PAUSED của `fromAccountId` ≥ 20 → `SCHEDULE_LIMIT_REACHED`.
6. Idempotency: cùng `(from, key)` cùng nội dung → trả lịch cũ (200); khác nội dung → `IDEMPOTENCY_CONFLICT`.
Không trừ tiền, không kiểm tra số dư/trạng thái tài khoản lúc tạo (BR-14, CR1-R3).

Hiển thị: lịch gồm `scheduleId, fromAccountId, toAccountId, amount, note, recurrence, firstRunAt, endDate, maxRuns, status, nextRunAt, consecutiveFailures, createdAt, correlationId`; `nextRunAt` là `scheduled_at` nhỏ nhất của lần `PENDING`, hoặc null. Lần chạy gồm `runId, scheduleId, scheduledAt, status, attempts, transferId, reasonCode, correlationId, finishedAt`. Danh sách trả mảng JSON như `history()`; `limit` kẹp 1..100 (mặc định 20), `offset` mặc định 0; lần chạy sắp mới nhất trước. Chỉ chủ tài khoản nguồn (`from_account == caller`) xem/hủy/tạm dừng; người khác nhận `FORBIDDEN`. README/mô tả API và SMS nêu rõ số dư chỉ được kiểm tra lúc chạy (CR1-R3).

### 3.9 Thông báo — DI-SchedNotify (`minipay/notify.py::scheduled_run_sms`), MSG-INT02-scheduled-run, SCH-INT02-scheduled-run

- Sự kiện outbox `ScheduledRunCompleted` / `ScheduledRunFailed` ghi cùng giao dịch DB với việc cập nhật kết quả lần chạy. Payload (TDD: MSG-INT02-scheduled-run): `eventId` (= `runId`), `scheduleId`, `runId`, `fromAccountId`, `toAccountId`, `amount`, `correlationId`, cộng `transferId` (thành công) hoặc `reasonCode` (thất bại). Cùng topic `minipay.transfer.events.v1` (CH-INT02-events), khóa phân vùng `fromAccountId`, nên `relay_outbox` không cần sửa.
- Tệp mới `contracts/scheduled-run.event.schema.json` (SCH-INT02-scheduled-run, JSONSchema 1.0, tương thích ngược): `required` = `eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId`; `amount` integer ≥ 1; đúng một trong `transferId` hoặc `reasonCode` (`oneOf`); `additionalProperties:false`. Header/trường `correlationId` bắt buộc (HDR-INT02-corr).
- `consume()` phân nhánh theo sự có mặt của `runId`: dùng `scheduled_run_sms(p)` và gửi tới điện thoại của người gửi (`fromAccountId`); sự kiện `TransferCompleted` giữ hành vi cũ. Khử trùng theo `eventId` và vòng thử gửi `SMS_MAX_ATTEMPTS` giữ nguyên; lỗi SMS không đổi kết quả lần chạy (FR-047).
- Mẫu SMS (không dấu như mẫu hiện có): thành công "MiniPay: lich <scheduleId> da chuyen <amount> VND den TK <to>"; thất bại "MiniPay: lich <scheduleId> that bai (<reasonCode>). So du chi duoc kiem tra luc chay."

### 3.10 Audit — DI-SchedAudit (`minipay/service.py::audit_schedule_event`)

`audit_schedule_event(con, event_type, actor, cid, schedule_id, reason=None)` gọi `audit()` hiện có với `ref_id=schedule_id`. Loại sự kiện (Đề xuất solution tên cụ thể; TDD chỉ quy định phải có loại sự kiện lịch và tác nhân SCHEDULER): `SCHEDULE_CREATED`, `SCHEDULE_CANCELLED`, `SCHEDULE_PAUSED`, `SCHEDULE_RESUMED`, `SCHEDULE_AUTO_PAUSED`, `SCHEDULE_RUN_SUCCEEDED`, `SCHEDULE_RUN_FAILED`, `SCHEDULE_RUN_MISSED`. `actor` = mã tài khoản cho thao tác người dùng, `"SCHEDULER"` cho kết quả lần chạy và tự tạm dừng. `reason` = `reasonCode` nếu thất bại. Lần chạy truy ra qua `correlation_id` riêng của nó. Không đổi bảng `audit_logs`.

## 4. Luồng xử lý (SCN-ScheduledRun)

Ngân sách thời gian: 60.000 ms từ `scheduled_at` đến lúc bắt đầu chạy (NFR-010). Tham gia: MOD-Scheduler, MOD-API, MOD-Transfer; gọi INT-01.

1. Vòng lặp nền mỗi 30 giây gọi `scan_and_run` (AD-05).
2. `recover_stale_runs`; đánh dấu `MISSED` các lần `PENDING` quá hạn > 24 giờ (MECH-CatchUp); chọn tối đa 20 lần chạy đến hạn của lịch `ACTIVE` (MECH-SchedulerThrottle).
3. Với từng lần: chiếm bằng `UPDATE` có điều kiện (MECH-RunClaim); `rowcount=0` thì bỏ qua.
4. Sinh `correlationId`, dựng body `{fromAccountId, toAccountId, amount, note}` và khóa idempotency (§3.7), giãn 0,1 giây giữa các lần.
5. Gọi `execute_transfer` (idempotency → hạn mức ngày/số dư → Fraud Check INT-01 → ghi sổ + outbox → audit). Mọi kiểm tra tài chính diễn ra lúc này (BR-14).
6. Kết quả (một giao dịch DB): 
   - Thành công hoặc phát lại giao dịch COMPLETED → `SUCCEEDED` + `transfer_id`, `consecutive_failures=0`, audit `SCHEDULE_RUN_SUCCEEDED` (SCHEDULER), outbox `ScheduledRunCompleted`.
   - `MiniPayError` có thể thử lại và còn lượt → về `PENDING` với `next_attempt_at`.
   - Còn lại → `FAILED` + `reason_code`, `consecutive_failures+1`, audit `SCHEDULE_RUN_FAILED`, outbox `ScheduledRunFailed`; đạt 3 và lịch định kỳ thì `PAUSED` + audit `SCHEDULE_AUTO_PAUSED`.
7. Khi lần chạy kết thúc (SUCCEEDED/FAILED/MISSED) và lịch còn `ACTIVE`: nếu `schedule_exhausted` hoặc lịch `ONCE` → lịch `COMPLETED`; ngược lại chèn lần chạy kế tiếp `PENDING` tại `occurrence(next_index)` và tăng `next_index`.
8. Luồng bất đồng bộ: `relay_outbox` → queue → `consume` → SMS tới người gửi (SCN-Notify mở rộng).

Luồng người dùng:
- Tạo lịch: kiểm tra (§3.8) → chèn `scheduled_transfers` + lần chạy #0 `PENDING` tại `firstRunAt` + audit `SCHEDULE_CREATED`, trong một `tx`.
- Hủy: lịch `CANCELLED`, mọi lần `PENDING` → `CANCELLED`; có lần `RUNNING` hoặc lịch `COMPLETED` → `SCHEDULE_NOT_CANCELLABLE`; đã `CANCELLED` → trả kết quả cũ (200).
- Tạm dừng (lịch `ACTIVE`, định kỳ): `PAUSED`; lần chạy `PENDING` không được chọn khi lịch không `ACTIVE`.
- Tiếp tục (lịch `PAUSED`): chèn lần `SKIPPED` cho mỗi `occurrence` đã qua kể từ `next_index` (không chạy bù), đánh dấu `SKIPPED` lần `PENDING` đã quá hạn, chèn lần kế tiếp ≥ now `PENDING`, đặt `consecutive_failures=0`, lịch `ACTIVE`; nếu hết lần chạy thì lịch `COMPLETED`.

## 5. Xử lý lỗi và an toàn

### 5.1 Phân loại lỗi lần chạy (BR-13, MECH-ScheduleRetry, FB-INT01)

| Mã lỗi từ lõi | Loại | Hành động |
| --- | --- | --- |
| `FRAUD_CHECK_UNAVAILABLE` | tạm thời | thử lại tối đa 3 lần, cách 300 s |
| `INSUFFICIENT_FUNDS` | thiếu số dư | thử lại tối đa 2 lần, cách 3600 s (OQ-010) |
| `ACCOUNT_INACTIVE`, `FRAUD_REJECTED`, `LIMIT_EXCEEDED` | vĩnh viễn | `FAILED` ngay |
| `SAME_ACCOUNT`, `ACCOUNT_NOT_FOUND`, `IDEMPOTENCY_CONFLICT`, mã khác | vĩnh viễn | `FAILED` ngay (Đề xuất solution: mặc định vĩnh viễn) |

"Thử lại tối đa N lần" hiểu là N lần chạy lại sau lần đầu, tức tối đa 1+N lần chiếm (`attempts ≤ 1+N`) (Q-S3). Fraud Check lỗi với số tiền < 5.000.000 VND vẫn thành công với `riskCheck=SKIPPED` (FB-INT01, giữ nguyên từ lõi).

### 5.2 An toàn và nhất quán

- Không chạy trùng: MECH-RunClaim + `UNIQUE(schedule_id, scheduled_at)` + khóa idempotency cố định theo `attempts` + `UNIQUE(from_account, idempotency_key)` của `transfers`. Ba lớp này độc lập nhau (NFR-011, RISK-005).
- Khởi động lại giữa chiếm và chuyển tiền: lần chạy kẹt `RUNNING` bị chiếm lại sau lease; chạy lại với cùng `attempts` nên cùng khóa, `execute_transfer` phát lại giao dịch đã có, không tạo mới (TC-S11).
- Tải: `LIMIT 20` + 10 rps giữ tải Fraud Check của scheduler dưới 50 rps của đối tác (LIM-INT01); ASM-001 vẫn Open nên cần xác nhận lại. Trường hợp xấu nhất 20 lần × 2 lời gọi × 1 s = 40 s trong một chu kỳ có thể đẩy lần chạy cuối quá 60 giây; circuit breaker (mở sau 5 lỗi, 30 s) giới hạn thực tế (xem §7).
- SQLite một luồng ghi (RISK-004, CR1-R4): mỗi ghi dùng `tx()` ngắn; Fraud Check gọi ngoài giao dịch DB như luồng cũ (NFR-006).
- Phân quyền: chủ tài khoản nguồn mới được tạo/xem/hủy/tạm dừng; scheduler gọi nội bộ, không dùng API key.
- Dữ liệu nhạy cảm: `fromAccountId`, `toAccountId` trong sự kiện (sensitiveFields của MSG-INT02-scheduled-run) không ghi vào log ứng dụng.
- Lỗi gửi SMS hoặc broker không đổi kết quả lần chạy; sự kiện nằm trong outbox và được `relay_outbox` gửi lại (ERR-INT02-broker).
- Không sửa nút KG `Baselined` cũ (R17); chỉ thêm nút mới.

## 6. Kế hoạch test (TestCase của patch)

Tất cả trong `tests/test_scheduler.py` (tệp mới) trừ TC-S13. Dùng mẫu của `tests/test_minipay.py`: `FakeFraud` (cần thêm chế độ trả lỗi theo kịch bản), fixture `env`, `TestClient(create_app(db, fraud))`. Đồng hồ giả: truyền `clock` vào `scan_and_run`, và đặt `firstRunAt` bằng cách ghi trực tiếp `scheduled_at` vào DB khi cần ép lần chạy đến hạn.

| TC | Test (vị trí) | Yêu cầu | Kịch bản chính |
| --- | --- | --- | --- |
| TC-S01 | `test_create_once_and_validation` | FR-040, FR-041 | Tạo ONCE: 201, `ACTIVE`, số dư không đổi. Lỗi: `firstRunAt` < 5 phút / > 90 ngày → `SCHEDULE_TIME_INVALID`; đích không có → `ACCOUNT_NOT_FOUND`; người khác → `FORBIDDEN`; định kỳ thiếu `endDate` và `maxRuns` → `INVALID_REQUEST`; lịch thứ 21 → `SCHEDULE_LIMIT_REACHED`; cùng Idempotency-Key phát lại 200 / khác nội dung 409. MONTHLY ngày 29/30/31 và năm nhuận (BR-12). |
| TC-S02 | `test_query_schedules_and_runs` | FR-042 | Lấy 1 lịch, danh sách theo `status`, phân trang `limit/offset`, các lần chạy mới nhất trước; người khác `FORBIDDEN`; mã lạ `SCHEDULE_NOT_FOUND`. |
| TC-S03 | `test_cancel_and_pause_resume` | FR-043, FR-044 | Hủy: lịch và lần PENDING → CANCELLED; hủy lại trả kết quả cũ; lần RUNNING → `SCHEDULE_NOT_CANCELLABLE`. Tạm dừng/tiếp tục; trạng thái sai và lịch ONCE → `SCHEDULE_STATE_INVALID`; lần bỏ lỡ → SKIPPED. |
| TC-S04 | `test_due_run_creates_one_transfer` | FR-045 | Lần đến hạn: đúng 1 giao dịch COMPLETED, run SUCCEEDED có `transferId`, khóa = lịch + thời điểm, hạn mức/số dư/Fraud Check kiểm lúc chạy, sổ cái cân. Lịch định kỳ sinh lần kế tiếp. |
| TC-S05 | `test_retry_policy_and_auto_pause` | FR-046 | Fraud down và ≥ 5.000.000: thử lại 3 lần cách 300 s rồi FAILED; thiếu số dư: 2 lần cách 3600 s; vĩnh viễn: FAILED ngay; 3 FAILED liên tiếp → lịch định kỳ PAUSED; SUCCEEDED xóa bộ đếm. |
| TC-S06 | `test_run_result_events_and_sms` | FR-047 | Outbox có `ScheduledRunCompleted`/`ScheduledRunFailed` hợp lệ schema; `/admin/outbox/run` ghi SMS cho người gửi; SMS lỗi không đổi kết quả run; sự kiện khử trùng theo `eventId`. |
| TC-S07 | `test_schedule_audit_actor_scheduler` | FR-048 | Audit cho tạo/hủy/tạm dừng/tiếp tục/kết quả; tác nhân SCHEDULER; có correlationId, `ref_id` = mã lịch. |
| TC-S08 | `test_catch_up_and_missed` | FR-049, NFR-012 | Lần quá hạn 23 giờ chạy đúng 1 lần; quá 24 giờ → MISSED + sự kiện lỗi; 100 lần quá hạn xử lý hết trong 5 chu kỳ. |
| TC-S09 | `test_scan_punctuality_fake_clock` | NFR-010 | Đồng hồ giả, quét mỗi 30 giây: ≥ 99% lần chạy có `started_at − scheduled_at ≤ 60 s`. |
| TC-S10 | `test_two_schedulers_no_duplicate` | NFR-011 | Hai luồng (hai kết nối) cùng gọi `scan_and_run` trên cùng lần chạy: 1 giao dịch, 1 SUCCEEDED, Fraud Check chỉ gọi 1 lần. |
| TC-S11 | `test_restart_between_claim_and_transfer` | NFR-011 | Ép RUNNING mà chưa có giao dịch, và RUNNING đã có giao dịch; sau lease quét lại: không tạo giao dịch thứ hai, run SUCCEEDED. |
| TC-S12 | `test_scheduler_correlation_id` | NFR-014 | Mỗi run một correlationId khác nhau; cùng giá trị trong `transfers`, `audit_logs`, tham số `fraud.check` và payload sự kiện. |
| TC-S13 | `benchmark_scheduler_100_runs` (`scripts/benchmark_scheduler.py::main`) | NFR-013 | Theo mẫu `scripts/benchmark.py` (`SlowFraud`, `ThreadPoolExecutor`): 100 lần chạy đến hạn trong khi gọi API chuyển tiền; khẳng định p95 API ≤ 500 ms; in kết quả (có thể ghi JSON riêng, không ghi đè `benchmark_result.json`). |

Hồi quy: 36 test hiện có phải còn đạt (tiêu chí đóng CR); thêm kiểm tra `transfer()` sau tách lõi giữ nguyên mã lỗi và HTTP.

## 7. Câu hỏi mở và rủi ro còn lại

Đã chốt trong patch (Confirmed): OQ-010, OQ-011, OQ-012, ASM-010, ASM-011. Còn lại:

| Mã | Nội dung | Mặc định solution |
| --- | --- | --- |
| Q-S1 | FR-045 ghi khóa = mã lịch + thời điểm dự kiến, nhưng REJECTED được lưu và phát lại theo khóa, nên thử lại cần khóa khác | Lần thử n≥2 thêm hậu tố `#n` (tiền tố giữ nguyên); cần Tech Lead xác nhận hoặc sửa FR-045 |
| Q-S2 | Hạn lease cho lần chạy kẹt RUNNING không có trong TDD | `SCHED_RUN_LEASE_S=120` |
| Q-S3 | "Thử lại tối đa 3 lần/2 lần" tính thêm sau lần đầu hay gồm lần đầu | Thêm sau lần đầu (tối đa 4 và 3 lần chiếm) |
| Q-S4 | `endDate` > 366 ngày dùng mã nào; `maxRuns` ngoài 1..60 | `SCHEDULE_TIME_INVALID` và `INVALID_REQUEST` |
| Q-S5 | Cách bật scheduler trong tiến trình | `create_app(..., run_scheduler=False)`, `_default()` bật |
| Q-S6 | BR-16 hiển thị mã lịch trong lịch sử, không có cột trong `transfers` | Thêm `scheduleId` qua LEFT JOIN `scheduled_runs` |
| Q-S7 | `recurrence` bắt buộc hay mặc định ONCE | Bắt buộc theo SRS bổ sung |

Rủi ro còn lại: RISK-005 (chạy trùng và tải Fraud Check; ASM-001 chưa xác nhận 50 rps, nên rate limit 10 rps là tạm); RISK-004 (tranh chấp ghi SQLite khi scheduler chạy cùng API); CR1-R3 (người dùng hiểu nhầm lịch bảo đảm có tiền, giảm nhẹ bằng nội dung API/SMS); độ chính xác thời gian phụ thuộc chu kỳ 30 giây cộng thời gian xử lý, trường hợp 20 lần chạy chậm có thể vượt 60 giây ở lần cuối (CR1-R2); CR1-R6 (phạm vi trượt).

Tóm tắt: thiết kế thêm `minipay/scheduler.py` (vòng lặp quét 30 giây, chiếm lần chạy bằng UPDATE có điều kiện), hai bảng, sáu endpoint, tách `execute_transfer` dùng chung, sự kiện/SMS/audit cho lần chạy, và 13 test theo TC-S01..S13. Điểm cần xác nhận nhất là Q-S1: khóa idempotency khi thử lại.

## 8. Phụ lục: đặc tả có cấu trúc cho test specs

### 8.1 Endpoint

| Mã | Method | Path | implements | Xác thực | idempotent |
| --- | --- | --- | --- | --- | --- |
| EP-CreateSchedule | POST | /v1/scheduled-transfers | FR-040, FR-041, FR-048 | Bearer API key | có (Idempotency-Key) |
| EP-GetSchedule | GET | /v1/scheduled-transfers/{scheduleId} | FR-042 | Bearer API key | có |
| EP-ListSchedules | GET | /v1/accounts/{accountId}/scheduled-transfers | FR-042 | Bearer API key | có |
| EP-ListRuns | GET | /v1/scheduled-transfers/{scheduleId}/runs | FR-042 | Bearer API key | có |
| EP-CancelSchedule | DELETE | /v1/scheduled-transfers/{scheduleId} | FR-043, FR-048 | Bearer API key | có |
| EP-UpdateScheduleStatus | PATCH | /v1/scheduled-transfers/{scheduleId} | FR-044, FR-048 | Bearer API key | không |

### 8.2 Trường của yêu cầu

| Mã | Của endpoint | name | Kiểu | required | min | max | minLength | maxLength | enum |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| FLD-Create-Authorization | EP-CreateSchedule | Authorization (header) | string | có | | | | | |
| FLD-Create-IdempotencyKey | EP-CreateSchedule | Idempotency-Key (header) | string | có | | | | | |
| FLD-Create-fromAccountId | EP-CreateSchedule | fromAccountId (body) | string | có | | | | | |
| FLD-Create-toAccountId | EP-CreateSchedule | toAccountId (body) | string | có | | | | | |
| FLD-Create-amount | EP-CreateSchedule | amount (body) | integer | có | 1 | 50000000 | | | |
| FLD-Create-note | EP-CreateSchedule | note (body) | string | không | | | | | |
| FLD-Create-recurrence | EP-CreateSchedule | recurrence (body) | string | có | | | | | ONCE, DAILY, WEEKLY, MONTHLY |
| FLD-Create-firstRunAt | EP-CreateSchedule | firstRunAt (body) | string (date-time) | có | | | | | |
| FLD-Create-endDate | EP-CreateSchedule | endDate (body) | string (date) | có điều kiện: định kỳ cần endDate hoặc maxRuns | | | | | |
| FLD-Create-maxRuns | EP-CreateSchedule | maxRuns (body) | integer | có điều kiện: định kỳ cần endDate hoặc maxRuns | 1 | 60 | | | |
| FLD-Get-Authorization | EP-GetSchedule | Authorization (header) | string | có | | | | | |
| FLD-Get-scheduleId | EP-GetSchedule | scheduleId (path) | string | có | | | | | |
| FLD-ListSchedules-Authorization | EP-ListSchedules | Authorization (header) | string | có | | | | | |
| FLD-ListSchedules-accountId | EP-ListSchedules | accountId (path) | string | có | | | | | |
| FLD-ListSchedules-status | EP-ListSchedules | status (query) | string | không | | | | | ACTIVE, PAUSED, COMPLETED, CANCELLED |
| FLD-ListSchedules-limit | EP-ListSchedules | limit (query) | integer | không | 1 | 100 | | | |
| FLD-ListSchedules-offset | EP-ListSchedules | offset (query) | integer | không | 0 | | | | |
| FLD-ListRuns-Authorization | EP-ListRuns | Authorization (header) | string | có | | | | | |
| FLD-ListRuns-scheduleId | EP-ListRuns | scheduleId (path) | string | có | | | | | |
| FLD-ListRuns-limit | EP-ListRuns | limit (query) | integer | không | 1 | 100 | | | |
| FLD-ListRuns-offset | EP-ListRuns | offset (query) | integer | không | 0 | | | | |
| FLD-Cancel-Authorization | EP-CancelSchedule | Authorization (header) | string | có | | | | | |
| FLD-Cancel-scheduleId | EP-CancelSchedule | scheduleId (path) | string | có | | | | | |
| FLD-Patch-Authorization | EP-UpdateScheduleStatus | Authorization (header) | string | có | | | | | |
| FLD-Patch-scheduleId | EP-UpdateScheduleStatus | scheduleId (path) | string | có | | | | | |
| FLD-Patch-status | EP-UpdateScheduleStatus | status (body) | string | có | | | | | PAUSED, ACTIVE |

Ghi chú: `limit` ngoài khoảng được kẹp vào 1..100 (như `history()`), không trả lỗi.

### 8.3 Phản hồi

| Mã | Phản hồi của | statusCode | errorCode | condition |
| --- | --- | --- | --- | --- |
| RSP-Create-201 | EP-CreateSchedule | 201 | | Tạo lịch thành công, `status=ACTIVE`, trả mã lịch |
| RSP-Create-200 | EP-CreateSchedule | 200 | | Cùng Idempotency-Key và cùng nội dung: trả lịch đã tạo |
| RSP-Create-400 | EP-CreateSchedule | 400 | INVALID_REQUEST | Thiếu Idempotency-Key hoặc trường bắt buộc, amount không nguyên dương, recurrence sai, định kỳ thiếu cả endDate và maxRuns, maxRuns ngoài 1..60, firstRunAt sai định dạng |
| RSP-Create-401 | EP-CreateSchedule | 401 | UNAUTHORIZED | Thiếu hoặc sai API key |
| RSP-Create-403 | EP-CreateSchedule | 403 | FORBIDDEN | fromAccountId khác tài khoản của người gọi |
| RSP-Create-404 | EP-CreateSchedule | 404 | ACCOUNT_NOT_FOUND | Tài khoản đích không tồn tại |
| RSP-Create-409 | EP-CreateSchedule | 409 | IDEMPOTENCY_CONFLICT | Cùng Idempotency-Key nhưng nội dung khác |
| RSP-Create-422-time | EP-CreateSchedule | 422 | SCHEDULE_TIME_INVALID | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày; endDate trước lần chạy đầu hoặc quá 366 ngày |
| RSP-Create-422-limit | EP-CreateSchedule | 422 | SCHEDULE_LIMIT_REACHED | Tài khoản đã có 20 lịch ACTIVE hoặc PAUSED |
| RSP-Create-422-amount | EP-CreateSchedule | 422 | LIMIT_EXCEEDED | amount > 50.000.000 VND |
| RSP-Create-422-same | EP-CreateSchedule | 422 | SAME_ACCOUNT | fromAccountId bằng toAccountId |
| RSP-Get-200 | EP-GetSchedule | 200 | | Trả trạng thái lịch và lần chạy kế tiếp (`nextRunAt`) |
| RSP-Get-401 | EP-GetSchedule | 401 | UNAUTHORIZED | Thiếu hoặc sai API key |
| RSP-Get-403 | EP-GetSchedule | 403 | FORBIDDEN | Người gọi không phải chủ tài khoản nguồn của lịch |
| RSP-Get-404 | EP-GetSchedule | 404 | SCHEDULE_NOT_FOUND | Không có lịch với scheduleId |
| RSP-ListSchedules-200 | EP-ListSchedules | 200 | | Trả danh sách lịch của tài khoản, phân trang, lọc theo status |
| RSP-ListSchedules-400 | EP-ListSchedules | 400 | INVALID_REQUEST | status ngoài enum |
| RSP-ListSchedules-401 | EP-ListSchedules | 401 | UNAUTHORIZED | Thiếu hoặc sai API key |
| RSP-ListSchedules-403 | EP-ListSchedules | 403 | FORBIDDEN | accountId khác tài khoản của người gọi |
| RSP-ListRuns-200 | EP-ListRuns | 200 | | Trả các lần chạy, mới nhất trước, mỗi lần có thời điểm dự kiến, trạng thái, transferId, reasonCode |
| RSP-ListRuns-401 | EP-ListRuns | 401 | UNAUTHORIZED | Thiếu hoặc sai API key |
| RSP-ListRuns-403 | EP-ListRuns | 403 | FORBIDDEN | Người gọi không phải chủ lịch |
| RSP-ListRuns-404 | EP-ListRuns | 404 | SCHEDULE_NOT_FOUND | Không có lịch với scheduleId |
| RSP-Cancel-200 | EP-CancelSchedule | 200 | | Hủy thành công (lịch CANCELLED, lần PENDING thành CANCELLED) hoặc lịch đã CANCELLED: trả kết quả cũ |
| RSP-Cancel-401 | EP-CancelSchedule | 401 | UNAUTHORIZED | Thiếu hoặc sai API key |
| RSP-Cancel-403 | EP-CancelSchedule | 403 | FORBIDDEN | Người gọi không phải chủ lịch |
| RSP-Cancel-404 | EP-CancelSchedule | 404 | SCHEDULE_NOT_FOUND | Không có lịch với scheduleId |
| RSP-Cancel-409 | EP-CancelSchedule | 409 | SCHEDULE_NOT_CANCELLABLE | Lịch đã COMPLETED hoặc có lần chạy đang RUNNING |
| RSP-Patch-200 | EP-UpdateScheduleStatus | 200 | | Chuyển PAUSED hoặc ACTIVE thành công, trả lịch |
| RSP-Patch-400 | EP-UpdateScheduleStatus | 400 | INVALID_REQUEST | status thiếu hoặc ngoài PAUSED/ACTIVE |
| RSP-Patch-401 | EP-UpdateScheduleStatus | 401 | UNAUTHORIZED | Thiếu hoặc sai API key |
| RSP-Patch-403 | EP-UpdateScheduleStatus | 403 | FORBIDDEN | Người gọi không phải chủ lịch |
| RSP-Patch-404 | EP-UpdateScheduleStatus | 404 | SCHEDULE_NOT_FOUND | Không có lịch với scheduleId |
| RSP-Patch-409 | EP-UpdateScheduleStatus | 409 | SCHEDULE_STATE_INVALID | Tạm dừng lịch không ACTIVE, tiếp tục lịch không PAUSED, hoặc lịch ONCE |

### 8.4 Tiêu chí chấp nhận

| Mã | Yêu cầu | Tiêu chí |
| --- | --- | --- |
| AC-FR-040-1 | FR-040 | POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE |
| AC-FR-040-2 | FR-040 | Số dư hai tài khoản không đổi và không có bản ghi `ledger_entries` mới sau khi tạo lịch |
| AC-FR-040-3 | FR-040 | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID |
| AC-FR-040-4 | FR-040 | toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND |
| AC-FR-040-5 | FR-040 | fromAccountId khác chủ API key thì trả 403 FORBIDDEN |
| AC-FR-040-6 | FR-040 | Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED |
| AC-FR-041-1 | FR-041 | Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) |
| AC-FR-041-2 | FR-041 | Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST |
| AC-FR-041-3 | FR-041 | Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED |
| AC-FR-041-4 | FR-041 | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận |
| AC-FR-042-1 | FR-042 | GET một lịch trả status và nextRunAt |
| AC-FR-042-2 | FR-042 | GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset |
| AC-FR-042-3 | FR-042 | GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước |
| AC-FR-042-4 | FR-042 | Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND |
| AC-FR-043-1 | FR-043 | DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED |
| AC-FR-043-2 | FR-043 | DELETE lịch đã CANCELLED trả 200 với kết quả cũ |
| AC-FR-043-3 | FR-043 | DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE |
| AC-FR-044-1 | FR-044 | PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này |
| AC-FR-044-2 | FR-044 | PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù |
| AC-FR-044-3 | FR-044 | PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID |
| AC-FR-045-1 | FR-045 | Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId |
| AC-FR-045-2 | FR-045 | Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến |
| AC-FR-045-3 | FR-045 | Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch |
| AC-FR-045-4 | FR-045 | Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch |
| AC-FR-046-1 | FR-046 | FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode |
| AC-FR-046-2 | FR-046 | INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED |
| AC-FR-046-3 | FR-046 | ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại |
| AC-FR-046-4 | FR-046 | Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED |
| AC-FR-047-1 | FR-047 | Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox |
| AC-FR-047-2 | FR-047 | Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox |
| AC-FR-047-3 | FR-047 | SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy |
| AC-FR-048-1 | FR-048 | Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch |
| AC-FR-048-2 | FR-048 | Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch |
| AC-FR-049-1 | FR-049 | Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần |
| AC-FR-049-2 | FR-049 | Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch |
| AC-NFR-010-1 | NFR-010 | Với đồng hồ giả và chu kỳ quét 30 giây, ít nhất 99% lần chạy bắt đầu trong 60 giây kể từ thời điểm dự kiến |
| AC-NFR-011-1 | NFR-011 | Hai scheduler cùng quét một lần chạy chỉ tạo một giao dịch |
| AC-NFR-011-2 | NFR-011 | Khởi động lại giữa lúc chiếm lần chạy và lúc chuyển tiền không tạo giao dịch thứ hai |
| AC-NFR-012-1 | NFR-012 | Sau khởi động lại, các lần chạy quá hạn trong 24 giờ được xử lý trong 5 chu kỳ quét đầu |
| AC-NFR-013-1 | NFR-013 | Khi 100 lần chạy đến hạn cùng lúc, p95 thời gian chuyển tiền qua API không quá 500 ms |
| AC-NFR-013-2 | NFR-013 | Một chu kỳ quét xử lý tối đa 20 lần chạy |
| AC-NFR-014-1 | NFR-014 | Mỗi lần chạy có correlationId riêng, khác nhau giữa các lần chạy |
| AC-NFR-014-2 | NFR-014 | correlationId của lần chạy xuất hiện trong giao dịch, audit, lời gọi Fraud Check và sự kiện |

### 8.5 Máy trạng thái

| Mã | Tên | governs | initial |
| --- | --- | --- | --- |
| SM-Schedule | Vòng đời lịch chuyển tiền | DE-ScheduledTransfer | ACTIVE |
| SM-ScheduledRun | Vòng đời lần chạy | DE-ScheduledRun | PENDING |

| Mã | Máy trạng thái | from | to | trigger | guard | effect |
| --- | --- | --- | --- | --- | --- | --- |
| TRN-Schedule-1 | SM-Schedule | ACTIVE | PAUSED | PATCH status=PAUSED | lịch định kỳ | Audit SCHEDULE_PAUSED; scheduler không chạy lịch |
| TRN-Schedule-2 | SM-Schedule | PAUSED | ACTIVE | PATCH status=ACTIVE | còn lần chạy theo BR-11 | Lần bỏ lỡ thành SKIPPED; sinh lần chạy kế tiếp; consecutive_failures về 0; audit SCHEDULE_RESUMED |
| TRN-Schedule-3 | SM-Schedule | ACTIVE | PAUSED | scheduler: lần chạy FAILED thứ 3 liên tiếp | lịch định kỳ | Audit SCHEDULE_AUTO_PAUSED (tác nhân SCHEDULER); không sinh lần chạy mới |
| TRN-Schedule-4 | SM-Schedule | ACTIVE | COMPLETED | lần chạy cuối kết thúc | lịch ONCE đã xong, hoặc đạt maxRuns hoặc endDate | Không sinh lần chạy mới |
| TRN-Schedule-5 | SM-Schedule | ACTIVE | CANCELLED | DELETE | không có lần chạy RUNNING | Lần PENDING thành CANCELLED; audit SCHEDULE_CANCELLED |
| TRN-Schedule-6 | SM-Schedule | PAUSED | CANCELLED | DELETE | không có lần chạy RUNNING | Lần PENDING thành CANCELLED; audit SCHEDULE_CANCELLED |
| TRN-Schedule-7 | SM-Schedule | PAUSED | COMPLETED | PATCH status=ACTIVE | không còn lần chạy nào theo BR-11 sau khi đánh dấu SKIPPED | Các lần còn lại SKIPPED; không sinh lần mới |
| TRN-ScheduledRun-1 | SM-ScheduledRun | PENDING | RUNNING | scheduler chiếm lần chạy | đến hạn (scheduled_at hoặc next_attempt_at ≤ now), lịch ACTIVE, UPDATE có điều kiện rowcount=1 | attempts+1; sinh correlationId; gọi lõi chuyển tiền |
| TRN-ScheduledRun-2 | SM-ScheduledRun | RUNNING | SUCCEEDED | lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) | | Lưu transferId; consecutive_failures về 0; audit; phát ScheduledRunCompleted; sinh lần kế tiếp hoặc COMPLETED lịch |
| TRN-ScheduledRun-3 | SM-ScheduledRun | RUNNING | PENDING | lõi trả lỗi tạm thời hoặc thiếu số dư | còn lượt thử lại (FRAUD_CHECK_UNAVAILABLE ≤ 3 lần, INSUFFICIENT_FUNDS ≤ 2 lần) | Đặt next_attempt_at = now + 5 phút hoặc 1 giờ |
| TRN-ScheduledRun-4 | SM-ScheduledRun | RUNNING | FAILED | lõi trả lỗi | lỗi vĩnh viễn hoặc hết lượt thử lại | Lưu reasonCode; consecutive_failures+1; audit; phát ScheduledRunFailed |
| TRN-ScheduledRun-5 | SM-ScheduledRun | PENDING | SKIPPED | tiếp tục lịch PAUSED | lần chạy đã quá hạn trong lúc tạm dừng | Không chạy bù |
| TRN-ScheduledRun-6 | SM-ScheduledRun | PENDING | MISSED | đầu chu kỳ quét | quá hạn hơn 24 giờ so với scheduled_at | Phát ScheduledRunFailed với reasonCode MISSED; audit; không tính FAILED liên tiếp |
| TRN-ScheduledRun-7 | SM-ScheduledRun | PENDING | CANCELLED | DELETE lịch | | |
| TRN-ScheduledRun-8 | SM-ScheduledRun | RUNNING | PENDING | scheduler phục hồi lần chạy kẹt | started_at cũ hơn lease (120 giây, đề xuất Q-S2) | Giữ attempts để dùng lại cùng khóa idempotency |
