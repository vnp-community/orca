# AG-CV-TASK-001-05: `runCodeIntelTool`: tệp tạm cho GitNexus, phân loại đầu ra, che bí mật

**From Solution:** [AG-CV-SOL-001-codeintel-agent-foundation](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) mục 2.4
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-tool-runner.ts` (mới), `agent/src/relay/codeintel-tool-output-classification.ts` (mới), `agent/src/relay/codeintel-secret-redaction.ts` (mới), và ba file `*.test.ts` cùng tên
**Depends on:** [01](./AG-CV-TASK-001-01-codeintel-errors-and-strict-params.md), [02](./AG-CV-TASK-001-02-codeintel-limits-and-concurrency-gate.md), [03](./AG-CV-TASK-001-03-codeintel-command-whitelist-and-child-env.md), [04](./AG-CV-TASK-001-04-run-tool-command-options-and-legacy-tool-guard.md)
**Status:** [ ] TODO

## Context

CR-001 1.9 (đã đo ở CR): `gitnexus cypher` 3 000 hàng qua pipe shell cho đúng 65 536 byte, qua `spawn` pipe 146 176 byte, JSON hỏng, `exit 0`; ghi ra tệp cho 411-436 KB hợp lệ. Vì vậy mọi `gitnexus` bắt buộc qua tệp tạm (contract §2.4). `gitnexus cypher` báo lỗi bằng `{"error":"…"}` ở stdout với exit 0; `codegraph` in lỗi dạng text ANSI exit 0 (`Symbol "…" not found`, `CodeGraph not initialized in …`); `status` ở thư mục chưa khởi tạo trả `{"initialized":false,…}`. Các hành vi này **do CR đo, chưa chạy lại** khi soạn solution: task này có bước ghi fixture.

## Việc cần làm

1. `codeintel-secret-redaction.ts`: `redactForClient(text, {home})` thay `$HOME` bằng `~`, che `gh[pousr]_[A-Za-z0-9]+`, `github_pat_…`, `AKIA[0-9A-Z]{16}`, `sk-[A-Za-z0-9-]+`, JWT (`eyJ…\.…\.…`), khối `-----BEGIN … PRIVATE KEY-----…`, `scheme://user:pass@`; hàm `tailForStderr(text)` = 2 KiB cuối đã che, bỏ ANSI (`/\u001b\[[0-9;]*m/g`).
2. `codeintel-tool-output-classification.ts`: `classifyGitNexusStdout(text)` -> `{kind:'json', value}` | lỗi: JSON không parse -> `TOOL_FAILED reason='truncated_stdout'`; đối tượng có `error` string -> phân loại (`not found|does not exist` -> `{notFound:true}` để handler chọn `SYMBOL_NOT_FOUND`; `Write operations` -> `TOOL_FAILED reason='write_blocked'` + log error; còn lại `TOOL_FAILED`); dạng khác -> `unknown_shape`. `classifyCodeGraphStdout(text)`: sau bỏ ANSI, nếu không bắt đầu `[`/`{` -> `Symbol "…" not found` -> `SYMBOL_NOT_FOUND`, `not initialized` -> `INDEX_MISSING`, còn lại `TOOL_FAILED reason='unknown_shape'`.
3. `codeintel-tool-runner.ts`: `runCodeIntelTool(cmd, binding, opts:{signal, deadline, tool})`:
   - dựng argv bằng task 03; resolve binary trong `config.toolPath.split(path.delimiter)` (hàm riêng của codeintel vì `resolveToolBinary` của registry là private và tách `':'`);
   - `acquire` cổng (task 02) với `signal`; đổi hết hạn: `min(20 s/lần CLI, deadline còn lại)` -> `CODEINTEL_TIMEOUT {tool, elapsedMs}`;
   - GitNexus: thư mục `fs.mkdtemp(path.join(os.tmpdir(), 'orca-codeintel-' + uid + '-'))` (chmod `0700`), tệp `0600` cờ `wx`, truyền `stdoutFile` cho `runToolCommand`; đọc tệp sau khi thoát; xoá tệp + thư mục trong `finally` (cả khi lỗi/timeout/huỷ); theo dõi kích thước mỗi 500 ms (> `toolMaxOutputBytes` -> kill + `OUTPUT_TOO_LARGE {bytes, limit}`); khi khởi động (hàm `sweepStaleCodeIntelTempDirs()` gọi lười ở lần chạy đầu) xoá thư mục `orca-codeintel-*` cũ hơn 1 giờ **thuộc cùng uid**;
   - CodeGraph: pipe thường, `maxOutputBytes` 16 MiB;
   - env = `buildCodeIntelChildEnv`; `cwd` = `binding.toplevel`; exit != 0 -> `TOOL_FAILED {tool, exitCode, stderrTail, reason?}`;
   - trả `{stdout, stderr, exitCode, durationMs, stdoutBytes}`; **không** ghi `rssPeakKb` (không có cách đo di động; contract §2.2 cho phép vắng).
4. Thêm `perf` thu thập: runner ghi `{tool, command: <verb hằng whitelist>, ms, stdoutBytes}` vào bộ gom trong `ctx` (task 08 dựng `perf` từ đây).

## Kiểm thử

Binary giả = script Node trong thư mục tạm đưa vào `config.toolPath` của test (không cần GitNexus thật).
`codeintel-tool-runner.test.ts`: (a) in 5 MB rồi `process.exit(0)` ngay -> nhận đủ 5 MB qua tệp (mô phỏng cụt pipe); (b) tệp tạm bị xoá ở thành công/lỗi/timeout/huỷ, thư mục mode `0700`; (c) > 16 MiB -> `OUTPUT_TOO_LARGE`, tiến trình bị kill; (d) treo -> `TIMEOUT`, `SIGKILL` sau `killGraceMs`; (e) con của con bị diệt; (f) 4 lệnh đồng thời -> tối đa 3; (g) argv ghi lại bởi script khớp whitelist (`-r` cuối); (h) env con không có `GITHUB_TOKEN`.
`codeintel-tool-output-classification.test.ts`: `{"error":"Write operations (CREATE…) are not allowed…"}`, `{"error":"Table Nope does not exist"}`, `{"error":"Symbol 'X' not found"}`, JSON cụt, `[]`, `{"markdown":…,"row_count":n}`; CodeGraph `\u001b[36mℹ Symbol "X" not found`, `✗ CodeGraph not initialized in /tmp`, `{"initialized":false}` (hợp lệ, không phải lỗi).
`codeintel-secret-redaction.test.ts`: canary `ghp_…`, `AKIA…`, `sk-…`, JWT, PEM, `https://u:p@h/` bị che; `$HOME` -> `~`; ANSI bị bỏ; chuỗi > 2 KiB bị cắt đuôi.

Lệnh: `pnpm exec vitest run src/relay/codeintel-tool-runner.test.ts src/relay/codeintel-tool-output-classification.test.ts src/relay/codeintel-secret-redaction.test.ts`

Re-verify thủ công, một lần (chỉ lệnh đọc), ghi kết quả vào PR: `gitnexus cypher "MATCH (n) RETURN n.id LIMIT 3000" -r /opt/repos/orca > /tmp/x.json` so với qua `spawn` pipe; thứ tự đối số `cypher <query> -r <path>` có chạy không.

## Tiêu chí hoàn thành

- [ ] GitNexus 3 000 hàng (> 256 KB) không cụt; `row_count` khớp (đo ở task SOL-002-04).
- [ ] Tệp tạm xoá ở mọi nhánh; thư mục `0700`, tệp `0600`.
- [ ] `{"error"}` exit 0 và text ANSI của CodeGraph được quy đúng mã.
- [ ] `stderrTail` đã che; không có `$HOME`/bí mật canary.
- [ ] `TestSpawnNeverUsesShell`: không `shell:true`, không `exec(`, mọi spawn đi qua `runToolCommand`.

## Rủi ro và lưu ý

- Nguyên nhân cụt stdout của GitNexus chưa rõ (có thể đổi theo phiên bản Node/GitNexus): fixture vàng (AG-CV-SOL-070) phải có ca lớn.
- `os.tmpdir()` dùng chung: tên thư mục theo `process.getuid?.()` (Windows không có; chặn Windows nên không áp dụng).
- Tệp tạm có thể chứa mã nguồn/đường dẫn: xoá sớm, không log nội dung.
