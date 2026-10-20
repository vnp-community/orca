# Giai đoạn 8: Profile Extended (TDD-Ontology module 04)

Mục tiêu: khi KG TDD có entity của module 04, tầng A sinh thêm điều kiện theo ONTOLOGY-TEST-TSO mục 7.2.

Tham chiếu: ONTOLOGY-TDD mục 5; ONTOLOGY-TEST-TSO mục 7.2; TDD v2 RISK-04, OQ-02.

Điều kiện bắt đầu: OQ-O1 (tên quan hệ Extended) đã có câu trả lời, hoặc chấp nhận tên đề xuất của ONTOLOGY-TDD mục 5 qua cấu hình.

---

## T8.1 Cấu hình profile

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T5.2
- **File:** `tsagent/ontology/profiles.yaml`, `tsagent/analyze/profiles.py`

Checklist:
- [x] Ánh xạ tên quan hệ Extended (mặc định theo ONTOLOGY-TDD mục 5)
- [x] Nhận diện profile từ `ontologyVersion` của gói (`+ext-N`); Extended chỉ bật khi gói khai
- [x] Test `tests/unit/test_profiles.py`

Tiêu chí xong: gói Core không đổi kết quả; gói Extended bật luật Extended.

---

## T8.2 Fixture KG theo module 04

- **Trạng thái:** Chờ review của người nắm ontology TDD (fixture dựng 2026-10-12 bằng `scripts/make_ext_fixture.py`)
- **Phụ thuộc:** T3.1

Checklist:
- [x] Dựng gói `tests/fixtures/minipay/cr001-ext.tdd-kg.json`: DesignObligation (OBL-1..4), Operation `ExecuteScheduledRun`, Invariant `INV-OneTransferPerRun`, IdempotencyPolicy, ConcurrencyPolicy, TimeoutBudget, Transaction, PermissionDesign cho 6 endpoint, ErrorContract có `sideEffectState`
- [x] Gói qua I1–I8
- [ ] Người nắm ontology TDD review fixture (cần người; PermissionDesign trỏ endpoint qua `enforcementPoints` vì endpoint nằm ở solution.md)

Tiêu chí xong: fixture được duyệt.

---

## T8.3 Luật seed Extended

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T8.1, T8.2
- **File:** `tsagent/analyze/seed_ext.py`

Checklist:
- [x] DesignObligation applicable → TestObligation `origin=designObligation`; DesignGap → TestGap
- [x] Operation: postcondition, precondition âm, errorOutcome
- [x] InteractionFlow → ScenarioPath; DecisionTable → mỗi row và "không khớp dòng nào"
- [x] Invariant, ConcurrencyPolicy → `invariant`, `concurrency`
- [x] ErrorContract → `errorHandling` + FaultModel theo `sideEffectState`
- [x] TimeoutBudget + RetryPolicy → `timing` theo `endToEndDeadline`
- [x] IdempotencyPolicy → gửi lại, khác payload, trùng đồng thời, sau retention
- [x] Transaction, Saga, Migration → `atomicity`, `compensation`, `migration`
- [x] ContractCompatibility, PermissionDesign, SecretHandling, AuditDesign, LoadProfile, RecoveryPlan, ObservabilityDesign, RolloutPlan theo bảng 7.2
- [x] TSO-014, 015, 017 phần Extended
- [x] Test `tests/unit/test_seed_ext.py` (TST-05 phần Extended)

Tiêu chí xong: tầng A của fixture Extended đủ điều kiện theo bảng 7.2; render ra TC tương ứng.
