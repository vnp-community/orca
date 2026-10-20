# Task: tích hợp tsagent vào workflow tdd-agent

Chi tiết từng task của [PLAN.md](PLAN.md). Mục số `§x` trỏ tới [SOLUTION](SOLUTION-INTEGRATE-TDD-AGENT.md), ca `TC-xx` ở SOLUTION mục 11.

## Quy ước theo dõi

- Mỗi task có checklist. **Chỉ tick `[x]` khi mục đã xong và test liên quan xanh.**
- Checklist bắt đầu bằng viết test (test trước, đỏ), kết thúc bằng test xanh + lint.
- Bị chặn: thêm một dòng `> BLOCKED: <lý do>` ngay dưới tiêu đề task; xóa dòng khi gỡ được.
- Xong task: điền dòng `Bằng chứng:` (lệnh test + kết quả, commit nếu có).
- Sau mỗi lần cập nhật, chạy `python scripts/tsagent/plan_progress.py` để làm mới bảng tiến độ trong PLAN.md.

---

## P0. Chuẩn bị

### T0.1 Ghi mốc test hiện tại

- [x] Chạy `pytest` toàn repo (tdd-agent + `tests/testspec_agent`), ghi số pass/fail/skip vào Nhật ký của PLAN
- [x] Chạy `ruff check` (cả `ruff-tsagent.toml`), ghi kết quả
- [x] Ghi các test đang đỏ sẵn (nếu có) để không nhầm là lỗi do thay đổi mới

Bằng chứng: `pytest -o addopts="" -m "not llm_real"` → 607 passed, 2 skipped, 2 deselected (167 s). `ruff --config ruff-tsagent.toml src/tsagent tests/testspec_agent` sạch. `ruff check src/tdd_agent` có sẵn 405 lỗi (I001, F401, FURB167…): quy ước là không tăng số lỗi ở tệp đã chạm, tệp mới phải sạch. Không có test đỏ sẵn.

---

## P1. tsagent

### T1.1 Cấu hình LLM bắt buộc (§8.1)

Phụ thuộc: T0.1 · Ca: TC-21, TC-22, TC-23, TC-24

- [x] `tests/testspec_agent/conftest.py`: fixture `autouse` đặt `TSAGENT_LLM=none`
- [x] Viết test TC-21, TC-22, TC-23, TC-24 (đỏ)
- [x] `config.py`: `_provider()` mặc định `""`, chỉ báo lỗi khi giá trị lạ
- [x] `errors.py` / `llm/__init__.py`: `LLMNotConfigured`, `LLMKeyMissing`, `LLMModelMissing` có `code`
- [x] `llm/__init__.py`: `check_llm(cfg)` (không gọi mạng); `make_provider` gọi `check_llm`
- [x] Bỏ model mặc định `claude-sonnet-5-5` của anthropic; openai/anthropic thiếu model → `LLM_MODEL_MISSING`
- [x] `Orchestrator.__init__` báo lỗi trước mọi bước khi chưa đặt chế độ
- [x] `tsagent` CLI: lỗi cấu hình in `code`, thoát mã 2; `serve` không khởi động
- [x] `selfcheck` đặt `none` cho chính nó
- [x] Test tsagent cũ xanh hết; ruff sạch

Bằng chứng: `tests/testspec_agent/unit/test_llm_config.py` (8 ca) xanh; `pytest tests/testspec_agent` 268 passed; ruff sạch. `test_foundation::test_defaults` đổi kỳ vọng mặc định thành `""`.

### T1.2 Bỏ qua cache khi retry (§8.2)

Phụ thuộc: T0.1 · Ca: TC-25 (phần `fresh`)

- [x] Test: `complete_json(..., fresh=True)` không đọc cache nhưng vẫn ghi
- [x] `llm/client.py`: tham số `fresh=False`
- [x] `Filler.call`, `StepFiller.call` nhận `fresh` và truyền xuống
- [x] Test xanh; ruff sạch

Bằng chứng: `test_group_retry.py::test_fresh_skips_cache_read_but_writes` xanh; `Filler.call`/`StepFiller.call` nhận `fresh`.

### T1.3 Retry theo nhóm (§8.3)

Phụ thuộc: T1.2 · Ca: TC-25, TC-26, TC-29, TC-31, TC-32

- [x] `config.py`: `llm_group_retries` từ `TSAGENT_LLM_GROUP_RETRIES` (mặc định 2, tối thiểu 0)
- [x] Viết test TC-25, TC-26, TC-29, TC-31, TC-32 bằng `FakeLLM` (đỏ)
- [x] `analyze/fill.py`: `run_groups`, `grade` (ok / partial / failed), `better`, `wait_breaker`
- [x] `Filler.run` dùng `run_groups`; áp kết quả tuần tự theo thứ tự nhóm
- [x] `StepFiller.run` dùng `run_groups`
- [x] Nhóm `failed` sau hết lượt: giữ mẫu luật + TestGap `llm-failed` kèm lỗi đại diện
- [x] `FillResult`: `failed_groups`, `retried`, `retry_calls`; lượt retry tính vào `llm_caps`
- [x] Lỗi `fatal` (401/403, chưa đăng nhập) không retry, dừng run
- [x] Test xanh; kết quả 1 luồng = 4 luồng (`graph_hash`); ruff sạch

Bằng chứng: `tests/testspec_agent/integration/test_group_retry.py` 11 ca xanh (TC-25, 26, 29, 31, 32 + giữ lần thử tốt nhất). Lệch nhỏ: tầng B, C (StepFiller) không có cơ chế TestGap riêng, nhóm lỗi ghi vào `skipped` + `failed_groups` + `run.result.warnings`; TestGap `llm-failed` chỉ ở tầng A. Test cũ đổi kỳ vọng: số lượt sửa nhân (N+1) (`test_analyze`).

### T1.4 Ngưỡng 50 % nhóm lỗi (§8.3)

Phụ thuộc: T1.3 · Ca: TC-27, TC-28, TC-30 (phần không áp ngưỡng)

- [x] Viết test TC-27, TC-28 (đỏ)
- [x] `LLMGroupsFailed(TsagentError)`, `code = "LLM_GROUPS_FAILED"`
- [x] Kiểm cuối mỗi bước analysis/design/specification: `failed * 2 > attempted` → ném lỗi
- [x] `attempted` loại trừ nhóm dùng lại, `LLMDisabled`, `BudgetExceeded`
- [x] Run `Failed`, `notes` nêu `<bước>: k/n nhóm lỗi sau N lần retry: <lỗi>`
- [x] Dưới ngưỡng: `run.result.warnings` liệt kê nhóm lỗi
- [x] `llm = none`: không retry, không áp ngưỡng
- [x] Test xanh; ruff sạch

Bằng chứng: `test_group_retry.py` (ngưỡng ở mức bước), `test_run_origin_llm.py::test_run_fails_when_most_groups_fail`, `::test_run_ok_with_warning_when_few_groups_fail` xanh. `LLMGroupsFailed` mang `result`, `failed`, `attempted`. Test cũ giả lập "mọi nhóm hỏng" (`test_analyze`, `test_design`, `test_specify`) đổi kỳ vọng sang `LLMGroupsFailed`; `test_security` dùng LLM tiêm ở lượt chẵn (retry lấy câu trả lời sạch).

### T1.5 Tham số `origin` của run (§7.3)

Phụ thuộc: T0.1

- [x] Test: `start(..., origin={...})` lưu vào `run.params.origin`, không đổi `runKey`
- [x] `orchestrator/run.py`: `prepare`/`start` nhận `origin`
- [x] Test câu SQL truy ngược phiên bản → baseline → run → origin
- [x] Test xanh; ruff sạch

Bằng chứng: `test_run_origin_llm.py::test_origin_saved_outside_run_key` xanh (gồm câu SQL truy ngược).

### T1.6 Front matter `llm:` (§8.4)

Phụ thuộc: T0.1 · Ca: TC-30

- [x] Test: ba tài liệu có `llm`, `llm_model`; chế độ `none` có dòng cảnh báo
- [x] `render/__init__.py`: `front_matter` lấy `run.mode`, `run.model`
- [x] Cập nhật fixture/golden của test render nếu có
- [x] Test xanh; ruff sạch

Bằng chứng: `test_run_origin_llm.py` (none + fake) xanh; golden PAY cập nhật có chủ đích (`UPDATE_GOLDEN=1`, thêm `llm: unknown`, `llm_model`).

### T1.7 Cập nhật nơi dùng tsagent độc lập (§8.5)

Phụ thuộc: T1.1, T1.3

- [x] `deploy/tsagent/.env.example`, compose: `TSAGENT_LLM=` để trống + chú thích bắt buộc; thêm `TSAGENT_LLM_GROUP_RETRIES`
- [x] `scripts/tsagent/*`: đọc `TSAGENT_LLM` tường minh, báo lỗi rõ nếu thiếu
- [x] `docs/tsagent/README.md`: bảng biến môi trường
- [ ] Build image `deploy/tsagent` không lỗi (nếu có Docker)

Bằng chứng: `.env.example` để trống `TSAGENT_LLM` + `TSAGENT_LLM_GROUP_RETRIES`; compose dùng `${TSAGENT_LLM:?…}`: `docker compose config` không đặt biến → lỗi (exit 1), đặt `none` → đạt. Script `compare_baseline.py`, `perf.py` vốn đặt `llm_provider` tường minh, không cần sửa. README cập nhật. Chưa build image (để T6).

---

## P2. Lõi tdd-agent

### T2.1 Mục cấu hình `[testspec]` (§9)

Phụ thuộc: T0.1 · Ca: TC-11

- [x] Viết test: giá trị mặc định; khóa lạ bị từ chối; giá trị sai → `CONFIG_INVALID`; biến `TDD_AGENT__TESTSPEC__*`
- [x] Viết test TC-11: đổi `group_retries` sau G2 → không cổng nào `stale`
- [x] `config.py`: `DEFAULTS["testspec"]`, kiểm giá trị
- [x] `FINGERPRINT_EXCLUDE = ("testspec",)` trong `fingerprint()`
- [x] Test tdd-agent cũ xanh; ruff sạch

Bằng chứng: `tests/test_testspec_config.py` 9 ca xanh; TC-11 ở mức luồng CR: `tests/test_testspec_flow.py::test_testspec_settings_do_not_invalidate_change_gates` xanh (đổi `group_retries` không làm cổng nào stale; đổi `llm` chỉ chạy lại GT).

### T2.2 Móc `ChildRuns` (§3.2)

Phụ thuộc: T0.1

- [x] Test: cổng giả gọi `ctx.children.start(...)` → run con có `parent_run_id`; `advance` run con chạy được trong lúc run cha giữ khóa
- [x] `orchestrator/context.py`: dataclass `ChildRuns`, trường `GateContext.children`
- [x] `orchestrator/engine.py`: gắn `ChildRuns` trong `_ctx`
- [x] `orchestrator/model.py`: `testspec_dispatched` vào `STATE_ORDER`
- [x] Test xanh; ruff sạch

Bằng chứng: `tests/test_child_runs.py` xanh; `tests/test_orchestrator.py` xanh.

---

## P3. Phép chiếu

### T3.1 `mapping.yaml` và bộ nạp (§6.2, §6.3)

Phụ thuộc: T0.1 · Ca: TC-14

- [x] Viết test hợp đồng TC-14: mọi lớp `data/ontology_v0.4/o4/classes.yaml` có dòng ánh xạ hoặc trong danh sách không chiếu (đỏ)
- [x] `agents/testspec/mapping.yaml` bản `version: 1`: lớp (điều kiện, lớp đích, thuộc tính), quan hệ (đảo chiều, gắn lại), danh sách không chiếu
- [x] Bộ nạp: kiểm schema của tệp, trả `version` + sha256 (dùng cho `extra_inputs`)
- [x] Đóng gói `mapping.yaml` trong `package-data` của `pyproject.toml`
- [x] Test xanh; ruff sạch

Bằng chứng: `tests/test_testspec_projection.py::test_every_v04_class_is_mapped_or_listed_unmapped` (TC-14), `::test_mapping_fingerprint_is_stable` xanh; bộ nạp kiểm lớp/quan hệ/hàm theo ontology tsagent (`TESTSPEC_MAPPING_INVALID`); `pyproject.toml` thêm `agents/testspec/*.yaml`. Giá trị nguồn tìm lần lượt trong `props`, `attrs`, `ext` (KG thật để giá trị ở cả ba chỗ).

### T3.2 Chuẩn hóa mã (§6.5)

Phụ thuộc: T0.1 · Ca: TC-13

- [x] Viết test TC-13: mã hợp lệ giữ nguyên; mã có `:`, `/`, `#`, khoảng trắng, chữ có dấu; ổn định; không trùng
- [x] `norm_code` + `code_map`
- [x] Test xanh; ruff sạch

Bằng chứng: `test_norm_code_*` (TC-13) xanh; `CodeMap` ghi `code_map`.

### T3.3 Đọc snapshot và chọn phạm vi (§5)

Phụ thuộc: T0.1 · Ca: TC-06, TC-07 (phần phạm vi)

- [x] Viết test trên fixture PBOM: phạm vi CR từ `cs_elements`; phạm vi theo TDD; láng giềng 2 bước; ref không phải mã phần tử → `unresolved_refs`
- [x] `agents/testspec/scope.py`: `load_snapshot` (kiểm hash), `cr_scope`, `tdd_scope`, `neighbors(depth=2)`
- [x] Chia phạm vi theo feature (`tdd_members`); phần tử không thuộc TDD nào → TDD đầu tiên
- [x] Tiêu chí CR → danh sách `AcceptanceCriterion` (văn bản từ G0, thiếu → `null`)
- [x] Lỗi: `SNAPSHOT_NOT_FOUND`, `TESTSPEC_SNAPSHOT_MISMATCH`, `TESTSPEC_EMPTY_SCOPE`, `TESTSPEC_PARAM:tdd`
- [x] Test xanh; ruff sạch

Bằng chứng: `test_cr_scope_targets_context_and_refs`, `test_tdd_scope_and_empty_scope` xanh. Lệch: tiêu chí CR gắn với mọi phần tử loại yêu cầu (Requirement, QualityTarget, BusinessRule) của dòng change spec. `tdd_scope` coi cả KG là một TDD khi KG không ghi thành viên TDD (dự án `init`).

### T3.4 Dựng gói `tdd-kg/v1` (§6.1, §6.4)

Phụ thuộc: T3.1, T3.2, T3.3 · Ca: TC-03 (phần I1–I8), TC-08, TC-12

- [x] Viết test: gói qua `tsagent.ingest` 0 lỗi; THIẾU → `OpenQuestion` + `blockedBy`; chiếu hai lần giống từng byte; `unmapped`, `dropped_edges` đúng (đỏ)
- [x] `agents/testspec/project.py`: nút (thuộc tính đích, `props.tddAgent`, `primary`, `testScope`), cạnh, `supersede`
- [x] Khối `ontology` với `required` nới (§6.4)
- [x] `generatedAt` = thời điểm tạo snapshot; sắp thứ tự; JSON ổn định
- [x] Nút sinh thêm: `-AC<n>`, `-TO`, `-RT`, `-Q-<attr>`
- [x] Test xanh; ruff sạch

Bằng chứng: `test_package_passes_tsagent_input_checks` (0 lỗi I1–I8), `test_missing_values_become_open_questions_not_values` (TC-08), `test_package_is_deterministic` (TC-12), `test_acceptance_without_text_is_gap` xanh; chạy thử tsagent `none` trên gói: run `done`, 3 TR, 5 TC, 5 TestGap. Lệch: phần tử trong phạm vi nhận biết bằng việc **không** có `testScope: "context"` (tsagent dùng `primary` cho Document chính, không dùng cho phần tử); gói có một nút `Document` chính cho TDD.

---

## P4. Cổng GT và TS1

### T4.1 `runner.py`: gọi tsagent (§7, §9.3)

Phụ thuộc: T1.1, T1.3, T1.4, T1.5, T1.6, T2.1, T3.4

- [x] Viết test: `tsagent_config` lấy đúng `[testspec]`, bỏ qua `TSAGENT_LLM`; đường dẫn db/cache/snapshots/specs
- [x] Preflight dùng `tsagent.llm.check_llm`
- [x] Gọi `Orchestrator.start(feature, [gói], origin=...)` cho từng feature
- [x] Ghi `.tdd-agent/runs/<child>/testspec/`: gói, `summary.json`, bản chụp tài liệu
- [x] Tóm tắt: TR, TC, khoảng trống, nhóm lỗi, `unmapped`, `unresolved_refs`, `uncovered_acceptance`, `deduplicated`
- [x] Ánh xạ kết quả tsagent → `GateOutcome` (§4.4)
- [x] Test xanh; ruff sạch

Bằng chứng: `tests/test_testspec_runner.py` (4 ca) xanh; `test_testspec_flow.py::test_child_run_writes_test_specs` kiểm gói, `summary.json`, bản chụp tài liệu, `origin` trong `tsagent.db`, `uncovered_acceptance`.

### T4.2 Cổng TS1 (§4)

Phụ thuộc: T4.1 · Ca: TC-03, TC-06, TC-07, TC-09 (phần run bị thay)

- [x] Viết test TC-03, TC-06, TC-07 (đỏ)
- [x] `ts1_gate`: nhận ra chế độ CR / tay; đọc `[testspec]` mỗi lần chạy
- [x] Kiểm run con bị thay (`child_run_id` trong task GT của run cha) → `ok {superseded_by}`
- [x] `ts1_extra_inputs`: chỉ version mapping
- [x] Test xanh; ruff sạch

Bằng chứng: `test_child_run_writes_test_specs` (TC-03, TC-07), `test_manual_run_for_one_tdd` (TC-06, `TESTSPEC_PARAM:tdd`), `test_changed_change_spec_starts_new_child_and_old_one_steps_aside` (run bị thay) xanh. Run bị thay nhận ra bằng run con mới nhất của run cha (`list_runs`), không đọc task GT (tránh tranh chấp khi GT chưa ghi xong).

### T4.3 Cổng GT (§3)

Phụ thuộc: T2.2, T4.2 · Ca: TC-01, TC-02, TC-04, TC-05, TC-10, TC-15

- [x] Viết test TC-01, TC-02, TC-04, TC-05, TC-10, TC-15 với `gate.LAUNCH` thay thế (đỏ)
- [x] `gt_gate`: `enabled = false` → `ok {enabled: false}`
- [x] Dựng request run con (§3.3) từ G0, G3, G4
- [x] Preflight lỗi → tạo run con, `advance` trong tiến trình; đạt → `launch_detached`
- [x] `launch_detached`: `start_new_session`, log `.tdd-agent/logs/testspec-<child>.log`, ghi `pid`
- [x] Đầu ra GT có `supersedes`
- [x] `gt_extra_inputs`: `enabled`, `llm`, `specs_dir`, mapping (không có `parallel`, `group_retries`)
- [x] Test xanh; ruff sạch

Bằng chứng: `test_gt_dispatches_child_and_g5_runs_on` (TC-01), `test_unset_llm_blocks_child_without_process` (TC-04), `test_api_provider_without_key` (TC-05), `test_disabled_branch_starts_nothing` (TC-10), `test_child_resumes_after_interruption` (TC-15), `test_launch_detached_command` xanh. TC-02: bộ test luồng CR cũ (G5→G7 hoàn tất khi run con bị chặn) xanh.

### T4.4 Khai báo trong `flows.py` (§2)

Phụ thuộc: T4.3 · Ca: TC-09, TC-11

- [x] Thêm GT sau G4 (`depends_on=("G0","G3","G4")`, `state_after="testspec_dispatched"`)
- [x] Đăng ký flow `testspec` (TS1)
- [x] Viết test TC-09: `change update` → G4 chạy lại → run con mới
- [x] Toàn bộ test luồng CR cũ (`test_agents_change_flow`, `test_orchestrator`, acceptance) xanh
- [x] Ruff sạch

Bằng chứng: `tests/test_testspec_flow.py` 9 ca xanh; toàn bộ test tdd-agent cũ xanh (383 passed) sau khi nối GT; ruff sạch ở tệp mới (`flows.py` có I001 sẵn từ trước).

---

## P5. Hiển thị và tài liệu

### T5.1 Dòng test specs trong `change status` (§10)

Phụ thuộc: T4.4 · Ca: TC-16

- [x] Viết test TC-16 cho mọi trạng thái của bảng §10 (đỏ)
- [x] `project.py`: tìm run con mới nhất theo `parent_run_id`, kiểm `pid` còn sống
- [x] Khóa `testspec` trong JSON tóm tắt + dòng hiển thị
- [x] Test xanh; ruff sạch

Bằng chứng: `tests/test_testspec_flow.py::test_change_status_line_*` (3 ca: đang sinh, dừng bất thường, xong, lỗi cấu hình, tắt) xanh; `change status` thêm khóa `testspec` + một dòng. Dòng hiển thị viết tiếng Anh theo các dòng sẵn có của `project.py`.

### T5.2 Tài liệu người dùng

Phụ thuộc: T5.1

- [x] `README.md`: nhánh test specs trong luồng CR, mục `[testspec]` bắt buộc chọn `llm`
- [x] `docs/CLI_Project_Usage.md`: `[testspec]`, chạy tay `run start testspec`, `run show`, đọc ba tài liệu
- [x] `docs/tsagent/README.md`: mục tích hợp cập nhật theo thiết kế mới
- [x] TDD và SOLUTION: đổi `status` sang `approved`/`implemented`, ghi lệch phát sinh khi làm (nếu có)

Bằng chứng: `README.md`, `docs/CLI_Project_Usage.md` (mục "Test specs (tsagent)", cổng GT, tệp `tsagent.db`), `docs/tsagent/README.md` (mục "Chạy trong workflow của tdd-agent"); `tdd-agent init` ghi sẵn `[testspec]` (`llm = ""`), đã thử `load_config` đọc được; TDD → `approved`, SOLUTION 0.2 `implemented` + mục 12.1 lệch khi hiện thực (C-08..C-15).

---

## P6. Kiểm chứng

### T6.1 Hồi quy toàn bộ

Phụ thuộc: T5.2

- [x] `pytest` toàn repo: số pass ≥ mốc T0.1 + test mới; 0 fail mới
- [x] `ruff check` sạch cả hai cấu hình
- [x] `tsagent selfcheck` đạt

Bằng chứng: `pytest -o addopts="" -m "not llm_real"` toàn repo: 683 passed, 2 skipped (mốc T0.1: 607; +76 test mới), 0 fail. `ruff --config ruff-tsagent.toml` sạch; `ruff check src/tdd_agent` 226 lỗi = đúng bằng HEAD (không thêm lỗi). `TSAGENT_LLM=none tsagent selfcheck` đạt.

### T6.2 E2E PBOM + CR T-14 (`llm = none`, tiến trình tách rời thật)

Phụ thuộc: T6.1

- [ ] Dự án tạm: `init` PBOM, `[testspec] llm = "none"`, `change start` T-14, duyệt tới G5
- [ ] `change status` hiện "đang sinh" rồi "xong"; ba tài liệu trong `docs/testspec/`
- [ ] Run CR đi hết G5 mà không chờ run con
- [ ] Ghi kết quả vào `docs/tsagent/implementation/` (tệp E2E mới)

Bằng chứng:

### T6.3 Đo MiniPay CR-001 với `claude-cli` (TC-41)

Phụ thuộc: T6.2

- [ ] `parallel = 4`, `group_retries = 2`; đo thời gian TS1 (chỉ tiêu ≤ 20 phút)
- [ ] Ghi số lượt gọi, số nhóm retry, số nhóm lỗi vào `docs/tsagent/implementation/PERF.md`
- [ ] So nội dung với lần sinh trước (`output-docs/CR-001-v2`): số TR/TC, khoảng trống

Bằng chứng:

---

## P7. Đọc KG thiết kế qua MCP (TDD 0.6, SOLUTION 0.3 mục 13)

### T7.1 Tool đọc `kg_elements` trên MCP server của tdd-agent

Phụ thuộc: T6.1

- [x] Viết test: lọc theo `ids` (kèm `missing`), theo `tdd` (cùng nghĩa `tdd_members`, KG không ghi thành viên thì cả KG), theo `classes`; phân trang; có `info` của snapshot
- [x] `mcp/tools.py`: `kg_elements(snapshot, ids, tdd, classes, offset)` trả `Paged`
- [x] `mcp/server.py`: thêm vào `READ_TOOLS`; cập nhật danh sách tool trong test MCP
- [x] Test MCP cũ xanh; ruff không thêm lỗi

Bằng chứng: `tests/test_mcp_kg_elements.py` 4 ca xanh (theo mã + `missing`, theo TDD/lớp, KG một TDD → `whole_kg`, phân trang `next`); `test_mcp_server.py`, `test_mcp_auth.py` xanh (80 passed); ruff không thêm lỗi. `info` chỉ gồm id, seq, hash, created_at, graph_version (không kèm `touched`).

### T7.2 Client MCP đọc KG cho TS1

Phụ thuộc: T7.1

- [x] Viết test: đọc đồ thị con theo change spec và theo TDD qua client MCP trong bộ nhớ; kết quả chiếu giống đọc thẳng snapshot
- [x] `agents/testspec/kgclient.py`: mở client (stdio: `tdd-agent -C <dự án> mcp serve`; http: URL + bearer token), gom trang `next`, chạy trong luồng riêng
- [x] Đọc theo lớp: phần tử gốc → láng giềng tới `context_depth` (chỉ đi qua lớp chiếu được) → cạnh chạm mọi phần tử đã lấy
- [x] Cấu hình `[testspec] kg_mcp` (rỗng = stdio cục bộ), `kg_mcp_token_env`; lỗi kết nối → `blocked: TESTSPEC_KG_MCP`
- [x] Ruff sạch

Bằng chứng: `tests/test_testspec_kgclient.py` 5 ca xanh: đồ thị con đọc qua MCP (client trong bộ nhớ tới server thật) cho gói giống từng byte với đọc cả snapshot (theo change spec và theo TDD); snapshot không có → `SNAPSHOT_NOT_FOUND`; lỗi kết nối → `TESTSPEC_KG_MCP`; HTTP thiếu token → `TESTSPEC_KG_MCP_TOKEN`; **stdio thật**: dự án SQLite tạm, client chạy `tdd-agent -C <dự án> mcp serve --transport stdio`. Cấu hình `kg_mcp`, `kg_mcp_token_env` (kiểm URL).

### T7.3 TS1, GT dùng MCP

Phụ thuộc: T7.2

- [x] TS1 không gọi `ctx.knowledge` nữa; đọc snapshot, change spec, phạm vi qua MCP
- [x] GT không đọc KG: chỉ truyền mã snapshot, mã dòng change spec, tiêu chí CR (từ đầu ra G0)
- [x] `tdds`, văn bản tiêu chí từ `verification` chuyển sang TS1 (đọc qua MCP)
- [x] Test luồng (`test_testspec_flow.py`) chạy qua client MCP trong bộ nhớ, xanh
- [x] Một test stdio thật: dự án SQLite tạm, TS1 mở `tdd-agent mcp serve` qua stdio

Bằng chứng: `tests/test_testspec_flow.py` 12 ca xanh qua MCP server trong bộ nhớ; TS1 báo `kg.mcp_calls`; GT chỉ chuyển `snapshot`, `cs_elements`, tiêu chí G0 (test kiểm đúng các khóa); `grep knowledge src/tdd_agent/agents/testspec` không còn chỗ dùng kho KG. Stdio thật kiểm ở T7.2.

### T7.4 Tài liệu

Phụ thuộc: T7.3

- [x] TDD 0.6 (ADR, luồng, mục 5.3), SOLUTION 0.3 mục 13
- [x] `docs/CLI_Project_Usage.md`, `docs/tsagent/README.md`: TS1 đọc KG qua MCP, cấu hình `kg_mcp`
- [x] Hồi quy toàn repo xanh

Bằng chứng: TDD 0.6 (INT-ADR-11, INT-MECH-07, INT-CON-03, mục 5.3, luồng 8.1, cấu hình), SOLUTION 0.3 mục 13 (SOL-18..23), `CLI_Project_Usage.md`, `docs/tsagent/README.md`, mẫu `init` ghi `kg_mcp`. Hồi quy toàn repo: 692 passed, 2 skipped, 0 fail; ruff `tdd_agent` 226 (= HEAD), ruff tsagent sạch.


---

## P8. Ngữ cảnh Plan / Solution (đề xuất TDD 0.7, SOLUTION 0.4 mục 14)

> Đã làm (2026-10-19) sau khi chủ dự án duyệt TDD 0.7 và SOLUTION mục 14, trừ phần ghi rõ bên dưới.

### T8.1 Cấu hình `context_mode`, `context_path` (§14.2)

Phụ thuộc: T7.4 · Ca: TC-51

- [x] Viết test: mặc định `none` và rỗng; `context_mode` lạ, `context_path` tuyệt đối hay có `..` → `CONFIG_INVALID`; biến `TDD_AGENT__TESTSPEC__CONTEXT_MODE`
- [x] Sửa test mặc định có sẵn của `[testspec]` (so khớp cả hai khóa mới)
- [x] `config.py`: `DEFAULTS["testspec"]`, hằng `TESTSPEC_CONTEXT_MODES`, kiểm giá trị
- [x] Hai khóa **không** vào `fingerprint()` (kiểm lại test TC-11)
- [x] Test cấu hình cũ xanh; ruff sạch

Bằng chứng: `tests/test_testspec_config.py` (mặc định, giá trị sai, biến môi trường), TC-11 vẫn xanh (hai khóa không vào `fingerprint()`).

### T8.2 Đọc Solution (§14.3, §14.4)

Phụ thuộc: T8.1 · Ca: TC-52, TC-53, TC-54

- [x] Viết test: dạng bản ghi Orca và dạng tài liệu; chỉ phương án đã chọn; fallback khuyến nghị kèm `notes`; **`status = draft` vẫn chạy** (chỉ ghi `notes`); không có `options` → `TESTSPEC_CONTEXT_INVALID`
- [x] `agents/testspec/context.py`: `Item`, `Bundle`, `parse_solution`
- [x] Tách giả định theo `needs_confirmation`, câu hỏi theo `blocking`, rủi ro của phương án chọn, tiêu chí + độ phủ
- [x] So `design_references.snapshot` (cảnh báo khi lệch ở TS1, §14.10)
- [x] Test xanh; ruff sạch

Bằng chứng: `tests/test_testspec_context.py` (TC-52, 53, 54: bản ghi Orca và tài liệu, phương án đã chọn / khuyến nghị, `status = draft` vẫn chạy, `snapshot` ghi lại).

### T8.3 Đọc Plan (§14.3, §14.4)

Phụ thuộc: T8.1 · Ca: TC-55, TC-56

- [x] Viết test: `tasks.md` hai task và một scenario bốn bước; task thiếu Done when; thiếu `proposal.md`/`design.md` chỉ cảnh báo; Plan rỗng → `TESTSPEC_CONTEXT_INVALID`
- [x] Viết test TC-75: `proposal.md` và `design.md` của OpenSpec change Request #2 → `Decision` proposal, 4 `Decision` thiết kế, `Risk` và `Assumption` (mục chứa `assum…`), `Constraint` (Non-Goals), Context ≤ 600 ký tự
- [x] `context.py`: `parse_plan` (ngữ pháp `T<phase>.<n>`, thuộc tính `[k=v]`, bốn dòng `Serves/Code/Verify/Done when`, `specs/**/spec.md`), `parse_proposal`, `parse_design` (theo tên mục, không phân biệt hoa thường; mục lạ vào `notes`)
- [x] `Code:` thành `codeTargets`, `Verify:` thành `verifyScenario` + `verifyCommand`
- [x] Test xanh; ruff sạch

Bằng chứng: `tests/test_testspec_context.py` (TC-55, 56, 75, 76: `tasks.md`, scenario, `proposal.md`, `design.md`, mục lạ và tệp thiếu vào `notes`).

### T8.4 Nạp tài liệu và kiểm đường dẫn (§14.2, §14.9)

Phụ thuộc: T8.2, T8.3 · Ca: TC-57

- [x] Viết test: không đặt đường dẫn; không tồn tại; ra ngoài repo bằng `..` và symlink; `params.context_path` ghi đè, `params.context_mode` bị bỏ qua
- [x] `context.py`: `resolve_source`, `fingerprint`, `load`
- [x] Mã lỗi `TESTSPEC_CONTEXT_MISSING`, `TESTSPEC_CONTEXT_PATH`, `TESTSPEC_CONTEXT_INVALID`
- [x] Test xanh; ruff sạch

Bằng chứng: `tests/test_testspec_context.py` (TC-57: đường dẫn thiếu, symlink ra ngoài, ghi đè `params`, tài liệu gửi qua MCP: tên tệp, dung lượng, ghi nguyên tử, không thay tài liệu tốt bằng tài liệu sai).

### T8.5 Chiếu thành nút (§14.5, §14.6)

Phụ thuộc: T8.4 · Ca: TC-58, TC-59, TC-60, TC-61, TC-62, TC-63

- [x] Viết test: gói Solution qua I1–I8; cạnh đúng miền/đích; giả định chưa xác nhận và câu hỏi chặn thành TestGap, giả định đã chốt thì không; Plan dập TestGap `no-ac`
- [x] Viết test: mục không gắn được vào `context.unanchored`; `none` cho gói giống từng byte; chiếu hai lần giống từng byte
- [x] `project.py`: `Projector.build(..., context=...)`, `_inject_context`, quy tắc gắn yêu cầu
- [x] Báo cáo chiếu có khối `context` (số nút theo loại, yêu cầu được gắn, `unanchored`, `notes`)
- [x] Test xanh; ruff sạch

Bằng chứng: `tests/test_testspec_context.py` (TC-58 đến 63: gói qua I1–I8, cạnh đúng miền/đích, TestGap cho giả định chưa xác nhận, `no-ac` biến mất, `unanchored`, `none` giống từng byte).

### T8.6 Gate, băm, hiển thị (§14.8, §14.10)

Phụ thuộc: T8.5 · Ca: TC-64

- [x] Viết test: đổi nội dung tài liệu → băm GT/TS1 đổi, GT chạy lại, `origin.context.sha256` mới; tài liệu giữ nguyên → `deduplicated`
- [x] `gate.py`: `ctxdoc.load` sau kiểm LLM và trước khi đọc KG; `extra_inputs` thêm `context`; `origin.context`; tóm tắt `context_mode`
- [x] `runner.py`: dòng của feature có `context`
- [x] `project.py` (`change status`) nêu chế độ; mẫu `init` ghi sẵn hai khóa dạng chú thích
- [x] Test xanh; ruff sạch

Bằng chứng: `tests/test_testspec_context_flow.py` 9 ca (TC-64, mã lỗi trước mọi lời gọi LLM, `origin.context`, dòng `change status`, snapshot lệch chỉ cảnh báo, **TC-77**: đổi qua lại giữa các đầu vào ghi lại Draft; test này đỏ khi bỏ bản sửa trong `runner.py`).

### T8.7 E2E và đo trên repo `notes` (§11.4)

Phụ thuộc: T8.6 · Ca: TC-65, TC-66

- [x] E2E `llm = "none"`: `run start testspec` với Solution của Request #2 → TestGap có A-2, không có A-1, A-3
- [~] Đo `claude-cli`: **đã đo `solution`** (7 TR, 16 TC, 9 TestGap, 94 s); chưa đo `plan` bằng `claude-cli` (mốc không ngữ cảnh: 6 TR, 10 TC, 8 TestGap, 15 lượt). Số liệu ở PERF.md
- [~] So gap "định dạng phản hồi", "xác thực" của `FR-NOTE-03` trước và sau; ghi kết luận vào TDD 0.7 mục 11.3
- [ ] Đánh dấu `llm_real`, không chạy trong `pytest` mặc định

Bằng chứng: PERF.md mục TC-66 (rules only: không ngữ cảnh 3/3/2, `solution` 3/4/3, `plan` 3/9/4; `claude-cli` + `solution` 7/16/9).

### T8.8 Tài liệu (§14)

Phụ thuộc: T8.6

- [x] `docs/CLI_Project_Usage.md`: bỏ nhãn "đề xuất", ghi đúng hành vi đã làm
- [x] SOLUTION (nhãn "đề xuất" thành "đã hiện thực", lệch phát sinh ở 12.3), TDD 0.7 trạng thái
- [ ] `docs/tsagent/README.md`: dòng trỏ tới mục 14 (chưa làm)
- [ ] Phía Orca: tệp `docs/integrate` ghi thêm cách ghi Solution/Plan vào `.tdd-agent/context/` và `run_start` có `context_path` (§14.11)
- [x] Hồi quy toàn repo xanh

### T8.9 Tool MCP `testspec_start`, `testspec_get` (§14.13)

Phụ thuộc: T8.6 · Ca: TC-67, TC-68, TC-69, TC-70, TC-71, TC-72, TC-73, TC-74

- [x] Viết test (client MCP trong bộ nhớ tới server thật): `testspec_start` Solution `document` và Plan `files`; sai `kind` → `TESTSPEC_CONTEXT_MODE_MISMATCH`; LLM chưa đặt và `enabled = false` → lỗi, không tạo run; gọi lại cùng `external` → `reused`
- [x] Viết test: tên tệp lạ, `..`, tuyệt đối, tệp 300 KiB, 41 tệp → từ chối, thư mục `.tdd-agent/context/` không đổi; `testspec_get` ba dạng và phân trang; không lộ khóa
- [x] `context.py`: `write_inline` (danh sách tên cho phép, dung lượng, ghi nguyên tử, thay thư mục cũ của cùng `ref`)
- [x] `mcp/tools.py`: `testspec_start` (thêm vào `WRITE_TOOLS`) theo thứ tự kiểm của §14.13, `testspec_get` đọc `tsagent.db` chỉ đọc; `mcp/server.py`: `READ_TOOLS`, mô tả, lược đồ tham số
- [x] Cập nhật test danh sách tool MCP có sẵn; test MCP cũ xanh; ruff sạch
- [x] Một test stdio thật (TC-74): chuỗi `testspec_get` → `testspec_start` → `run_dispatch` → `run_get` → `testspec_get`

Bằng chứng: `tests/test_mcp_testspec.py` 21 ca (TC-67 đến 74, gồm chuỗi Orca qua server MCP); danh sách tool trong `test_mcp_support.py`; chạy thật qua **stdio**: `tdd-agent -C <repo notes> mcp serve` → `testspec_get`, `testspec_start`, `run_dispatch`, `run_get`, `testspec_get` (PERF.md, TC-66). Hồi quy toàn repo xanh (trừ `test_reads_over_a_real_stdio_server` cần `tdd_agent` cài trong môi trường chạy test; môi trường này không cài).

### T8.10 Tích hợp phía Orca (ngoài repo `tdd-agentic-ai`)

Phụ thuộc: T8.9

- [x] Thêm `testspec_start`, `testspec_get` vào allowlist cầu MCP (`agent/src/relay/agent-rpc-dispatch-mcp.ts`, `deploy/agent/README.md`, `deploy/agent/agent-runtime.env.example`; test `agent-rpc-dispatch-mcp.test.ts` xanh). Còn: bật `[mcp] write = true` cho dự án dùng
- [ ] `request-service`: bước hoặc nút "Test specs" của Request: `testspec_get()` → `testspec_start` với Solution (bản ghi + `acceptance_criteria` + `request.ref`) hoặc các tệp OpenSpec change qua `plan/bundle/file` → `run_dispatch` → theo dõi `run_get`
- [ ] Hiển thị kết quả (tab Test specs, TestGap); ghi `external` để tìm lại run
- [ ] Skill `tdd-openspec-author`: trích mã `TC-…` vào dòng `Verify:` khi có test specs (chế độ `solution`)
- [ ] Ghi vào `docs/integrate` của Orca (task riêng, ngoài tài liệu này)

Còn lại (chưa làm): bước/nút "Test specs" trong `request-service`, tab hiển thị, skill, tài liệu `docs/integrate`.
