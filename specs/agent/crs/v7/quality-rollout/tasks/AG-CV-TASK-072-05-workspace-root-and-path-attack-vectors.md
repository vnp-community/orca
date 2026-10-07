# AG-CV-TASK-072-05: `workspaceRoot` và đường dẫn: bộ vector dùng chung, symlink, thứ tự kiểm

**From Solution:** [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/security-workspace-root-and-paths.test.ts` (mới), `agent/src/relay/codeintel/__fixtures__/path-attack-vectors.json` (mới)
**Depends on:** 072-01; AG-CV-SOL-001
**Status:** [x] DONE

## Context

Agent-rpc §2.1 (tuyệt đối, không NUL, ≤ 4096, `realpath`, gốc git worktree, `ORCA_CODEINTEL_ALLOWED_ROOTS`; đường dẫn tham số tương đối, không `..`, không `\`), §7.4 (`workspaceRoot:"../../etc"` → `-33002` **trước** `expandTilde`), §9.3. `expandTilde` (`agent/src/relay/context.ts`) mở rộng `~` về HOME.
Bộ vector này được BE-CV-TASK-072-01 đọc từ cây `agent/`: giữ JSON thuần, `\u` cho ký tự điều khiển; dạng `{input, field, expect, code, note}` (solution 2.6).
Test viết theo hợp đồng; mã bị test thuộc AG-CV-SOL-001/002/003/004/081 (chưa tồn tại): import qua hằng đường dẫn ở đầu tệp. Chưa chạy.

## Việc cần làm

1. Viết JSON vector (ca ở solution 2.6, gồm `~`, `~/.ssh`, `..`, `%2e%2e`, `%252e%252e`, `..%c0%af`, `．．`, `\`, NUL, >4096, >1024, UTF-8 hỏng, UNC/ổ đĩa Windows).
2. Test chạy vector qua validator; với root: spy `expandTilde`/`fs.realpath` để chứng minh thứ tự (từ chối trước).
3. Thư mục tạm: symlink root ra ngoài, symlink thư mục con, `/tmp/x/orca` vs `/tmp/x/orca-old` với `ALLOWED_ROOTS`, `/` cuối và `//`.
4. Khẳng định `spawn` nhận đường dẫn đã `realpath`.

## Kiểm thử

- Như mục 2.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel/security-workspace-root-and-paths.test.ts` (17 passed).

## Tiêu chí hoàn thành

- [x] Mọi vector `deny` bị từ chối đúng mã; `allow` được nhận.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Windows/UNC chỉ khẳng định từ chối; TOCTOU chỉ giảm thiểu.
- Phối hợp BE-CV-TASK-072-01 để cùng schema vector.
