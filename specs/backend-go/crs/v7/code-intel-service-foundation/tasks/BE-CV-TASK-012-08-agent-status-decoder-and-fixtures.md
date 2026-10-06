# BE-CV-TASK-012-08: Bộ giải mã `codeintel.status` của agent và fixture

**From Solution:** BE-CV-SOL-012-index-status-aggregation
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/agent_status.go`, `agent_status_test.go`, `backend-go/services/code-intel-service/testdata/agent-status/{full-ready,stale-repo-root,missing,not-installed,codegraph-root-mismatch,malformed}.json` (mới)
**Depends on:** BE-CV-TASK-011-04
**Status:** [ ] TODO

---

## Context

Hình dạng chuẩn: `CONTRACT-codeintel-agent-rpc.md` §4.1 (đã áp PQ-19: `binding.worktreeMismatch` boolean, `indexes.codegraph.rootMismatch`) và phong bì §2.2 (`sources`, `headCommit`, `stale`, `warnings`, `perf`, `data`). Tệp vàng G1 (`AG-CV-SOL-070`) chưa có: dùng fixture tay theo §4.1 và đối chiếu khi có.

## Việc cần làm

1. `DecodeAgentStatus(envelope map[string]any) (AgentStatus, error)`: đọc `data.binding`, `data.tools`, `data.indexes`, `headCommit`, `stale`, `warnings`; trường lạ bỏ qua; sai kiểu bắt buộc → `CODEINTEL_RESULT_INVALID` (`apperrors` Kind `Internal`, thông điệp không chứa giá trị).
2. Kiểm biên: chuỗi ≤ 512 (đường dẫn ≤ 4096), số không âm, `state ∈ missing|building|ready|stale|unknown` (lạ → `unknown`), `indexScope ∈ exact|repo_root|stale|none` (lạ → `none`), `freshness ∈ fresh|fresh_base|stale|unknown` (lạ → `unknown`), `percent` sai kiểu → nil.
3. Struct chỉ giữ trường cần; **đường dẫn tuyệt đối** vào các trường riêng `…Private` (không đi tới proto UI).
4. Sáu fixture: `full-ready` (§4.1 nguyên văn), `stale-repo-root` (GitNexus `repo_root/fresh_base`, CodeGraph `exact/fresh`), `missing` (state `missing` cả hai), `not-installed` (`available:false`), `codegraph-root-mismatch`, `malformed` (kiểu sai).
5. Test fuzz nhẹ (`go test -fuzz` không bắt buộc): đầu vào ngẫu nhiên không panic.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/... -run AgentStatus`
- Mỗi fixture → kết quả kỳ vọng; `malformed` → lỗi; không panic với `nil`, mảng thay object.

## Tiêu chí hoàn thành

- [ ] Giải mã đúng §4.1 trên fixture đầy đủ.
- [ ] Enum lạ rơi về `unknown`/`none`.
- [ ] Không đường dẫn tuyệt đối trong kiểu public trả đi.

## Rủi ro và lưu ý

- Khi tệp vàng AG-070 có, thay fixture tay hoặc thêm test đối chiếu và ghi lệch.
