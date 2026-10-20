# Giai đoạn 5: Tầng A (phân tích)

Mục tiêu: từ baseline nguồn sinh tầng A đủ theo luật seed Core; LLM điền nội dung có căn cứ; kiểm TSO-001..007.

Tham chiếu: ONTOLOGY-TEST-TSO mục 4, 7.1, 8, 9; TDD v2 mục 5.6, FR-004, 005; MECH-10, 11, 12; CON-11.

---

## T5.1 Mã nút

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.2
- **File:** `tsagent/analyze/codes.py`

Checklist:
- [x] Hàm sinh mã theo TDD v2 mục 5.6 (`TOB-`, `TCND-`, `TORC-`, `TR-`, `TD-`, `TCOV-`, `TC-`), suy từ mã nguồn, ổn định qua phiên bản
- [x] Chuẩn hóa ký tự, giới hạn độ dài (băm phần đuôi khi quá dài)
- [x] Test `tests/unit/test_codes.py`: ổn định, không trùng trên fixture

Tiêu chí xong: test xanh.

---

## T5.2 Luật seed profile Core

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T3.3, T5.1
- **File:** `tsagent/analyze/seed_core.py`

Checklist:
- [x] TestBasisItem cho nút nguồn được tham chiếu; `inScope=false` khi `testScope=context`
- [x] FR + AC: obligation; mỗi AC một `functional`, oracle `acceptanceCriterion`
- [x] FR thiếu AC: TestGap "thiếu AC"
- [x] NFR: điều kiện theo `category`; oracle `threshold` với `measurement` từ `condition`, thiếu thì TestGap hỏi cách đo
- [x] Endpoint + Field: Partition thiếu, min/max, độ dài, enum → `inputValidation`; Endpoint + Response → `functional` hoặc `inputValidation`, oracle `errorContract`
- [x] StateMachine + Transition: `stateTransition`; mỗi guard một `guardViolation`
- [x] Module `dependsOn`: `errorHandling`
- [x] Integration: ErrorMapping → `errorHandling` + FaultModel (`unknownOutcome` → `afterCommit`); Timeout + Retry + Scenario → `timing` trường hợp xấu nhất; CircuitBreaker; `idempotent` → `idempotency`; PartnerLimit; Channel trùng, DLQ; inbound + SecurityScheme → `access`
- [x] Scenario ≥ 2 Module: ScenarioPath main và failure; `timing` theo `responseBudgetMs`
- [x] RiskRating từ `priority` của Requirement và Risk `impact=High`
- [x] TestGap từ Assumption, OpenQuestion chưa Confirmed **chỉ** qua `assumes`, `blockedBy` của chính chủ thể
- [x] Test `tests/unit/test_seed_core.py`: từng luật trên PAY và CR-001; bảng kỳ vọng số nút (TST-05 phần Core)

Tiêu chí xong: tầng A seed của CR-001 có các điều kiện v1 bỏ sót trong BASELINE-V1.md mà không cần LLM.

---

## T5.3 Gói ngữ cảnh

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T5.2
- **File:** `tsagent/analyze/context.py`

Checklist:
- [x] `context_pack(subject)`: nút chủ thể, láng giềng 1–2 bước, nút seed cần điền, văn bản nguồn liên quan
- [x] Giới hạn kích thước, cắt theo ưu tiên quan hệ, ghi chú khi cắt
- [x] Danh sách trắng mã
- [x] Băm ổn định để trúng cache
- [x] Test `tests/unit/test_context.py`

Tiêu chí xong: gói ngữ cảnh của mọi chủ thể CR-001 dưới giới hạn.

---

## T5.4 LLM điền tầng A

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T5.3, T4.3
- **File:** `tsagent/analyze/fill.py`, `tsagent/analyze/prompts/analysis.md`

Checklist:
- [x] Prompt: vai trò, quy tắc (chỉ dùng mã trong danh sách trắng; mỗi điều kiện có `basedOn`, `evidence`; thiếu thông tin thì trả `gaps`, không đoán)
- [x] Ghép theo `seedCode`; nội dung LLM ghi `derivation=llm`, `model`, `promptHash`, `confidence`
- [x] `proposedConditions` thành nút `origin=llmProposal`
- [x] `gaps` thành TestGap có `askedQuestion` và owner
- [x] Kiểm sau mỗi lượt (mã lạ, thiếu evidence, `seedCode` không có): một lượt sửa; vẫn sai thì giữ seed và tạo TestGap
- [x] Chế độ `none`: nội dung mẫu luật lấy từ văn bản nguồn
- [x] Ghi stage bước `analysis`
- [x] Test `tests/integration/test_analyze.py` với `FakeLLM`: hợp lệ; bịa mã; thiếu evidence; JSON hỏng; hết trần; breaker mở (TST-06, 07)

Tiêu chí xong: test xanh; chạy thật CR-001 bằng `claude-cli` với trần nhỏ cho tầng A hợp lệ.

---

## T5.5 Luật TSO-001..007

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T5.2
- **File:** `tsagent/validate/tso_rules.py`, `tsagent/validate/report.py`

Checklist:
- [x] Khung báo cáo: luật, mức (chặn, chặn baseline, cảnh báo), số đã kiểm, danh sách vi phạm có mã nút; gate
- [x] TSO-001..007 theo ONTOLOGY-TEST-TSO mục 9
- [x] Gate GA
- [x] Test `tests/unit/test_tso_rules_a.py`: mỗi luật có ca đạt và không đạt

Tiêu chí xong: gate GA tính được trên CR-001.

---

## T5.6 So sánh tầng A với mốc v1

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T5.4, T5.5, T0.1

Checklist:
- [x] `scripts/compare_baseline.py`: đếm điều kiện theo `conditionKind`, đối chiếu khoảng trống trong BASELINE-V1.md
- [x] Ghi `implementation/COMPARE-A.md` (chế độ `none` và `claude-cli`)

Tiêu chí xong: mỗi khoảng trống của v1 có điều kiện tương ứng hoặc lý do chưa có.
