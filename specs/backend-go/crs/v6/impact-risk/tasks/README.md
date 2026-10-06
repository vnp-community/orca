# impact-risk: tasks backend (TASK-REQ-030-01 đến 08)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Mỗi task làm được trong 0,5 đến 2 ngày (task 05, 06 gần 2 ngày).

Solution: [BE-REQ-SOL-030](../solutions/BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md). Danh sách solution: [solutions/README](../solutions/README.md).

## Bảng Solution → Task

| Task | Tên | Priority | Service | Phụ thuộc |
|---|---|---|---|---|
| [TASK-REQ-030-01](./TASK-REQ-030-01-impact-risk-migration.md) | Migration `impact_risk` hai dialect (5 bảng) | P1 | `request-service` (mới) | TASK-REQ-002-01, 001-04 |
| [TASK-REQ-030-02](./TASK-REQ-030-02-risk-scoring-domain-rp1.md) | Domain `rp/1`: chín chiều, `Score`, luật cứng, digest, so lệch | P1 | `request-service` (mới) | TASK-REQ-001-01, 027-03 (`CanonicalJSON`) |
| [TASK-REQ-030-03](./TASK-REQ-030-03-impact-risk-repositories.md) | Repository hai dialect (lease, một `collecting`, hiệu lực theo digest) | P1 | `request-service` (mới) | 01, 02, TASK-REQ-001-04 |
| [TASK-REQ-030-04](./TASK-REQ-030-04-scanners-and-gitnexus-parser.md) | `migrationscan`, `contractscan`, `gitnexusparse`, `AreaResolver`, fixture từ mẫu thật | P1 | `request-service` (mới) | 02 |
| [TASK-REQ-030-05](./TASK-REQ-030-05-impact-collector-triggers-and-actual.md) | `ImpactCollector`, worker lease, consumer kích hoạt, `AssessActual`, `RecordRiskOutcome` | P1 | `request-service` (mới) | 02, 03, 04, TASK-REQ-029-06, 029-08, 033-04, SOL-007/012/013 |
| [TASK-REQ-030-06](./TASK-REQ-030-06-risk-gate-acceptance-and-drift-approval.md) | `RiskGate`, `AcceptRisk`, `OverrideRiskGate`, drift Approval, `PlanPreconditions` theo rủi ro | P1 | `request-service` (mới) | 05, TASK-REQ-009-04, 010-03, 013-05, 012-03, 014-04, 024-01 |
| [TASK-REQ-030-07](./TASK-REQ-030-07-impact-read-api-graph-compare-narrator.md) | RPC đọc, `GetImpactGraph` (4 lens), `CompareImpact`, heatmap, `ImpactNarrator` | P1 | `request-service` (mới), `proto` | 03, 05, CR-REQ-032, 036 |
| [TASK-REQ-030-08](./TASK-REQ-030-08-risk-policy-shadow-enforce-calibration-and-wiring.md) | `RiskPolicy` admin, shadow/enforce, `impact-calibrate`, chỉ số, e2e | P1 | `request-service` (mới) | 02 đến 07, TASK-REQ-024-01/07, 025-03 |

## Sơ đồ thứ tự

```
030-01 ─▶ 030-03 ─┐
030-02 ───────────┼─▶ 030-05 ─┬─▶ 030-06 ─┐
030-04 ───────────┘           └─▶ 030-07 ─┴─▶ 030-08
   │
   └─ giai đoạn 0 (impact-calibrate, task 08 bước 1): chạy được ngay sau 030-02 + 030-04, TRƯỚC 030-05
```

01, 02, 04 song song; 03 sau 01 và 02; 05 sau 02, 03, 04; 06 và 07 sau 05 (06 còn cần SOL-009 bổ sung `ApprovalGuard`); 08 cuối.

## Ghi chú

- **Số migration:** `NNNN` của task 01 là số lớn nhất hiện có trong `request-service/migrations` cộng một, bắt buộc `ls` lúc làm; số `request-service` chồng nhau giữa các CR 001/002/004/006 nên báo người điều phối.
- **Mặc định `shadow`:** `REQUEST_IMPACT_ENABLED=false` mặc định; bật rồi vẫn là `shadow` cho tới khi admin tenant đặt `active` qua `SetRiskPolicy` (task 08). Không có đường nào chặn cổng bằng điểm chưa hiệu chỉnh.
- **Phụ thuộc cứng ngoài feature:** SOL-009 cần `ApprovalGuard` (task 06); SOL-012 cần nhãn `gate:feature_flag`, `check:rollback_rehearsal`; SOL-029 cần `AgentRelay` và sự kiện `execution.verified`. Các solution đó do agent khác soạn; task chỉ ghi giao diện cần.
- **Mẫu thật bắt buộc:** task 04 bước 1 (chạy GitNexus CLI lưu mẫu) và task 08 bước 1 (ca lịch sử với `expected_level` do người duyệt điền) không được bỏ qua; không dùng mẫu bịa.
- **Chưa kiểm chứng:** định dạng đầu ra GitNexus, thời gian `impact` trên repo 247 nghìn symbol, worktree dùng chung index hay không, `buf breaking` qua SSH và với clone nông, ngưỡng và trọng số `rp/1`. Mỗi task nêu ở mục Rủi ro.
- Không file nào tên `helpers`/`utils`/`common`/`misc`; không `max-lines` disable; Git theo `guides/reference/git-compatibility.md` (`rev-list --count`, `rev-parse`, `diff --name-only -z`, `ls-files`, `status --porcelain=v1`).
