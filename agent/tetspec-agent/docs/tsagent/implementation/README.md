# Xây mới tsagent v2: theo dõi tiến độ

> **Vị trí hiện tại (2026-10-13):** tsagent đã chuyển sang repo `tdd-agentic-ai` (nhánh `tasks/integrate-tetspec-agent`). Đường dẫn trong các tệp giai đoạn ghi theo vị trí cũ `backend/agents/testspec-agent/` của repo vnp-workplace; ánh xạ sang vị trí mới:
>
> | Cũ (vnp-workplace) | Mới (tdd-agentic-ai) |
> | --- | --- |
> | `backend/agents/testspec-agent/tsagent/` | `src/tsagent/` |
> | `backend/agents/testspec-agent/tests/` (`from tests.…`) | `tests/testspec_agent/` (`from testspec_agent.…`) |
> | `backend/agents/testspec-agent/scripts/` | `scripts/tsagent/` |
> | `backend/agents/testspec-agent/deploy/`, `Makefile` | `deploy/tsagent/`, lệnh docker trong [../README.md](../README.md) |
> | `make test`, `make lint` | `.venv/bin/pytest tests/testspec_agent`, `.venv/bin/ruff check --config ruff-tsagent.toml …` |
> | `docs/crs/v17/testspec-agent/v2/` | `docs/tsagent/` |
>
> Plugin Turtle (T3.5, FR-017) đã gỡ khi chuyển (D21): tsagent không dùng RDF.

Kế hoạch: [PLAN.md](./PLAN.md). Thiết kế: [TDD-TSAGENT-v2.md](../TDD-TSAGENT-v2.md), [ONTOLOGY-TEST-TSO.md](../ONTOLOGY-TEST-TSO.md), [ONTOLOGY-TDD.md](../ONTOLOGY-TDD.md).

Code v1 bị gỡ hoàn toàn ở giai đoạn 0 (sau khi đo mốc và lưu trữ); v2 viết lại từ đầu, không tái dùng module v1.

## Cách dùng

1. Chọn task chưa bắt đầu mà các task phụ thuộc đã xong (cột "Phụ thuộc", hoặc PLAN mục 5).
2. Mở file giai đoạn, đổi `Trạng thái` của task thành `Đang làm`.
3. Làm từng mục; xong mục nào đổi `- [ ]` thành `- [x]` ngay trong file giai đoạn.
4. Khi mọi mục của task là `[x]` và đạt Definition of Done (PLAN mục 8): đổi trạng thái thành `Xong (ngày, commit)` và đánh dấu ô của task trong bảng dưới.
5. Xong mọi task của một giai đoạn thì đánh dấu giai đoạn đó.
6. Lệch so với TDD v2 ghi vào [DEVIATIONS.md](./DEVIATIONS.md).

## Tiến độ theo giai đoạn

- [x] **Giai đoạn 0**: Đo mốc, lưu trữ, xóa v1, khung mới ([PHASE-0-reset.md](./PHASE-0-reset.md))
- [x] **Giai đoạn 1**: Nền tảng ([PHASE-1-foundation.md](./PHASE-1-foundation.md))
- [x] **Giai đoạn 2**: Kho SQLite ([PHASE-2-store.md](./PHASE-2-store.md))
- [x] **Giai đoạn 3**: Nạp đầu vào, delta ([PHASE-3-ingest.md](./PHASE-3-ingest.md))
- [x] **Giai đoạn 4**: Client LLM ([PHASE-4-llm.md](./PHASE-4-llm.md))
- [x] **Giai đoạn 5**: Tầng A ([PHASE-5-analyze.md](./PHASE-5-analyze.md))
- [x] **Giai đoạn 6**: Tầng B, C, render ([PHASE-6-design-specify-render.md](./PHASE-6-design-specify-render.md))
- [x] **Giai đoạn 7**: Orchestrator, MCP, CLI, đóng gói, workflow ([PHASE-7-orchestrator-interfaces.md](./PHASE-7-orchestrator-interfaces.md))
- [ ] **Giai đoạn 8**: Profile Extended ([PHASE-8-extended.md](./PHASE-8-extended.md))
- [x] **Giai đoạn 9**: Hoàn thiện ([PHASE-9-hardening.md](./PHASE-9-hardening.md))

## Tiến độ theo task

| Xong | Task | Tên | Phụ thuộc | Test hoặc đầu ra chính |
| --- | --- | --- | --- | --- |
| [x] | T0.1 | Đo mốc v1, xuất fixture | — | BASELINE-V1.md |
| [x] | T0.2 | Lưu trữ v1 | T0.1 | ARCHIVE-V1.md, khôi phục được |
| [x] | T0.3 | Xóa v1, dựng khung dự án | T0.2 | `make init`, `make test`, `make lint` |
| [x] | T0.4 | Hạ tầng test | T0.3 | `pytest -m unit` |
| [x] | T1.1 | Cấu hình, lỗi, log | T0.4 | test_config, test_logging |
| [x] | T1.2 | Kiểu miền | T1.1 | test_model |
| [x] | T1.3 | `tso.yaml` | T1.1 | test_ontology_yaml |
| [x] | T1.4 | Codegen | T1.3 | test_codegen |
| [x] | T1.5 | Kết nối DB, migrate, giao dịch | T1.4 | test_db |
| [x] | T2.1 | API nguồn | T1.5, T1.2 | test_store_source |
| [x] | T2.2 | API KG test, vùng đệm, phiên bản | T1.5, T1.2 | test_store_versions |
| [x] | T2.3 | Vòng đời, duyệt, baseline | T2.2 | test_lifecycle (TST-13) |
| [x] | T2.4 | Toàn vẹn, snapshot, xuất | T2.2 | test_store_ops (TST-16, 18) |
| [x] | T3.1 | Bộ nạp `tdd-kg/v1` | T2.1 | test_kg_json (TST-01, 02, 03) |
| [x] | T3.2 | Bộ phân tích md mới | T1.2 | test_md_parser |
| [x] | T3.3 | Gộp nguồn, tạo baseline | T3.1, T3.2 | test_ingest |
| [x] | T3.4 | Delta, tập ảnh hưởng | T3.3 | test_delta (TST-04) |
| [x] | T3.5 | Plugin Turtle (tùy chọn) (bỏ 2026-10-13, D21) | T3.1 | TST-19 |
| [x] | T4.1 | Nhà cung cấp HTTP | T1.1 | test_llm_http |
| [x] | T4.2 | Nhà cung cấp `claude-cli` | T1.1 | test_llm_claude_cli |
| [x] | T4.3 | `complete_json`, cache, trần | T4.1, T4.2, T1.4 | test_llm_client |
| [x] | T4.4 | LLM giả cho test | T4.3 | fixture `fake_llm` |
| [x] | T5.1 | Mã nút | T1.2 | test_codes |
| [x] | T5.2 | Luật seed Core | T3.3, T5.1 | test_seed_core (TST-05) |
| [x] | T5.3 | Gói ngữ cảnh | T5.2 | test_context |
| [x] | T5.4 | LLM điền tầng A | T5.3, T4.3 | test_analyze (TST-06, 07) |
| [x] | T5.5 | Luật TSO-001..007 | T5.2 | test_tso_rules_a |
| [x] | T5.6 | So sánh tầng A với mốc v1 | T5.4, T5.5, T0.1 | COMPARE-A.md |
| [x] | T6.1 | Luật thiết kế | T5.4 | test_design_rules |
| [x] | T6.2 | LLM tầng B, TSO-008..010, 016 | T6.1, T4.3 | test_design (TST-08) |
| [x] | T6.3 | Tầng C | T6.2 | test_specify (TST-09) |
| [x] | T6.4 | Render | T6.3, T2.2 | test_render (TST-11) |
| [x] | T6.5 | So sánh với mốc v1 | T6.4, T5.6 | COMPARE-BC.md, TST-21 |
| [x] | T7.1 | Orchestrator theo bước | T6.4, T3.4 | test_orchestrator (TST-12, 15) |
| [x] | T7.2 | Duyệt và baseline | T2.3, T6.3 | test_review (TST-14) |
| [x] | T7.3 | MCP gateway | T7.1 | test_mcp (TST-17) |
| [x] | T7.4 | CLI | T7.1, T2.4 | test_cli |
| [x] | T7.5 | Đóng gói, triển khai | T7.3, T7.4 | `make start-dev` |
| [x] | T7.6 | Tích hợp workflow minipay | T7.3, T7.5 | workflow mock chạy hết |
| [x] | T8.1 | Cấu hình profile | T5.2 | test_profiles |
| [ ] | T8.2 | Fixture KG module 04 | T3.1 | fixture được duyệt |
| [x] | T8.3 | Luật seed Extended | T8.1, T8.2 | test_seed_ext (TST-05) |
| [x] | T9.1 | Kiểm thử đầu-cuối với workflow | T7.6 | COMPARE-E2E.md |
| [x] | T9.2 | Hiệu năng, vận hành | T7.5 | PERF.md (TST-16, 20) |
| [x] | T9.3 | Rà bảo mật | T7.3 | SECURITY.md |
| [x] | T9.4 | Tài liệu | T9.1 | README, TDD baselined |

## Truy vết yêu cầu → task

| Yêu cầu TDD v2 | Task |
| --- | --- |
| FR-001, FR-002 | T3.1, T3.2, T3.3 |
| FR-003 | T3.4 |
| FR-004 | T5.2, T8.3 |
| FR-005 | T5.3, T5.4 |
| FR-006 | T6.1, T6.2 |
| FR-007 | T6.3 |
| FR-008 | T5.5, T6.2, T6.3, T7.2 |
| FR-009 | T6.4 |
| FR-010 | T2.2, T7.1 |
| FR-011 | T2.3, T7.2 |
| FR-012 | T1.5, T7.1 |
| FR-013 | T1.4, T1.5, T2.4 |
| FR-014 | T7.3, T7.4 |
| FR-015 | T4.3 |
| FR-016 | T2.4 |
| FR-017 | T3.5 (bỏ, D21) |
| NFR-001 | T4.3, T5.3, T6.4 |
| NFR-002, NFR-003 | T5.5, T6.2, T6.3 |
| NFR-004 | T1.4, T2.3 |
| NFR-005, NFR-006, NFR-009 | T9.2 |
| NFR-007 | T5.4 |
| NFR-008 | T6.3, T7.3, T9.3 |
| NFR-010, NFR-011 | T1.5, T7.1 |
| NFR-012 | T3.1, T3.2 |
| NFR-013 | T7.2 |

## Tài liệu sinh ra khi triển khai

| Tài liệu | Task |
| --- | --- |
| BASELINE-V1.md, fixtures-v1/ | T0.1 |
| ARCHIVE-V1.md | T0.2 |
| COMPARE-A.md | T5.6 |
| COMPARE-BC.md | T6.5 |
| COMPARE-E2E.md | T9.1 |
| PERF.md | T9.2 |
| SECURITY.md | T9.3 |
| DEVIATIONS.md | Mọi task (khi lệch TDD) |

## Ngoài phạm vi (ghi ý mới, không làm trong kế hoạch này)

- (trống)
