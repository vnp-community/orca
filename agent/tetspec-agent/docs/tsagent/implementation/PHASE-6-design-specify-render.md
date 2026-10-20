# Giai đoạn 6: Tầng B (thiết kế), tầng C (đặc tả), render

Mục tiêu: từ tầng A sinh TR, TestDesign, CoverageItem, TestCase; render ba tài liệu md theo 29119-3 từ `tso:`.

Tham chiếu: ONTOLOGY-TEST-TSO mục 5, 6, 9; TDD v2 FR-006, 007, 009; NFR-001; ADR-V2-04, 05, 11.

---

## T6.1 Luật thiết kế

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T5.4
- **File:** `tsagent/design/rules.py`

Checklist:
- [x] `conditionKind` → kỹ thuật chính (ONTOLOGY-TEST-TSO mục 7)
- [x] Gom điều kiện thành TR theo (chủ thể, mức, `testType`); mức L1 cho ScenarioPath, L3 cho errorHandling, timing, idempotency của Integration và phụ thuộc Module, L2 còn lại
- [x] `testType` theo ISO 25010 từ `conditionKind`
- [x] `verificationMethod`: `benchmark` cho `performance`, `test` cho phần còn lại (cấu hình được)
- [x] Khung PassCriterion: mỗi điều kiện một mục đánh số
- [x] Khung TestDesign mỗi (TR, kỹ thuật); CoverageItem mỗi điều kiện, Partition sinh nhiều CoverageItem
- [x] Ghi stage bước `design`
- [x] Test `tests/unit/test_design_rules.py` trên CR-001

Tiêu chí xong: mọi TestCondition của CR-001 có CoverageItem ở chế độ `none`.

---

## T6.2 LLM điền tầng B, luật TSO-008..010, 016

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T6.1, T4.3
- **File:** `tsagent/design/fill.py`, `tsagent/design/prompts/design.md`

Checklist:
- [x] LLM viết `statement` (điều cần kiểm, không chép tên FR), `scopeNote`, lời văn PassCriterion
- [x] LLM chọn `value` cho CoverageItem trong phạm vi Partition và oracle
- [x] Chế độ `none`: văn bản mẫu luật
- [x] TSO-008, 009, 010, 016; gate GB
- [x] Test `tests/integration/test_design.py` với `FakeLLM` (TST-08)

Tiêu chí xong: TR của CR-001 có `statement` khác tên FR, PassCriterion đánh số, `scopeNote`.

---

## T6.3 Tầng C: đặc tả

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T6.2
- **File:** `tsagent/specify/rules.py`, `tsagent/specify/fill.py`, `tsagent/specify/prompts/specification.md`, `tsagent/validate/secrets.py`

Checklist:
- [x] Khung TestCase mỗi CoverageItem
- [x] TestSet theo (mức, chủ thể); TestEnvironment theo mức
- [x] StubSpec cho mọi `dependsOn`, `integratesVia` outbound của chủ thể L3
- [x] FaultInjection cho mọi FaultModel
- [x] LLM viết bước, `targetCode`, dữ liệu, fixture, assertion; số liệu và mã lỗi lấy từ oracle, LLM không đổi được
- [x] Assertion luôn có oracle; `expected` đối chiếu oracle
- [x] Dữ liệu nhạy cảm chỉ `secretRef`; quét mẫu bí mật
- [x] Chế độ `none`: bước, assertion theo mẫu luật
- [x] TSO-011..015, 017..019
- [x] Test `tests/integration/test_specify.py` (TST-09): assertion sai oracle, fault model thiếu injection, bí mật đều bị bắt

Tiêu chí xong: gate GS (trừ duyệt) đạt trên CR-001 ở chế độ `none` và với `FakeLLM`.

---

## T6.4 Render ba tài liệu

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T6.3, T2.2
- **File:** `tsagent/render/`

Checklist:
- [x] 01-test-requirements.md: thông tin tài liệu, test basis, TR theo mức với tiêu chí đánh số và `scopeNote`, truy vết nguồn → obligation → condition → TR, TestGap mở, báo cáo gate
- [x] 02-test-design.md: TestDesign, kỹ thuật, CoverageItem, oracle, Partition, FaultModel, stub, dữ liệu, môi trường, test set
- [x] 03-test-cases.md: TC với bước, dữ liệu, fixture, stub, fault injection, assertion kèm oracle
- [x] Lịch sử phiên bản, phạm vi hồi quy
- [x] Xác định: thứ tự ổn định, không dấu thời gian trong thân, băm trong front matter
- [x] Từ chối ghi đè bản sửa tay (bảng `rendered_file`)
- [x] Render phiên bản cũ ra stdout hoặc đường dẫn khác
- [x] Test `tests/integration/test_render.py`: golden PAY; chạy hai lần giống từng byte (TST-11); bản sửa tay bị từ chối

Tiêu chí xong: ba tài liệu CR-001 render được; TST-11 xanh.

---

## T6.5 So sánh với mốc v1

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T6.4, T5.6

Checklist:
- [x] Mở rộng `scripts/compare_baseline.py`: số TR, TC theo mức; tỷ lệ TR có tiêu chí cụ thể; assertion có oracle; test NFR vô nghĩa (phải 0)
- [x] Ghi `implementation/COMPARE-BC.md`
- [x] TST-21: mọi khoảng trống của v1 có TC tương ứng

Tiêu chí xong: TST-21 xanh.
