# AG-CV-TASK-001-03: Whitelist lệnh có kiểu và môi trường tiến trình con

**From Solution:** [AG-CV-SOL-001-codeintel-agent-foundation](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) mục 2.4, 2.6
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-command-whitelist.ts` (mới), `agent/src/relay/codeintel-child-env.ts` (mới), `agent/src/relay/codeintel-command-whitelist.test.ts` (mới), `agent/src/relay/codeintel-child-env.test.ts` (mới)
**Depends on:** [01](./AG-CV-TASK-001-01-codeintel-errors-and-strict-params.md) (dùng `assertSafeClientString`)
**Status:** [x] DONE

## Context

D5 / contract §2.4, §9 mục 1: mọi lệnh là đối tượng có kiểu, không có API argv tự do; GitNexus luôn `-r <registryPath tuyệt đối>`, CodeGraph luôn `-p <projectPath>` (riêng `status` dùng đối số vị trí `codegraph status <projectPath> -j`, vì `status --help` không có `-p`, theo CR-001 2.3); giá trị client không bắt đầu `-`; danh sách cấm `analyze, clean, remove, uninstall, publish, setup, index, init, uninit, sync, serve, mcp, wiki, group, daemon, unlock, install, upgrade, telemetry, eval-server, check`. Đã tái hiện ở contract §9: `gitnexus impact -r orca handleInvoke "--repo=vnp-workplace"` trả repo khác, nên `-r` đặt cuối và giá trị người dùng đứng trước.

`config.toolEnv` hiện là `{...process.env, PATH, HOME, ANTHROPIC_API_KEY, GITHUB_TOKEN, GH_TOKEN}` (`agent-config.ts:81-88`); tiến trình con codeintel không cần bí mật nào.

## Việc cần làm

1. Kiểu `GitNexusCommand`, `CodeGraphCommand` đúng solution 2.4 (không thêm verb ngoài danh sách; `affected` nhận `files: string[]`, mỗi phần tử qua `assertRelativeRepoPath`).
2. `buildGitNexusArgv(cmd, registryPath)` và `buildCodeGraphArgv(cmd, projectPath)` trả `string[]` duy nhất; `-r`/`-p` luôn phần tử cuối; cờ JSON `-j` cho mọi lệnh CodeGraph trừ `node`; `limit`/`depth` chuyển bằng `String(int)` sau kiểm biên. `GitNexus cypher` truyền câu truy vấn là một đối số riêng (khuôn `['cypher', query, '-r', path]`; vị trí chính xác của `query` so với `-l` **chưa kiểm chứng**, ghi fixture ở task 05).
3. Xuất `GITNEXUS_VERBS`, `CODEGRAPH_VERBS` (tập đóng) và `FORBIDDEN_SUBCOMMANDS`; không xuất hàm nhận `string[]` argv.
4. `codeintel-child-env.ts`: `buildCodeIntelChildEnv(config)` = `config.toolEnv` bỏ mọi khoá khớp `/(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION)/i` cộng `SSH_AUTH_SOCK`, tiền tố `AWS_`, `GOOGLE_`, `ORCA_`; thêm `NO_COLOR=1`; luôn giữ `PATH` (`config.toolPath`) và `HOME`. Chú thích ngắn nêu lý do (con không cần bí mật) và rằng đây là chỗ duy nhất đổi nếu chủ hợp đồng yêu cầu `toolEnv` nguyên văn.
5. Không đổi `tool gitnexus`/`codegraph` ở `agent-tool-registry.ts` (task 04 lo).

## Kiểm thử

`codeintel-command-whitelist.test.ts`: snapshot bảng "verb -> argv" cho từng verb (`TestWhitelistIsClosed`: tập verb là tập con của tập hợp đồng §9); `TestNoForbiddenSubcommand` (quét mọi argv sinh ra, phần tử đầu không thuộc danh sách cấm); `TestUserValuesNeverStartWithDash` (uid, name, file, search, symbol, kind, filter, files[]); `TestRepoFlagIsLast` (`--repo=x`, `-rx`, `-r` làm giá trị đều bị từ chối trước khi dựng argv); quét nguồn mọi `codeintel-*.ts` (trừ `codeintel-reindex-*.ts`) không chứa literal `'analyze'|'clean'|'remove'|'sync'`; không có `shell: true`/`exec(` (`TestSpawnNeverUsesShell` sơ bộ, hoàn tất ở task 05).
`codeintel-child-env.test.ts`: `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `GH_TOKEN`, `AWS_SECRET_ACCESS_KEY`, `SSH_AUTH_SOCK`, `ORCA_FOO` bị loại; `PATH`, `HOME`, `LANG` còn; `NO_COLOR === '1'`; không sửa đổi đối tượng `config.toolEnv` gốc.

Lệnh: `pnpm exec vitest run src/relay/codeintel-command-whitelist.test.ts src/relay/codeintel-child-env.test.ts`

## Tiêu chí hoàn thành

- [x] Không có đường nào sinh argv ngoài hai hàm `build*Argv`.
- [x] `-r`/`-p` luôn cuối; giá trị bắt đầu `-` bị từ chối.
- [x] Env con không chứa biến khớp mẫu bí mật; `config.toolEnv` không bị sửa.

## Rủi ro và lưu ý

- Chưa kiểm chứng việc hai CLI hỗ trợ `--` kết thúc tuỳ chọn: dùng quy tắc "không bắt đầu `-`" thay vì dựa vào `--`.
- Lọc env: nếu GitNexus/CodeGraph cần biến khớp mẫu (ví dụ proxy có `AUTH`), công cụ sẽ lỗi; chưa biết. Task 08 ghi bước kiểm trên dev server thật. Cần chủ hợp đồng xác nhận (solution mục 8 câu 3).
