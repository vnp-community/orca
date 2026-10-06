# BE-CV-TASK-037-05: Mã chết (`dead.unused-export`) và owner (CODEOWNERS + lịch sử)

**From Solution:** BE-CV-SOL-037-structure-findings-and-dismissals
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/structurefinding/{dead_export_findings.go, owner_resolution.go, codeowners_matching.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-037-03
**Status:** [ ] TODO

---

## Context

CR §2.3 (mã chết, owner). `scm-integration-service/internal/usecase/codeowners.go` có `ParseCodeowners`/`MatchOwners` nhưng nằm trong `internal` của module khác (không import được) và dùng `path/filepath`; viết lại bằng `path`.

## Việc cần làm

1. `dead_export_findings.go`: `DeadExportFindings(symbols []SymbolRef) []Finding`: chỉ `kind:"function"`, ngôn ngữ Go, đường dẫn dưới `backend-go/services/`; loại `_test.go`, `/cmd/`, `usecasetest`, `proto/gen`; `severity:"info"`, `confidence:"medium"`, `titleKey` `codeintel.finding.dead.unused-export`, `params{name, file}`; `evidence{path, line=startLine, symbol}`; khoá `SymbolSubject(symbol.key)`.
2. `codeowners_matching.go`: `ParseCodeowners(content string) (rules []Rule, warnings []string)` (bỏ dòng trống, `#`, dòng một cột; dòng section `[Tên]`/`^[Tên]` bỏ + cảnh báo `section_unsupported`); `OwnersOf(rules, path string) []string` — luật cuối khớp thắng; khớp kiểu gitignore: mẫu có `/` đầu neo gốc, kết thúc `/` là thư mục, `*`, `**`, không dùng `filepath`.
3. `owner_resolution.go`: `ResolveOwner(path string, rules []Rule, history *HistoryShare) *Owner`: ưu tiên CODEOWNERS (`source:"codeowners"`, `names`), nếu rỗng dùng `history` (`source:"history"`, tác giả ≥ 50% commit, không ai đạt ⇒ top 2, `share`); `ParseShortlog(raw []byte)` cho đầu ra `git shortlog -sn` (`   12\tTên`); **không email**, bỏ dòng có `<…>`.
4. Hàm dựng lệnh cho use case: `ShortlogArgs(days int, path string) []string` trả `["shortlog","-sn","--since=<N>.days","--no-merges","HEAD","--",path]` (HEAD bắt buộc); kiểm `path` không chứa ký tự `SHELL_METACHARACTERS`, nếu có ⇒ bỏ owner cho tệp đó (không lỗi).

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/structurefinding/ -run 'Dead|Codeowners|Owner|Shortlog'`: `testdata/findings/CODEOWNERS.sample` (luật chồng, `**`, thư mục, một cột, section GitLab); so hành vi với bộ test `scm-integration-service/.../codeowners_test.go` (đọc khi viết để lấy ca; không import); `unusedExports` fixture ⇒ `NewFleetDefinitionStore` hai bản, không hàm test; không `Method`; owner không email; `ShortlogArgs` có `HEAD` và `--`; đường dẫn có `$` bị bỏ.

## Tiêu chí hoàn thành

- [ ] Tiêu chí mã chết và owner của §9 đạt.
- [ ] `ParseCodeowners` chạy giống nhau khi đường dẫn có `\` (Windows) — dùng `path`.

## Rủi ro và lưu ý

- Tỷ lệ báo nhầm mã chết chưa đo; `confidence:"medium"` + lời nhắn "có thể gọi qua reflection/đăng ký ngoài mã" ở `params`.
