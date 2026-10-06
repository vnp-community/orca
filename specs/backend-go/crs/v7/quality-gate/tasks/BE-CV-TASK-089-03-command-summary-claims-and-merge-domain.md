# BE-CV-TASK-089-03: Domain chuẩn hoá lệnh, suy `ran_command`, hợp nhất lượt (hàm thuần)

**From Solution:** BE-CV-SOL-089-agent-turn-provenance
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/domain/agent_turn.go`, `agent_turn_merge.go`, `agent_command_summary.go`, `agent_claim_extractor.go` (mới)
**Depends on:** BE-CV-TASK-089-02
**Status:** [ ] TODO

## Việc cần làm
1. `NormalizeCommandPreview`: chương trình + tối đa một tiểu lệnh; bỏ đối số, đường dẫn, env, chuyển hướng, URL; cắt theo rune.
2. Bảng mẫu có `v` ánh xạ `(name, sub)` → `category`; **backend tính lại category**, bỏ giá trị client.
3. `ValidateCommandsSummary` (≤ 20 mục, tên `^[A-Za-z0-9._+-]{1,32}$`, ≤ 8 KiB); `DeriveRanCommandClaims`; lọc `stated` (chỉ khi cờ), `confidence` ≤ `medium`.
4. `MergeAgentTurn(existing, incoming)`: trường có giá trị thắng rỗng; không ghi đè bằng rỗng; `source` `renderer`+`hook` ⇒ `both`.

## Kiểm thử
- Bảng ca lệnh (`pnpm test --filter x`, `cd /tmp && rm -rf …`, URL, `FOO=bar cmd`, `curl -H "Authorization: …"`); fuzz ngắn không panic với chuỗi cắt giữa rune; bảng merge.

## Tiêu chí hoàn thành
- [ ] đầu ra không chứa đối số/đường dẫn/secret; [ ] `ran_command` không bao giờ > `medium`; [ ] merge không xoá dữ liệu.

## Rủi ro
- Bảng mẫu theo CLI của từng agent (tên tool khác nhau) chưa kiểm chứng với dữ liệu hook thật.
