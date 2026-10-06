# AG-CV-TASK-072-01: Bộ ghi `spawn` (test-only) và quét tĩnh điểm spawn

**From Solution:** [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/spawn-recorder.ts` + `.test.ts` (mới), `agent/src/relay/codeintel/security-spawn-sites.test.ts` (mới)
**Depends on:** AG-CV-SOL-001
**Status:** [ ] TODO

## Context

Agent-rpc §9.1 `TestSpawnNeverUsesShell`. Khuôn mock: `agent-print-mode-exec.test.ts` (`FakeChild`, `vi.mock("node:child_process")`) và `agent-rpc-dispatch-misc.test.ts`.
`runToolCommand` cũ có `shell:false` nhưng không giới hạn dung lượng/không SIGKILL: không dùng làm mẫu cho runner mới.
Test viết theo hợp đồng; mã bị test thuộc AG-CV-SOL-001/002/003/004/081 (chưa tồn tại): import qua hằng đường dẫn ở đầu tệp. Chưa chạy.

## Việc cần làm

1. `installSpawnRecorder(vi, opts)` ghi `file, argv, options` của `spawn/execFile/execFileSync/exec/spawnSync`, không chạy gì.
2. `security-spawn-sites.test.ts` (`TestSpawnCallsOnlyInRunner`): đọc văn bản các tệp codeintel không-test; `child_process` chỉ ở danh sách cho phép (runner, reindex commands, git probe, quality runner); cấm `shell: true`, `exec(`, `execSync(`, chuỗi lệnh ghép.
3. Đảm bảo không import ngược `spawn-recorder` từ mã sản phẩm (test quét import).

## Kiểm thử

- `records argv and options without spawning`
- `flags an unlisted file that imports child_process`
- `no non-test file uses shell:true or exec(`

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Hai tệp test xanh.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Danh sách cho phép phải sửa khi thêm tệp spawn (ý định).
