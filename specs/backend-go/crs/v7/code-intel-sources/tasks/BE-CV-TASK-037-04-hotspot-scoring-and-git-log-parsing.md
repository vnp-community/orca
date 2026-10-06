# BE-CV-TASK-037-04: Hotspot: parse `git log`, `churn`, proxy độ phức tạp, `centrality`, điểm

**From Solution:** BE-CV-SOL-037-structure-findings-and-dismissals
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/structurefinding/{git_log_parsing.go, hotspot_scoring.go}` và `_test.go` (mới); `internal/domain/structurefinding/git_quoted_path.go` (mới)
**Depends on:** BE-CV-TASK-037-01, BE-CV-TASK-037-03
**Status:** [ ] TODO

---

## Context

Chỉ dùng `git log` (≤ Git 2.25; `--since`, `--no-merges`, `--name-only`, `--format` đều cũ). Agent cấm cờ `-c` trước subcommand nên đường dẫn không-ASCII có thể bị C-quote; ngoài ra `SHELL_METACHARACTERS` cấm `& | ; $ \` < > \ !` trong đối số: `--format=%x00%H%x09%an` không chứa ký tự cấm.

## Việc cần làm

1. `git_quoted_path.go`: `DecodeCQuotedPath(s string) (string, bool)` giải mã `"…"` với `\ooo` (bát phân), `\n`, `\t`, `\"`, `\\`; tham chiếu hành vi của `agent/src/shared/git-cquoted-path.ts` `decodeGitCQuotedPath` (đọc tệp đó khi viết; chưa đọc lúc soạn); không quote ⇒ trả nguyên.
2. `git_log_parsing.go`: `ParseChurn(raw []byte) ([]CommitFiles, error)` tách theo NUL/TAB đúng định dạng `%x00%H%x09%an` + dòng tên tệp; không lấy email. Trả `{Hash, Author, Files []string}`. Chống input xấu: bỏ commit không có hash 40/64 hex; ≤ 8 MiB (vượt ⇒ `ErrLogTooLarge`).
3. `hotspot_scoring.go`: `Churn(commits, window) map[path]Churn{Commits, Authors}`: bỏ commit chạm > 100 tệp, bỏ tệp nhiễu (`pnpm-lock.yaml`, `package-lock.json`, `go.sum`, `i18n/locales/*.json`, `proto/gen/**`, `**/*.md`, `docs/**`, `specs/**`); `Percentile(values)` (hạng phần trăm, xác định khi bằng nhau); `HotspotFindings(churn, sizes, inDegree, opts)`: `complexity = 0.5·pr(totalLines)+0.5·pr(longestSymbol)`, `centrality = pr(inDegree)`, `score = pr(churn)·complexity·centrality`; vào danh sách khi `churn ≥ 3` và `score ≥ 0.05`; ≤ 50, giảm dần; `metrics{churn, authors, complexity, centrality, score, totalLines, longestSymbolLines}`; `severity:"info"`; `confidence:"high"`, hạ `low` khi dữ liệu chia khoảng.
4. `SplitWindow(from, to)` trả hai khoảng khi lần đầu vượt hạn mức (dùng bởi use case).

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/structurefinding/ -run 'Churn|Hotspot|CQuoted|GitLog'` trên `testdata/findings/git-log-90d.txt`: bỏ commit 150 tệp; bỏ tệp nhiễu; tên tệp dấu cách/Unicode; không có email trong đầu ra (kiểm bằng quét byte); biên `churn=2/3`, `score=0.049/0.05`; hạng phần trăm bằng nhau; ≤ 50; 100 lần giống nhau; input hỏng không panic.

## Tiêu chí hoàn thành

- [ ] Tiêu chí hotspot của §9 đạt.
- [ ] Nhãn `complexity` là "độ dài hàm" ở `titleKey`/`params` (không gọi cyclomatic).

## Rủi ro và lưu ý

- Di chuyển/đổi tên hàng loạt làm hụt churn (renameLimit); chấp nhận, nêu trong `params.note` khi phát hiện cây `src/` và `frontend/src/` cùng tồn tại? (không cố suy; chỉ ghi ở tài liệu UI).
- Percentile trên tập nhỏ dễ cho `score` cực đoan; giữ ngưỡng `churn ≥ 3`.
