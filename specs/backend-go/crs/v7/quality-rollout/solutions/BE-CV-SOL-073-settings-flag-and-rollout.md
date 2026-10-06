# BE-CV-SOL-073: Cờ `code_intel_enabled` theo tenant, e2e phía backend, kiểm tra đăng ký, rollout và tài liệu vận hành

> 📋 Proposed. Chưa triển khai, chưa chạy. Viết ngày 2026-10-06 từ CR-CV-073, ba hợp đồng v7 và code hiện có; `code-intel-service` chưa tồn tại.

**CR:** [CR-CV-073](../../../../../../docs/crs/v7/quality-rollout/CR-CV-073-e2e-feature-flag-rollout-runbook.md)
**Service:** `code-intel-service` (mới: `codeintel_settings.proto`, use case cờ, `feature_gate.go`, `e2e/`), `backend-go/ci/` (script mới), `.github/workflows/`, `tests/code-intel/` (mới, Python), `docs/guides/code-intel/` (mới)
**TDD tham chiếu:** [`arch/10`](../../../../tdd/architecture/10-deployment-infrastructure.md) (môi trường, CI/CD, rollout từng service), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (health, alert, runbook), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, migration), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (cổng, test theo lớp), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (gRPC conventions), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md)
**Task:** [`../tasks/README.md`](../tasks/README.md)

---

## 1. Hợp đồng áp dụng

| Nguồn | Mục / PQ | Dùng để |
|---|---|---|
| `CONTRACT-codeintel-proto-and-data-map.md` | **PQ-01** (`CODEINTEL_DISABLED`; `FEATURE_DISABLED` bị bỏ; `QUALITY_GATE_DISABLED`, `AI_REVIEW_DISABLED`; cờ quét bảo mật ⇒ `PROFILE_UNKNOWN`), **PQ-23** (tên biến, `migrate-codeintel`, `orca-go-code-intel`, DB `codeintel`), **PQ-24** (cache 5 s, ngoại lệ `GetSettings`/`SetSettings`/`GetReindexJob`, đủ cột từ migration `0002`, fail closed), §2.1 (`codeintel_settings.proto`), §3.1 (`GetSettings`, `SetSettings`), §3 đầu (thứ tự guard → tenant → cờ → OPA…), §4.2 T1 (`tenant_settings`), §6.1–6.3, §7 (đợt), §8.2–8.3, §10 (danh sách file wiring), O-17 | cờ, RPC, wiring |
| `CONTRACT-codeintel-ui-api.md` | §2.3 (`CODEINTEL_DISABLED` ⇒ client `disabled`), §4.1 `Settings`, §3.1 `settings.get|set`, §6 (frontend không mở `subscribe` khi tắt), U7, U8 | e2e liên quan UI/gateway |
| `CONTRACT-codeintel-agent-rpc.md` | §2.4 (`ORCA_CODEINTEL_DISABLED=1`), §3.2 mã lỗi, §4.10 reindex (`--index-only`), §9 | kịch bản E05, kill-switch |

## 2. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `backend-go/go.work` (20 module, `go 1.26.0`; không có `code-intel-service`), `backend-go/Makefile` (`SERVICES`), `backend-go/deploy/postgres-init-databases.sh` (`DATABASES`, kết thúc `… credential mcp`), `deploy/dev/docker/postgres/init-databases.sh` (dòng 17: `DATABASES` ngắn, có `issuestatussync`; **hai tệp init khác nhau**, compose dev gắn tệp ở `deploy/dev/docker/postgres/`, `deploy/dev/docker-compose.yml:135`; `backend-go/docker-compose.yml:23` gắn tệp `backend-go/deploy/`), `deploy/dev/scripts/migrate.sh` dòng 27 (`SERVICES="auth tenant … mcp issuestatussync"`), `deploy/dev/docker-compose.yml` (`mcp-service` dòng 408, `MCP_ENABLED: ${MCP_ENABLED:-false}` dòng 576, `migrate-mcp` dòng 741, `MCP_SERVICE_ADDR` dòng 71, tên container `orca-go-*`), `backend-go/ci/check-opa-bundle-in-images.sh` (mảng `svcs`), `backend-go/services/mcp-service/internal/domain/tenant_settings.go` (`TenantSettings`, `DefaultTenantSettings`), `mcp-service/README.md` dòng 35, 73 (`MCP_TENANT_DEFAULT_ENABLED`, mặc định `true`), `backend-go/common/internalcaller`, `tests/mcp/` (`check_mcp_*.py`), `tests/backend/` (`check_framework.py`, `orca_api_session.py`, `api_websocket_channels.py`), `tests/playwright.web.config.ts` (`testDir: './e2e/mcp-web'`, `testMatch: '**/*.web.e2e.ts'`, Vite cổng 5174), `tests/e2e/mcp-web/support/*`, `package.json` script `test:e2e:mcp-web`.

| # | Correction relative to CR-CV-073 | Bằng chứng |
|---|---|---|
| C1 | Có **hai** tệp `init-databases.sh` (backend-go và deploy/dev) với danh sách khác nhau; CR chỉ nhắc `backend-go/deploy/postgres-init-databases.sh`. Script kiểm đăng ký phải kiểm cả hai (hợp đồng §10 đã liệt kê cả hai) | `ls`, `grep DATABASES` |
| C2 | Tên DB ở `migrate.sh` dùng tên ngắn (`issuestatussync`), ở `postgres-init-databases.sh` dùng tên đầy đủ; với code-intel hợp đồng chốt `codeintel` cho cả hai nên một tên | PQ-23 |
| C3 | Tên job migrate: CR viết `migrate-code-intel`; hợp đồng PQ-23: `migrate-codeintel`; biến: `CODEINTEL_ENABLED` (không `CODE_INTEL_ENABLED`) | PQ-23 |
| C4 | Cờ theo tenant mẫu `mcp-service`: dòng lười, `MCP_TENANT_DEFAULT_ENABLED` quyết định dòng **mới**; code-intel đổi: `CODEINTEL_TENANT_DEFAULT_ENABLED` mặc định `false` | `tenant_settings.go`, README |
| C5 | Không có hệ thống feature-flag chung trong repo (đúng); khung e2e MCP dùng WS giả dialect session-client | đọc `tests/` |
| C6 | `config/vitest.config.ts` không tồn tại ở gốc; test web của repo chạy qua `package.json` `test:e2e:mcp-web` | `ls config` |
| C7 | CI backend 18 workflow `backend-go-*`; Go CI 1.25 so với `go.work` 1.26 | `ls .github/workflows` |

## 3. Lệch giữa CR và hợp đồng

| # | CR-CV-073 nói | Hợp đồng nói (thắng) | Hệ quả |
|---|---|---|---|
| L1 | Bảng `codeintel.tenant_settings(tenant_id, code_intel_enabled, updated_by, updated_at)` là bảng **mới do CR-073 tạo** | T1: bảng do **CR-011** tạo ở migration `0002_code_intel_core` với **đủ cột** (`quality_gate_enabled`, `quality_security_scan_enabled`, `index_policy`, `ai_review_level`, `ai_review_model`, `agent_turn_store_prompt_excerpt`, `agent_claim_text_enabled`, `hotspot_window_days`…), không `ALTER` sau | solution này **không** tạo migration; chỉ dùng bảng và cung cấp use case/RPC/proto |
| L2 | Biến `CODE_INTEL_ENABLED`; job `migrate-code-intel` | `CODEINTEL_ENABLED`, `CODEINTEL_QUALITY_GATE_ENABLED`, `CODEINTEL_AI_REVIEW_ENABLED`, `CODEINTEL_TENANT_DEFAULT_ENABLED`, `CODEINTEL_TENANT_DEFAULT_QUALITY_GATE_ENABLED`; `migrate-codeintel` | dùng tên của hợp đồng |
| L3 | Bộ đệm "≤ 5 s (đề xuất)" | PQ-24: **5 s**, chốt | không còn "đề xuất" |
| L4 | Ngoại lệ khi tắt: chỉ `GetSettings` | PQ-24: `GetSettings`, `SetSettings` (admin), `GetReindexJob`; ui-api U7: `settings.get|set` | thêm hai RPC ngoại lệ; `SetSettings` vẫn cần admin |
| L5 | `GetSettings` trả `enabled` | §3.1: `{effective, tenant}`; ui-api §4.1 `Settings{effective{codeIntelEnabled, qualityGateEnabled, qualitySecurityScanEnabled, aiReviewEnabled}, tenant{…9 trường}, updatedBy, updatedAt}`; `SetSettings` nhận ≥ 1 trường | proto theo hợp đồng; validate `hotspot_window_days` 30..365, `index_policy`, `ai_review_level`, allowlist `ai_review_model` |
| L6 | Mã lỗi `CODEINTEL_DISABLED` cho mọi RPC | PQ-01: thêm cờ chất lượng ⇒ `CODEINTEL_QUALITY_GATE_DISABLED` (mọi RPC `QualityGateService`); AI ⇒ `CODEINTEL_AI_REVIEW_DISABLED`; quét bảo mật ⇒ `CODEINTEL_PROFILE_UNKNOWN` | bảng phân loại cổng có ba tầng; e2e E12 mở rộng |
| L7 | Kênh `settings.get|set` do CR-040 đăng ký | đúng; phiên thiết bị: `settings.get` được phép, mọi kênh khác bị từ chối (U8) | không viết kênh ở đây, chỉ test e2e qua fake |
| L8 | Ngưỡng/giai đoạn rollout (3 ngày, 2 tuần…) | O-15: mọi con số là giả định | giữ làm cấu hình rollout, ghi giả định |
| L9 | Nhắc 25 RPC ở README 3.6 cho `TestEveryRPCHasScenario` | §3.1 + §3.2 = **49** RPC | ma trận kịch bản đọc từ `ServiceDesc`; RPC `QualityGateService` có kịch bản thuộc CR-082/085 hoặc được `waive` có lý do |
| L10 | `RequestReindex` không huỷ; "chạy tới hết" | O-17: `CancelReindex` chưa có RPC/kênh; agent có `reindexCancel` | giữ "chạy tới hết", ghi vào runbook |
| L11 | Tiêu chí quay lui "migration `down` từ chối nếu có dữ liệu" | thuộc migration của CR-011 | ghi phụ thuộc; e2e kiểm khi migration có |

## 4. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| FE | `FE-CV-SOL-073-flag-gating-and-web-e2e` | ẩn lối vào theo `settings.get`, không mở `subscribe` khi tắt; Playwright T2 + `code-intel-fake-backend.ts` dựng từ tệp vàng của `BE-CV-SOL-070` |
| AG | `AG-CV-SOL-073-agent-kill-switch` | `ORCA_CODEINTEL_DISABLED=1`; BE ghi vào runbook |
| AG | `AG-CV-SOL-070`, `004`, `081` | repo mẫu, `--index-only`, `git status` không đổi (E05, T3) |
| BE | `BE-CV-SOL-010` (khung, wiring, `apperrors`), `011-*` (bảng T1, repo), `013-authorization-flags-and-audit` (OPA, audit), `013-agent-call-gate-and-quotas`, `021`, `023`, `024`, `040-codeintel-channel-foundation` (`CODE_INTEL_SERVICE_ADDR` rỗng ⇒ `CODEINTEL_UNAVAILABLE`), `050` (FE) | đối tượng e2e; **trùng phạm vi** với 013 về đọc cờ: xem Q1 |
| BE | `BE-CV-SOL-070`, `071`, `072` | e2e dùng tệp vàng; điều kiện chuyển giai đoạn dùng alert 071 và test 072 |

## 5. Giải pháp

### 5.1 Cờ (task 01, 02, 03)

- Proto `codeintel_settings.proto` (§2.1 mục 8: `TenantSettings`, `GetSettings*`, `SetSettings*`) và hai RPC thêm vào `service CodeIntelService` ở `codeintel.proto` (task 01). Domain `TenantSettings` + `Validate` (mẫu `mcp-service/.../tenant_settings.go`), cổng `TenantSettingsRepository` (hiện thực ở BE-CV-SOL-011), use case `GetSettings`, `SetSettings` (chỉ `role=admin` từ metadata `x-orca-role`; ghi audit `codeintel.settings.set` qua cổng audit; đổi `ai_review_level=diff` cần xác nhận hai bước ở UI nhưng server chỉ kiểm giá trị).
- `EffectiveFlags` (task 02): `code_intel_enabled_effective = CODEINTEL_ENABLED ∧ tenant.code_intel_enabled`; chất lượng `∧ CODEINTEL_QUALITY_GATE_ENABLED ∧ tenant.quality_gate_enabled ∧ code_intel`; AI `∧ ai_review_level≠off ∧ CODEINTEL_AI_REVIEW_ENABLED`; quét bảo mật `∧ quality_gate ∧ quality_security_scan_enabled`. Thiếu dòng ⇒ chèn dòng lười với `code_intel_enabled = CODEINTEL_TENANT_DEFAULT_ENABLED` (mặc định `false`; không đổi dòng cũ). Cache ≤ 5 s theo `tenant_id`, lỗi đọc ⇒ tắt (fail closed), cache **không** lưu lỗi.
- `feature_gate.go` (task 03): interceptor unary + stream đặt **sau** guard nội bộ và trích tenant, **trước** OPA (§3 đầu). Bảng phân loại mọi RPC: `allowedWhenDisabled` = {`GetSettings`, `SetSettings`, `GetReindexJob`}; `QualityGateService` ⇒ `CODEINTEL_QUALITY_GATE_DISABLED` khi cờ chất lượng tắt (và `CODEINTEL_DISABLED` khi cờ gốc tắt); `GenerateReviewSummary` thêm cổng AI; RPC không có trong bảng ⇒ bị từ chối. Hành vi khi tắt giữa chừng: job reindex đang chạy chạy hết; consumer `codeintel.indexChanged`, outbox, bảo trì vẫn chạy; dữ liệu giữ nguyên. Gắn bộ đếm `orca_codeintel_disabled_rejections_total{rpc}` (BE-CV-SOL-071).

### 5.2 Kiểm tra đăng ký (task 04)

`backend-go/ci/check-code-intel-service-wiring.sh` (mới): `set -euo pipefail`, thông báo `FAIL:` như `check-opa-bundle-in-images.sh`, `grep -q` có neo từ (không `sed`; chạy trên Linux runner, Windows dùng WSL). Kiểm sự hiện diện `codeintel`/`code-intel-service` ở: `backend-go/go.work`, `backend-go/Makefile` `SERVICES`, **cả hai** `postgres-init-databases.sh`, `deploy/dev/scripts/migrate.sh` `SERVICES`, `deploy/dev/scripts/build-local.sh`, `deploy/dev/docker-compose.yml` (service `code-intel-service`, `container_name: orca-go-code-intel`, `migrate-codeintel`, `CODE_INTEL_SERVICE_ADDR` cho `api-gateway`, `CODEINTEL_ENABLED: ${CODEINTEL_ENABLED:-false}`), `backend-go/ci/check-opa-bundle-in-images.sh` (`svcs`, vì có `code_intel.rego`), `.github/workflows/backend-go-code-intel-service.yml` (ma trận `dialect`). Test của script: chạy trên bản sao thư mục tạm, xoá từng mục thì đỏ.

### 5.3 e2e T1 và tầng khác (task 05–08)

T1 `code-intel-service/e2e/` (tag `e2e`, testcontainers, ma trận `postgres`/`mysql`): agent **giả** phát lại tệp vàng của SOL-070, infra-fleet giả trong bộ nhớ, outbox và DB thật. Kịch bản E01–E18 theo CR-073 §2.3 (phần T1) với hiệu chỉnh: E04 ERD cần CR-031; E17 MCP chỉ khi CR-041; E15/E16 chỉ T3. `TestEveryRPCHasScenario` đọc 49 RPC từ `ServiceDesc`; RPC chưa có kịch bản phải nằm trong bảng `waivedScenario` có lý do và solution sở hữu. T2 (Playwright) và `code-intel-fake-backend.ts` thuộc `FE-CV-SOL-073`. T3 (`tests/code-intel/*.py`, `backend-go/ci/code-intel-e2e/run-code-intel-e2e.sh`, `.github/workflows/code-intel-e2e.yml`) hằng đêm/`workflow_dispatch`, **không chặn**; stack `deploy/dev` có chạy được trên runner CI hay không **chưa kiểm chứng**. T4 (Orca cỡ thật) dùng chung runner benchmark của SOL-071 (chưa có).

### 5.4 Rollout, quay lui, tài liệu (task 09)

Năm giai đoạn (0 Nội bộ, 1 Dogfood, 2 Beta, 3 Mở rộng, 4 GA) theo CR-073 §2.6, khớp đợt §7.3 của hợp đồng; cờ theo **tenant**, không theo lens; mặc định `false` kể cả tenant mới tới quyết định riêng. Quay lui: tắt cờ tenant hoặc `CODEINTEL_ENABLED=false`; cắt gateway `CODE_INTEL_SERVICE_ADDR=""` ⇒ `CODEINTEL_UNAVAILABLE`; `ORCA_CODEINTEL_DISABLED=1` ở agent; **không** `down` migration khi còn dữ liệu; giữ chỉ mục dev server. Tài liệu `docs/guides/code-intel/` (7 tệp ở CR-073 §2.8) viết tiếng Việt; hướng dẫn cài ghi `gitnexus analyze --index-only`, `PATH` của systemd (`deploy/agent/orca-agent.service`), hai loại token không nhầm; phần chưa kiểm chứng (cách cài CodeGraph không tương tác, thời gian dựng chỉ mục Orca) ghi rõ. Diễn tập quay lui (tắt cờ, cắt gateway, bật lại) là điều kiện giai đoạn 2.

## 6. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Cờ thi hành ở `code-intel-service` bằng interceptor có bảng phân loại | một điểm; WS, MCP, push cùng theo |
| D2 | Tắt cờ: chặn đọc/ghi, cho `GetSettings`/`SetSettings`/`GetReindexJob`, giữ dữ liệu, job đang chạy hoàn tất | không giam dữ liệu, không để `analyze` nửa chừng |
| D3 | Fail closed, cache 5 s không lưu lỗi | PQ-24 |
| D4 | Không migration trong solution này | bảng T1 thuộc CR-011 (L1) |
| D5 | T1 và T2 chặn PR; T3, T4 không | công cụ ngoài và stack đầy đủ không ổn định |
| D6 | `TestEveryRPCHasScenario` đọc từ `ServiceDesc` | thêm RPC mà quên e2e thì đỏ |
| D7 | Rollback không dùng `down` | tránh mất dữ liệu review |
| D8 | Script wiring kiểm cả hai `init-databases.sh` | lệch đã thấy (C1) |
| D9 | E05 khẳng định `git status --porcelain` không đổi sau reindex | `analyze` mặc định ghi `AGENTS.md`/`CLAUDE.md` (CR-073 mục 1.5) |

## 7. Tiêu chí chấp nhận

- [ ] `GetSettings` (mặc định `effective` tắt; thiếu dòng ⇒ chèn lười theo `CODEINTEL_TENANT_DEFAULT_ENABLED`) và `SetSettings` (admin, audit, validate) hoạt động trên Postgres và MySQL.
- [ ] Biến tổng tắt ⇒ mọi RPC trừ ba RPC ngoại lệ trả `CODEINTEL_DISABLED`; tenant tắt cũng vậy; bật cả hai thì chạy; tắt chất lượng ⇒ `QUALITY_GATE_DISABLED`; hiệu lực tắt ≤ 5 s (đo bằng giả đồng hồ).
- [ ] Tắt giữa chừng: job reindex hoàn tất, consumer/outbox chạy, dữ liệu còn nguyên sau bật lại; tenant A bật không ảnh hưởng B.
- [ ] `TestEveryRPCHasScenario` xanh; E01–E14, E18 (phần T1) xanh hai dialect; E05 khẳng định cây làm việc không đổi.
- [ ] `check-code-intel-service-wiring.sh` xanh và nằm trong CI; xoá `codeintel` khỏi `migrate.sh` thì đỏ.
- [ ] `code-intel-e2e.yml` chạy được qua `workflow_dispatch` (chưa kiểm chứng).
- [ ] Tài liệu `docs/guides/code-intel/` đủ, ghi rõ `--index-only` và `PATH` systemd; quay lui được diễn tập một lần.
- [ ] Không `max-lines` disable.

## 8. Kiểm thử

| Test | Nội dung |
|---|---|
| `internal/usecase/settings_usecase_test.go`, `effective_flags_test.go` | bảng hiệu lực, validate, admin-only, lười chèn dòng, fail closed, cache 5 s |
| `internal/adapter/grpc/feature_gate_test.go` | mọi RPC có phân loại; hành vi từng lớp khi tắt |
| repo `tenant_settings` hai dialect (BE-CV-SOL-011) | đọc, ghi, mặc định, cô lập |
| `e2e/*` | E01–E18 (T1), `TestEveryRPCHasScenario`, `feature_flag_test.go` |
| `ci/check-code-intel-service-wiring.sh` + test | xoá từng mục ⇒ đỏ |
| `tests/code-intel/check_code_intel_flow.py`, `check_code_intel_flag.py` | T3 (không chặn) |

Lệnh: `cd backend-go/services/code-intel-service && go test ./... && go test -tags=integration ./internal/adapter/... && go test -tags=e2e ./e2e/...` (Docker). **Chưa chạy.**

## 9. Rủi ro và điểm chưa kiểm chứng

- Stack `deploy/dev` trong runner CI và agent thật + công cụ thật trong CI chưa kiểm chứng; T3 vì thế không chặn.
- Cách cài CodeGraph không tương tác; thời gian dựng chỉ mục Orca chưa đo; `--index-only` chưa chạy để xác nhận không đụng tệp nào (E05 là kiểm chứng).
- `~/.gitnexus/registry.json` theo `HOME`; nhiều agent cùng `HOME` chưa kiểm chứng.
- Dev compose dùng superuser nên RLS không hiệu lực ở dev (README v7 §8 điểm 16).
- Cờ tắt ≤ 5 s là thiết kế cache, chưa đo thời gian thực.
- SSH/remote: `relay-ssh` (CR-006) ngoài phạm vi; e2e không giả định thực thi cục bộ. GitLab/provider khác: không có thành phần provider ở solution này.
- Mặc định cờ tenant mới ở GA và việc quyết định thuộc sản phẩm.

## 10. Câu hỏi mở

1. Ranh giới với `BE-CV-SOL-013-authorization-flags-and-audit`: ai sở hữu bộ đọc cờ và `feature_gate.go`? Đề xuất: 073 sở hữu interceptor + `EffectiveFlags` + RPC cờ; 013 sở hữu OPA/audit/hạn mức và dùng `EffectiveFlags`. Cần chốt trước khi viết code (tránh hai bộ đọc cờ).
2. Cần RPC `CancelReindex` (O-17)?
3. Playwright: dùng `MCP_E2E_BASE_URL` hay thêm `CODE_INTEL_E2E_BASE_URL`; chuyển hỗ trợ dùng chung ra khỏi `mcp-web` (phía FE).
4. T3, T4 hằng đêm trên runner chuyên dụng có được chấp nhận?
5. Đưa cờ vào `tenant-service` thay vì `code-intel-service` (CR-073 Q5): giữ ở service này.

## 11. Khoảng trống hợp đồng ghi nhận

- Hợp đồng không giao chủ sở hữu `feature_gate.go`/bộ đọc cờ giữa CR-013 và CR-073 (Q1).
- Không nói `SetSettings` có audit hay phải phát sự kiện (đề xuất: audit, không outbox).
- Không có cách đọc cờ AI/quét bảo mật cho từng RPC `QualityGateService` (chỉ nói `GenerateReviewSummary` và `StartQualityRun` với profile bảo mật).

## 12. Tham chiếu

- `backend-go/go.work`, `backend-go/Makefile`, `backend-go/deploy/postgres-init-databases.sh`, `deploy/dev/docker/postgres/init-databases.sh`, `deploy/dev/scripts/migrate.sh`, `deploy/dev/docker-compose.yml`, `backend-go/ci/check-opa-bundle-in-images.sh`
- `backend-go/services/mcp-service/internal/domain/tenant_settings.go`, `mcp-service/README.md`, `backend-go/common/internalcaller`
- `tests/playwright.web.config.ts`, `tests/e2e/mcp-web/`, `tests/mcp/`, `tests/backend/`, `.github/workflows/backend-go-issue-status-sync.yml`
- `docs/crs/v7/quality-rollout/CR-CV-073-…md`; mẫu `specs/backend-go/crs/v6/request-quality-rollout/solutions/BE-REQ-SOL-025-e2e-feature-flag-rollout.md`
