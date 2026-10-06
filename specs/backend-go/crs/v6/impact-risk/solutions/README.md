# impact-risk: solutions backend (BE-REQ-SOL-030)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Tài liệu ngày 2026-10-06; đã đối chiếu với `backend-go/Makefile`, `proto/buf.yaml`, `backend-go/ci/`, `backend-go/policy/orca-authz/`, GitNexus CLI (`--help`) và `.gitnexus/gitnexus.json` cùng ngày. `request-service` chưa có thư mục: mọi đường dẫn của nó là "(mới)".

Nguồn: [docs/crs/v6/impact-risk](../../../../../../docs/crs/v6/impact-risk/README.md). README v6 [mục 8](../../../../../../docs/crs/v6/README.md) thắng mục 3 khi mâu thuẫn. Tài liệu thiết kế tham chiếu: [`tdd/README.md`](../../../../tdd/README.md), `architecture/03, 05, 07, 08, 09`, [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md), [`services/task-service.md`](../../../../tdd/services/task-service.md).

## Bảng CR → Solution → Task

| CR | Solution | Service | Task (xem [tasks/README](../tasks/README.md)) |
|---|---|---|---|
| [CR-REQ-030](../../../../../../docs/crs/v6/impact-risk/CR-REQ-030-impact-assessment-and-risk-scoring.md) `ImpactAssessment`, `RiskAcceptance`, `RiskPolicy`, bộ thu thập, điểm bằng quy tắc, `GraphPayload`, chạy bóng | [BE-REQ-SOL-030](./BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md) | `request-service` (mới), `proto/orca/request/v1` | TASK-REQ-030-01 đến 08 |

Không đổi `task-service`, `agent`, `infra-fleet-service`.

## Thứ tự phụ thuộc

```
SOL-029 (AgentRelay, base_sha, execution.verified)   SOL-033 (hồ sơ năng lực)   SOL-007/012/013 (sự kiện, lease)
        │                                                  │                          │
 030-01 migration ─▶ 030-03 repository ─┐                  │                          │
 030-02 domain rp/1 ───────────────────┼─▶ 030-05 collector + consumer + AssessActual ◀┘
 030-04 scanners + parser ─────────────┘            │
                                                    ├─▶ 030-06 RiskGate, AcceptRisk, Override, drift ◀── SOL-009 (cần ApprovalGuard)
                                                    └─▶ 030-07 RPC đọc, đồ thị, narrator
                                                                 │
                                                030-06 + 030-07 ─▶ 030-08 policy admin, shadow/enforce, hiệu chỉnh, e2e
```

Giai đoạn 0 (thử ngoại tuyến) của CR chạy được ngay sau 030-02 và 030-04, **trước** khi viết 030-05 (công cụ `cmd/impact-calibrate` ở 030-08, có thể tách sớm). CR giao theo ba giai đoạn: 0 thử ngoại tuyến, 1 `shadow` (mặc định), 2 `enforce` do admin tenant bật.

## Quyết định chung

| # | Quyết định | Lý do |
|---|---|---|
| R1 | Điểm do quy tắc có phiên bản (`rp/1`), hàm thuần; `narrative` AI ngoài `digest` | Tái lập, kiểm toán |
| R2 | Ba thời điểm: Option, Plan/Phase/Task, thực tế; nhưng `plan|phase|task` chỉ đo được tín hiệu `basis=path` và độ phủ (chưa có diff) | Không giả vờ độ tin cậy cao; công cụ thật chạy ở `actual_*` |
| R3 | Mỗi chiều có `basis` (`tool|path|declared`); `confidence` tối đa `medium` nếu có chiều không phải `tool` | Phản ánh nguồn dữ liệu |
| R4 | Chiều không đo được tính 50 và `risk=unknown` ở đồ thị | "Không đo được" khác "thấp" |
| R5 | `buf breaking` chạy trực tiếp, không `make proto-lint` | `A && B \|\| true` nuốt mọi lỗi |
| R6 | Kích hoạt bằng consumer bền trên sự kiện có sẵn (`solution.proposed`, `plan.generated`, `execution.verified`, `phase.completed`) | Tách khỏi đường chặn; at-least-once + `processed_events` |
| R7 | `RiskAcceptance` gắn `assessment_digest`; digest chủ thể Approval gồm `assessment.digest` | Đánh giá đổi thì phải duyệt lại |
| R8 | Drift dùng Approval `phase` với `stage=drift_review` | Không thêm `subject_type` |
| R9 | `shadow` mặc định; `enforce` chỉ qua `SetRiskPolicy` khi `EnforceReadiness` đạt | Ngưỡng là ước lượng |
| R10 | Đồ thị trả `GraphPayload` của CR-REQ-032; tên kênh WS do CR-REQ-016/036 chốt | Một hợp đồng cho mọi lens |

## Đã kiểm chứng và điểm khác CR gốc

- `Makefile` dòng 80 đến 81: `buf lint && buf breaking --against '.git#branch=main' || true` luôn thành công; CI không có `buf breaking` (đúng CR). Việc sửa Makefile và thêm job CI là đề nghị riêng ngoài series.
- GitNexus CLI: `impact` có `--summary-only`, `--depth`, `--limit`; `detect-changes` có `--scope compare --base-ref`; **không có `--json`**. Parser phải dựa mẫu thật (task 04, bước 1 bắt buộc).
- Ở `plan|phase|task` chưa có diff nên `buf breaking` và quét nội dung migration không chạy được (khác bảng 2.2 của CR); chỉ tín hiệu theo đường dẫn và `go test -cover` (SOL-030 mục 1, điều 1).
- Trigger "sau COMMIT Plan" của CR là sự kiện `orca.request.plan.generated` của SOL-012.
- **SOL-009 chưa có `ApprovalGuard`, `viewed_impact_digest`, `accepted_finding_ids`, `required_approvals`**; task 06 định nghĩa giao diện cần thiết và coi đây là phụ thuộc cứng (không sửa SOL-009). Hai người duyệt cho mức Nghiêm trọng chưa chốt: cờ `REQUEST_RISK_REQUIRE_TWO_APPROVERS=false` giữ chỗ.
- Nhãn `gate:feature_flag` và `check:rollback_rehearsal` chưa có trong `plan_labels.go` (SOL-012); `PlanRiskPreconditions` chỉ kiểm khi nhãn tồn tại.
- Hàm canonical JSON và digest dùng của TASK-REQ-027-03 (không tạo bản mới).
- Chưa chạy bất kỳ test nào; mọi lệnh test là lệnh dự kiến.
