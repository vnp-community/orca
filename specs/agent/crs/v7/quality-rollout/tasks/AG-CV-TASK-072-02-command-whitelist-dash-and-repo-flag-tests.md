# AG-CV-TASK-072-02: Whitelist đóng, lệnh cấm, giá trị bắt đầu `-`, cờ `-r`/`-p`, reindex `--index-only`

**From Solution:** [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/security-command-whitelist.test.ts` (mới)
**Depends on:** 072-01; AG-CV-SOL-001, 002, 003, 004
**Status:** [ ] TODO

## Context

Agent-rpc §9.1 (danh sách chính xác), §2.4 (danh sách cấm: `analyze, clean, remove, uninstall, publish, setup, index, init, uninit, sync, serve, mcp, wiki, group, daemon, unlock, install, upgrade, telemetry, eval-server, check`; `check --cycles` chỉ cho `structuralFacts kind:"cycles"`; `analyze` luôn `--index-only`).
Đã tái hiện bởi CR-072 (chưa chạy lại): `gitnexus impact -r orca handleInvoke "--repo=vnp-workplace"` trả repo khác; `--` trước tham số vị trí bị từ chối "too many arguments" với `impact` (các lệnh khác chưa kiểm).
Test viết theo hợp đồng; mã bị test thuộc AG-CV-SOL-001/002/003/004/081 (chưa tồn tại): import qua hằng đường dẫn ở đầu tệp. Chưa chạy.

## Việc cần làm

1. `TestWhitelistIsClosed`: duyệt bảng kiểu lệnh (`GitNexusCommand`, `CodeGraphCommand`) với tham số mẫu; tập `argv[0]` == hợp đồng; `check` chỉ cùng `--cycles`. Thêm lệnh mà quên test → đỏ.
2. `TestNoForbiddenSubcommand`: mọi method × tham số xấu → argv không chứa lệnh cấm ngoài `codeintel-reindex-commands`.
3. `TestUserValuesNeverStartWithDash`: `"-r"`, `"--repo=vnp-workplace"`, `"-rvnp-workplace"`, U+FF0D, U+2212, khoảng trắng đầu → `INVALID_PARAMS` và `calls.length===0`; giá trị hợp lệ đứng sau `--` khi áp dụng.
4. `TestRepoFlagIsLast`: `-r <registryPath tuyệt đối>`/`-p` do agent đặt; không phần tử argv nào do người dùng quyết định ngoài vị trí sau `--`.
5. `TestReindexArgvIsIndexOnly`: mọi tổ hợp `mode/tools/workers` → `gitnexus analyze` có `--index-only`.
6. `TestSpawnNeverUsesShell`: `options.shell` luôn `false`/`undefined`.

## Kiểm thử

- Như mục 2; dùng `it.each` cho vector.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Bảy bất biến xanh; ca `--repo=`/`-r…` bị từ chối trước spawn.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Hành vi `--` chưa kiểm cho từng subcommand: nếu một lệnh không hỗ trợ `--`, từ chối mọi giá trị người dùng bắt đầu bằng `-` vẫn đủ.
