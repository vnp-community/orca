# So sánh tầng A v2 với mốc v1 (T5.6)

Mốc v1: [BASELINE-V1.md](./BASELINE-V1.md). Sinh bằng `scripts/compare_baseline.py`. Chế độ `claude-cli` chỉ chạy cho ba chủ thể liên quan khoảng trống G1–G3 (trần 3 lượt) để giữ chi phí thấp; các chủ thể khác giữ nội dung mẫu luật.

Kết luận: G1–G5, G13, G14 được phủ ở tầng A ngay ở chế độ `none` (không cần LLM). G6, G8, G11, G12 thuộc tầng B; G9 thuộc Giai đoạn 7; G7 và G10 có lý do ở bảng (phạm vi lát cắt, nguồn chưa có).

## Chế độ `none`

Chạy 2026-10-12, đầu vào cr001-slice.tdd-kg.json, solution.md, supplement.md. Gate GA: gate GA: đạt; TSO-007(15).

| Lớp | Số nút |
| --- | --- |
| FaultModel | 5 |
| Partition | 33 |
| RiskRating | 12 |
| ScenarioPath | 2 |
| TestBasisItem | 158 |
| TestCondition | 163 |
| TestGap | 15 |
| TestObligation | 29 |
| TestOracle | 135 |

| conditionKind | Số điều kiện |
| --- | --- |
| access | 12 |
| concurrency | 4 |
| errorHandling | 5 |
| functional | 49 |
| guardViolation | 13 |
| inputValidation | 48 |
| observability | 3 |
| performance | 5 |
| resilience | 5 |
| stateTransition | 15 |
| timing | 4 |

Lượt gọi LLM: 0; nút `derivation=llm`: 0; điều kiện LLM điền: 0; đề xuất thêm: 0; mục bị loại: 0.

| # | Khoảng trống v1 | Tầng A v2 | Minh chứng |
| --- | --- | --- | --- |
| G1 | trùng đồng thời | có, 4 điều kiện `concurrency` | `TCND-SCHEDULED-TRANSFER-AC-NFR-011-1-concurrency`, `TCND-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency`, `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-concurrency-parallel`, `TCND-SCHEDULED-TRANSFER-NFR-011-concurrency-target` |
| G2 | chết giữa chừng | có, 4 điều kiện (FaultModel `afterCommit` hoặc khởi động lại) | `TCND-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency`, `TCND-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience`, `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid`, `TCND-SCHEDULED-TRANSFER-NFR-012-resilience-target` |
| G3 | ngân sách xấu nhất | có, 1 điều kiện `timing` trường hợp xấu nhất | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-worst-INT-01` |
| G4 | throttle 20 lần/chu kỳ, 10 rps | có, 2 điều kiện trên MECH-SchedulerThrottle | `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-at-limit`, `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit` |
| G5 | NFR mất 99 %, 100 lần cùng lúc | có, measurement giữ nguyên văn (2/2) | `TCND-SCHEDULED-TRANSFER-NFR-010-timing-target`, `TCND-SCHEDULED-TRANSFER-NFR-013-performance-target` |
| G6 | test NFR vô nghĩa (+1 cho chỉ tiêu 0/100 %) | chưa đo được ở tầng A; kiểm bằng TSO-016 ở tầng B (Giai đoạn 6) | — |
| G7 | không có L3 cho MOD-Notify, INT-02, INT-03 | các nút này là `context` trong lát cắt CR-001 nên không là chủ thể; mức L3 do tầng B quyết định | — |
| G8 | chỉ một hành trình L1 | ScenarioPath main + failure cho mọi Scenario có ≥ 2 Module; gom L1 ở tầng B (Giai đoạn 6) | `TPATH-SCHEDULED-TRANSFER-SCN-ScheduledRun-fail-INT-01`, `TPATH-SCHEDULED-TRANSFER-SCN-ScheduledRun-main` |
| G9 | không có hồi quy | chọn hồi quy ở Giai đoạn 7 (`tso_regression`) | — |
| G10 | BR-10 (UTC+7) không có | BR-10 chỉ có trong srs.md, không thuộc đầu vào; KG TDD chưa có lớp BusinessRule (cần profile Extended: DecisionTable/Invariant) | — |
| G11 | TR chép tên FR, tiêu chí dồn một ô | tầng A tách điều kiện theo AC/Field/Response; tiêu chí đánh số ở tầng B (Giai đoạn 6) | — |
| G12 | TR chồng lấn | `scopeNote` ở tầng B (Giai đoạn 6) | — |
| G13 | pendingConfirmation lan 123 nút | 0 TestGap từ giả định chưa xác nhận (chỉ theo `assumes`/`blockedBy` trực tiếp) | — |
| G14 | phân quyền từng endpoint | có điều kiện `access` cho 6/6 endpoint trong phạm vi | `Cancel`, `Create`, `Get`, `ListRuns`, `ListSchedules`, `Patch` |

## Chế độ `claude-cli` (chỉ MECH-RunClaim,NFR-011,SCN-ScheduledRun, trần 3)

Chạy 2026-10-12, đầu vào cr001-slice.tdd-kg.json, solution.md, supplement.md. Gate GA: gate GA: đạt; TSO-007(18).

| Lớp | Số nút |
| --- | --- |
| FaultModel | 5 |
| Partition | 33 |
| RiskRating | 12 |
| ScenarioPath | 2 |
| TestBasisItem | 158 |
| TestCondition | 170 |
| TestGap | 18 |
| TestObligation | 30 |
| TestOracle | 142 |

| conditionKind | Số điều kiện |
| --- | --- |
| access | 12 |
| atomicity | 1 |
| concurrency | 5 |
| errorHandling | 6 |
| functional | 49 |
| guardViolation | 13 |
| inputValidation | 48 |
| observability | 4 |
| performance | 6 |
| resilience | 5 |
| stateTransition | 16 |
| timing | 5 |

Lượt gọi LLM: 3; nút `derivation=llm`: 41; điều kiện LLM điền: 9; đề xuất thêm: 7; mục bị loại: 0.

| # | Khoảng trống v1 | Tầng A v2 | Minh chứng |
| --- | --- | --- | --- |
| G1 | trùng đồng thời | có, 5 điều kiện `concurrency` | `TCND-SCHEDULED-TRANSFER-AC-NFR-011-1-concurrency`, `TCND-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency`, `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-concurrency-parallel`, `TCND-SCHEDULED-TRANSFER-NFR-011-concurrency-target` |
| G2 | chết giữa chừng | có, 5 điều kiện (FaultModel `afterCommit` hoặc khởi động lại) | `TCND-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency`, `TCND-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience`, `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid`, `TCND-SCHEDULED-TRANSFER-NFR-012-resilience-target` |
| G3 | ngân sách xấu nhất | có, 1 điều kiện `timing` trường hợp xấu nhất | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-worst-INT-01` |
| G4 | throttle 20 lần/chu kỳ, 10 rps | có, 2 điều kiện trên MECH-SchedulerThrottle | `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-at-limit`, `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit` |
| G5 | NFR mất 99 %, 100 lần cùng lúc | có, measurement giữ nguyên văn (2/2) | `TCND-SCHEDULED-TRANSFER-NFR-010-timing-target`, `TCND-SCHEDULED-TRANSFER-NFR-013-performance-target` |
| G6 | test NFR vô nghĩa (+1 cho chỉ tiêu 0/100 %) | chưa đo được ở tầng A; kiểm bằng TSO-016 ở tầng B (Giai đoạn 6) | — |
| G7 | không có L3 cho MOD-Notify, INT-02, INT-03 | các nút này là `context` trong lát cắt CR-001 nên không là chủ thể; mức L3 do tầng B quyết định | — |
| G8 | chỉ một hành trình L1 | ScenarioPath main + failure cho mọi Scenario có ≥ 2 Module; gom L1 ở tầng B (Giai đoạn 6) | `TPATH-SCHEDULED-TRANSFER-SCN-ScheduledRun-fail-INT-01`, `TPATH-SCHEDULED-TRANSFER-SCN-ScheduledRun-main` |
| G9 | không có hồi quy | chọn hồi quy ở Giai đoạn 7 (`tso_regression`) | — |
| G10 | BR-10 (UTC+7) không có | BR-10 chỉ có trong srs.md, không thuộc đầu vào; KG TDD chưa có lớp BusinessRule (cần profile Extended: DecisionTable/Invariant) | — |
| G11 | TR chép tên FR, tiêu chí dồn một ô | tầng A tách điều kiện theo AC/Field/Response; tiêu chí đánh số ở tầng B (Giai đoạn 6) | — |
| G12 | TR chồng lấn | `scopeNote` ở tầng B (Giai đoạn 6) | — |
| G13 | pendingConfirmation lan 123 nút | 0 TestGap từ giả định chưa xác nhận (chỉ theo `assumes`/`blockedBy` trực tiếp) | — |
| G14 | phân quyền từng endpoint | có điều kiện `access` cho 6/6 endpoint trong phạm vi | `Cancel`, `Create`, `Get`, `ListRuns`, `ListSchedules`, `Patch` |
