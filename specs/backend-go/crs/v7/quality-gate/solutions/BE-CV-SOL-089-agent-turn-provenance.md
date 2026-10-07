# BE-CV-SOL-089-agent-turn-provenance: Lượt agent bền (`agent_turns`), tóm tắt lệnh, lời tự báo và đối chiếu độc lập

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Nguồn dữ liệu duy nhất ở MVP là **renderer ghi** (PQ-35); nguồn hook phía backend **không** thuộc hợp đồng này (chưa kiểm chứng, O-8).

**CR:** [CR-CV-089](../../../../../../docs/crs/v7/quality-gate/CR-CV-089-agent-provenance-and-claim-reconciliation.md)
**Service:** `code-intel-service` (mới) · `codeintel_agent_turn.proto` (mới) · dòng `rpc` thêm vào `codeintel_quality_gate.proto`
**Hợp đồng:** [CONTRACT-codeintel-proto-and-data-map.md](../../CONTRACT-codeintel-proto-and-data-map.md) (PQ-22, PQ-35, §2.1 dòng 21, §3.2, §4.2 T14, §4.3, §5, §6.1), [CONTRACT-codeintel-ui-api.md](../../CONTRACT-codeintel-ui-api.md) (§3.2 `quality.turn.record|turns|turn`, §4.7 `AgentTurn`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (§Multi-tenancy, §Transactional outbox), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (§Audit logging, §Input validation), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (§Event conventions), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

---

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

### 0.1 Hợp đồng áp dụng

| PQ / mục | Áp dụng |
|---|---|
| PQ-22 | `agent_turns` là kho **provenance** phía backend; `ReviewTurnMarker.turnId` = `client_turn_id` = `"${paneKey}:${doneAt}"`; marker từng tệp **không** đưa vào bảng này |
| PQ-35 | Chỉ nguồn A (renderer gọi `RecordAgentTurn`); `agent.hook` không dùng; `source` lưu `renderer` (cột cho phép `hook|both` để không đổi migration sau) |
| PQ-04 | Request dùng `selector`; `repo_binding_id` chỉ ở bảng/response |
| PQ-01, PQ-03 | `CODEINTEL_QUALITY_GATE_DISABLED` khi cờ tắt; id lượt không thuộc tenant/binding → `CODEINTEL_NOT_FOUND`; tham số sai → `CODEINTEL_INVALID_PARAMS` |
| PQ-24 | `agent_turn_store_prompt_excerpt`, `agent_claim_text_enabled` đã nằm ở T1 (migration `0002`), cache cờ 5 s |
| §4.2 T14, §4.3 | Cột, khoá, chỉ mục, giữ ≤ 200 lượt/binding và 90 ngày (văn bản 30 ngày) |
| §5 | Outbox `orca.codeintel.agent_turn.recorded` `{repo_binding_id, turn_id, end_head_commit}`; consumer `quality.run_finished` đối chiếu |
| §6.3 | `review_write` cho ghi; `quality_read` cho đọc |
| H8 | Không lưu prompt/transcript/lệnh nguyên văn; chuỗi tự do qua bộ che bí mật |

### 0.2 Lệch giữa CR và hợp đồng

| # | CR nói | Hợp đồng quyết | Xử lý |
|---|---|---|---|
| L1 | `agent_turn.proto`; request `repo_binding_id` | PQ-07/04: `codeintel_agent_turn.proto`; `selector` | Theo hợp đồng |
| L2 | Hai nguồn A/B, nguồn B "chờ spike" | PQ-35: B ngoài hợp đồng | Chỉ A; mã nguồn B không viết, cột `source` giữ ba giá trị |
| L3 | `RecordAgentTurn` do renderer gửi `commandsSummary`, `claims` | ui-api §3.2 đúng vậy | Backend **không tin**: tự tính lại `category`, bắt `ran_command` suy từ `commandsSummary`, chỉ nhận `stated` khi cờ bật (task 089-03, 089-05) |
| L4 | `DeleteAgentTurns` (Q5) | §3.2 không có RPC này | **Không** làm; ghi Q2 |
| L5 | Tham số `before?` của `ListAgentTurns` không định kiểu | ui-api §3.2 `before?` | Đề xuất con trỏ mờ `base64(ended_at_utc|id)`; chờ người duyệt (Q3) |
| L6 | Consumer `index.changed` điền `verification` | §5 liệt 089 là consumer của `index.changed` | Không thấy lý do dùng sự kiện này để đối chiếu (đối chiếu chỉ cần run); không viết consumer, ghi Q4 |
| L7 | `CODEINTEL_AGENT_TURN_RETENTION_DAYS`, `…_TEXT_RETENTION_DAYS` | §6.2 không liệt | Giữ hai biến, mặc định 90/30, giá trị khởi điểm chưa hiệu chỉnh |
| L8 | `model_source ∈ agent_session|transcript|unknown` | Renderer gửi `model?` nhưng không nói nguồn | Lưu `model_source='unknown'` cho mọi giá trị do renderer gửi (chưa kiểm chứng được); Q5 |

### 0.3 Phụ thuộc chéo khu vực

| Hướng | Solution | Dùng gì |
|---|---|---|
| FE đối ứng | `FE-CV-SOL-089-agent-turn-recorder` (hàm thuần dựng payload, `prompt_digest` bằng `crypto.subtle`, gọi `codeIntel.quality.turn.record`) · `FE-CV-SOL-060-review-notes-and-turn-compare` (`ReviewTurnSwitcher`, `turnId`) · `FE-CV-SOL-087-quality-scorecard-and-state` (thẻ lượt "Agent đã chạy … Chạy lại …") | JSON `AgentTurn`; quy ước `clientTurnId` |
| AG | Không có solution AG (§8.2). Ghi rõ: Part A chưa có bộ phát `agent.hook` (CR §1.2, chưa kiểm chứng ở phiên này) nên không có việc AG | — |
| BE | SOL-085-evaluator (`quality_trend_points.turn_key`, cờ/OPA), SOL-085-waivers-and-trend (consumer `agent_turn.recorded` ghi điểm), BE-CV-SOL-082 (`quality_runs.head_commit/source/dirty/error_code`), BE-CV-SOL-013 (che bí mật, quyền, quota), BE-CV-SOL-040-codeintel-quality-channels (kênh `turn.*`) | |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: CR-089 đủ; hợp đồng đủ; `common/{eventbus,outbox,dbcapability,auditclient}`; `infra-fleet-service/internal/adapter/devserveragent/session.go` (CR dẫn `:539`, `agentHookNotificationParams` chỉ giải mã `worktreeId`, `ptyId`, `providerSession` — theo CR, **chưa tự mở lại** ở phiên này); `agent/src/shared/agent-status-*.ts` (theo CR).

Xác nhận ở phiên soạn: chưa có `code-intel-service`, chưa có bảng `agent_turns` hay proto `codeintel`. Các khẳng định về `agent.hook`/Part A trong CR **chưa kiểm chứng lại** (nguồn: CR-089 §1.2); solution này không dựa vào chúng.

### Correction relative to CR-CV-089

| # | CR nói | Mã thật / hợp đồng | Xử lý |
|---|---|---|---|
| C1 | `secret_redactor.go` của CR-013 | Chưa tồn tại; chủ sở hữu bộ che chưa chốt (O-16) | Port `TextRedactor` trong `usecase`; adapter do BE-CV-SOL-013 |
| C2 | Cờ chất lượng "CR-085 2.2" | Cột đã ở T1 | Đọc qua cổng cờ BE-013 |
| C3 | Tín hiệu "agent xong" | PQ-35: chỉ `orca.infra.agent.statusChanged` hoặc hint P1; **không** thuộc solution này | Không có |

## 2. Giải pháp chi tiết

### 2.1 Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_agent_turn.proto    # AgentTurn, CommandSummary, AgentClaim, ClaimVerification + Record/List/Get req-resp
  (+ 3 dòng rpc trong codeintel_quality_gate.proto)
services/code-intel-service/internal/
  domain/agent_turn.go, agent_turn_merge.go          # entity + MergeAgentTurn (hàm thuần)
  domain/agent_command_summary.go                    # chuẩn hoá tên lệnh + category (bảng mẫu có phiên bản)
  domain/agent_claim_extractor.go                    # ran_command từ commands_summary; kiểm stated
  domain/agent_claim_reconciler.go                   # consistent|contradicted|unverified|not_claimed
  usecase/record_agent_turn.go, reconcile_agent_turn.go, agent_turn_maintenance.go
  usecase/agent_turn_queries.go                      # List/Get
  adapter/{postgres,mysql}/agent_turn_repository.go
  adapter/eventbus/agent_turn_reconcile_consumer.go
  adapter/grpc/agent_turn_server.go                  # phương thức của QualityGateServer
  migrations/{postgres,mysql}/0006_agent_turns.{up,down}.sql
```

### 2.2 Ghi lượt: `RecordAgentTurn`

Chuỗi: cờ (`code_intel` ∧ `quality_gate`) → OPA `review_write` → `selector → binding` → kiểm đầu vào → hợp nhất/ghi → outbox → trả `{turn}`.

**Kiểm đầu vào** (mọi giá trị do client gửi đều không tin):

| Trường | Quy tắc |
|---|---|
| `clientTurnId` | 1–128 ký tự in được, không điều khiển; chuỗi mờ |
| `agentType` | chữ thường, `^[a-z0-9._-]{1,64}$`, rỗng → `unknown` |
| `model` | ≤ 128; `model_source` luôn `unknown` (L8) |
| `endedAt` | RFC 3339 UTC, không quá 5 phút ở tương lai (đề xuất, chưa hiệu chỉnh); `startedAt ≤ endedAt` |
| `endHeadCommit` | `^[0-9a-f]{7,64}$` |
| `filesDigest`, `promptDigest` | rỗng hoặc 64 hex |
| `promptExcerpt` | chỉ lưu khi `agent_turn_store_prompt_excerpt`; ≤ 160, qua `TextRedactor`; còn lại bỏ im lặng |
| `commandsSummary` | `v==1`, ≤ 20 `commands`; `name`, `sub` khớp `^[A-Za-z0-9._+-]{1,32}$`; `category` **do backend tính lại** từ `name/sub` bằng bảng mẫu có `v`; `toolCounts` ≤ 32 khoá; tổng JSON ≤ 8 KiB |
| `claims` | tối đa 20 mục; bỏ mọi mục `basis=stated` khi `agent_claim_text_enabled=false`; `confidence` luôn ≤ `medium`, `stated` luôn `low`; `evidence` ≤ 120 ký tự qua `TextRedactor`; **mọi** `ran_command` do backend suy từ `commandsSummary` (không nhận từ client) |

**Hợp nhất:** transaction: `SELECT … FOR UPDATE` theo `(tenant_id, repo_binding_id, client_turn_id)` (cả hai dialect, InnoDB/PG); nếu có → `MergeAgentTurn(existing, incoming)` (trường có giá trị thắng trường rỗng; không bao giờ ghi đè giá trị đã có bằng rỗng; `source` hợp `renderer`+`hook` → `both`; `version+1`); nếu chưa → chèn, `base_head_commit` = `end_head_commit` của lượt liền trước `ended_at < ?` của cùng binding (NULL ở lượt đầu); vi phạm duy nhất (PG `23505`, MySQL `1062`) → thử lại **một lần** bằng nhánh hợp nhất. Lượt đến sai thứ tự không sửa `base_head_commit` của lượt sau (hạn chế đã biết).

`expires_at = ended_at + CODEINTEL_AGENT_TURN_RETENTION_DAYS`. Ghi outbox `orca.codeintel.agent_turn.recorded` cùng transaction, `event_id` = UUID v5 của `(tenant, turn_id, version)`.

### 2.3 Tóm tắt lệnh và lời tự báo

- `domain/agent_command_summary.go`: bảng mẫu **có phiên bản** ánh xạ `(name, sub)` → `category ∈ test|lint|typecheck|build|install|git|other` (vd `pnpm test`/`go test`/`vitest` → `test`; `oxlint`/`golangci-lint` → `lint`; `tsc` → `typecheck`). Hàm `NormalizeCommandPreview(preview string) (name, sub string, ok bool)` (thuần, fuzz): lấy chương trình + tối đa một tiểu lệnh, bỏ đối số, đường dẫn, biến môi trường, chuyển hướng, URL; cắt ở ranh giới rune UTF-8. Dùng để **làm sạch** tên lệnh client gửi và sẵn cho nguồn B sau này.
- `ran_command` chỉ nghĩa "đã chạy lệnh loại đó" (không có mã thoát) → `confidence=medium` tối đa; UI không gọi là "báo đạt".
- `stated`: chỉ khi cờ tenant bật; mức `low`.

### 2.4 Đối chiếu độc lập (`ReconcileAgentTurn`)

Kích hoạt bởi consumer durable `orca.codeintel.quality.run_finished` (và có thể gọi lại khi `RecordAgentTurn` đến sau run). Với từng lượt của binding có `end_head_commit = run.head_commit`:

| Claim `kind` | Category của run độc lập | Ghi chú |
|---|---|---|
| `tests_pass`, `tests_fail` | `test` | |
| `lint_clean` | `lint` | |
| `typecheck_clean` | `typecheck` | |
| `build_ok`, `all_done` | — | không có kiểm tra độc lập tương ứng → `unverified`, `agreementReason="no_equivalent_check"` (quyết định của solution này, chưa có trong CR) |

Chọn run độc lập: `quality_runs.source='local'` (hoặc `ci`), cùng `head_commit`, check cùng category theo `checks[].category ↔ checks[].profile` của **profile hiệu lực** (đọc qua `GetQualityProfile` use case của SOL-085); không có check cùng category → `unverified`.

| `agreement` | Điều kiện |
|---|---|
| `consistent` | run `succeeded` cùng category, `error=0`, không `env_not_ready` |
| `contradicted` | run cùng category có `error`/test thất bại **và** `tree_dirty_end=false` **và** run `dirty=false` |
| `unverified` | chưa có run; run `unknown` do `CODEINTEL_ENV_NOT_READY`/`error_code≠''`; `tree_dirty_end=true` (`agreementReason="tree_may_differ"`: lượt chưa có dấu vân tay cây, nên không bao giờ `contradicted` khi cây bẩn) |
| `not_claimed` | không có claim cùng category |

Kết quả ghi vào `verification` (≤ 8 KiB): `{v:1, items:[{kind, agreement, reason, runId}]}`. **Không** đổi `verdict` của cổng (CR §2.4 (3); cờ `contradicted → warn` mặc định tắt và **không được hiện thực ở solution này**, Q1 kế thừa).

### 2.5 Đọc: `ListAgentTurns`, `GetAgentTurn`

`quality_read`. `ListAgentTurns`: `limit ≤ 50` (ngoài khoảng → `CODEINTEL_INVALID_PARAMS`), sắp `ended_at DESC, id DESC`, `before` (L5); gắn `gate?:{verdict, evaluatedAt}` bằng **một** truy vấn `quality_trend_points … WHERE tenant_id=? AND repo_binding_id=? AND turn_key IN (…)` (≤ 50 khoá; tránh N+1). `GetAgentTurn(turn_id)`: id lạ/khác tenant/khác binding → `CODEINTEL_NOT_FOUND`.

### 2.6 Bảo trì và vòng đời

Công việc nền (`CODEINTEL_MAINTENANCE_INTERVAL`, lô 500, `withMaintenanceTx`): xoá lượt `expires_at < now` hoặc vượt 200/binding (giữ mới nhất); sau `…_TEXT_RETENTION_DAYS` đặt `prompt_excerpt = NULL` và xoá `claims[].evidence`; xoá lượt mồ côi (binding không còn) quá 7 ngày (§4.3). Idempotent, đồng hồ DB.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Backend tự suy `ran_command` và tự tính `category` | Client không đáng tin (CR §1.2 điểm 3: hook/renderer có thể giả) |
| D2 | `unverified` là trạng thái thật | H7 |
| D3 | `contradicted` chỉ khi cây sạch cả hai phía | Tránh quy kết sai do môi trường/cây thay đổi |
| D4 | Không lưu prompt mặc định | Đồng nhất CR-060 |
| D5 | `FOR UPDATE` + thử lại một lần thay vì `INSERT … ON CONFLICT` thuần | Hợp nhất trường (không phải ghi đè) cần đọc trước; cùng ngữ nghĩa hai dialect |
| D6 | `build_ok`/`all_done` luôn `unverified` | Không có kiểm tra độc lập tương đương trong hợp đồng profile |

## 4. Tiêu chí chấp nhận

- [x] Hai lần `RecordAgentTurn` cùng `clientTurnId` → một hàng; trường đã có không bị ghi đè bằng rỗng; hai dialect cùng kết quả.
- [x] `base_head_commit` lượt N = `end_head_commit` lượt N−1; lượt đầu NULL.
- [x] Bảng ca chuẩn hoá lệnh (`pnpm test --filter x`, `cd /tmp && rm -rf …`, URL, `FOO=bar cmd`, `curl -H "Authorization: …"`) không để lọt đối số/đường dẫn/secret; ≤ 20 mục; không panic với chuỗi cắt giữa rune.
- [x] `promptExcerpt` không lưu khi cờ tắt; bật thì ≤ 160 ký tự và đã che; `promptDigest` không đảo ngược được.
- [x] Claim `ran_command` không bao giờ `confidence` > `medium`; `stated` không sinh khi cờ tắt; claim do client gửi mà `commandsSummary` không hậu thuẫn bị bỏ.
- [x] Bốn trạng thái `agreement` có test; cây bẩn → `unverified/tree_may_differ`; run `env_not_ready` → `unverified`.
- [x] `verification` không đổi `verdict` của `GetQualityGate`.
- [x] Bảo trì xoá >90 ngày và >200/binding, xoá văn bản >30 ngày; chạy lặp không đổi kết quả.
- [x] Người không có `review_write` không ghi được; người không thuộc project → `CODEINTEL_NOT_AUTHORIZED`; cờ tắt → `CODEINTEL_QUALITY_GATE_DISABLED`.
- [x] Mọi truy vấn có `tenant_id`; tenant A không thấy/ghi lượt tenant B (hai dialect); `GetAgentTurn` id tenant khác → `CODEINTEL_NOT_FOUND`.
- [x] Không có `agentType`/`model`/id lượt trong nhãn metric hoặc log mức INFO (H8; CR-095 telemetry cũng không nhận).

## 5. Kiểm thử

- **Unit:** chuẩn hoá lệnh (bảng + fuzz ngắn), `MergeAgentTurn` (bảng), `AgentClaimReconciler` (4 trạng thái × cây sạch/bẩn × run), kiểm đầu vào.
- **Repository (integration, hai dialect):** hai goroutine ghi cùng `clientTurnId`, unique, `FOR UPDATE`, chỉ mục, UTF-8, lô xoá.
- **Use case:** cờ tắt, quyền, `TextRedactor` giả, outbox cùng transaction (rollback không để lại event).
- **Consumer:** giao lặp `run_finished` cùng `event_id`; run đến trước lượt (thứ tự ngược) → reconcile khi lượt được ghi.
- **Hợp đồng:** golden `AgentTurn` (ui-api §4.7) dùng chung với FE.
- **Spike bắt buộc (không thuộc solution):** ghi envelope `agent.hook` thật trước khi bàn nguồn B (O-8).

## 6. Rủi ro và điểm chưa kiểm chứng

- Không có mã thoát và không có lịch sử tool: `ran_command` chỉ chứng minh đã chạy lệnh; độ chính xác của `stated` chưa đo.
- Renderer đóng → mất lượt; nguồn B không có (PQ-35). `commands_summary` có thể rỗng.
- Dữ liệu là "bằng chứng ghi nhận", không xác thực (agent có thể giả sự kiện hook/renderer bị thao túng).
- `model` chỉ do client khai (L8). Chưa đọc `agent_sessions.model_id` ở phiên này.
- Chưa có bộ che bí mật dùng chung (O-16); `TextRedactor` là cổng, chưa có adapter thật.
- Nhóm hạn mức 200/90/30 là giá trị khởi điểm chưa hiệu chỉnh; chưa qua đội bảo mật (quyền riêng tư dữ liệu lượt).

## 7. Câu hỏi mở

- **Q1.** Cho `contradicted → warn` vào cổng? Mặc định không (O13) — chưa làm.
- **Q2.** `DeleteAgentTurns` (yêu cầu xoá dữ liệu theo admin) cần thêm vào hợp đồng §3.2 nếu muốn; hiện không.
- **Q3.** Kiểu `before` của `ListAgentTurns` (L5).
- **Q4.** `index.changed` có thật sự cần consumer của 089 (L6)?
- **Q5.** Nguồn `model` đáng tin (từ `agent_sessions.model_id` qua infra-fleet) — CR riêng.
- **Q6.** Liên kết `agent_turns ↔ task.execution_links` (CR §Q6) — để CR-092.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-089-agent-provenance-and-claim-reconciliation.md`
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-22, PQ-35, T14, §5), `CONTRACT-codeintel-ui-api.md` (§3.2, §4.7)
- `/opt/repos/orca/backend-go/common/{eventbus,outbox,dbcapability,auditclient}/`
