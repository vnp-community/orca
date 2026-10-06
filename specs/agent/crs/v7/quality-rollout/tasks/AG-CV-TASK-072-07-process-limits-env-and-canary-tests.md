# AG-CV-TASK-072-07: Giới hạn stdout/timeout cây tiến trình, lọc env, canary secret

**From Solution:** [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/security-process-limits-and-env.test.ts` (mới), `agent/src/relay/codeintel/__fixtures__/canary-secrets.json` (mới)
**Depends on:** 072-01, 072-05; AG-CV-SOL-001, 081; AG-CV-TASK-071-04 (`fake-codeintel-cli`)
**Status:** [ ] TODO

## Context

Agent-rpc §2.3: stdout ≤ 16 MiB (kill + `OUTPUT_TOO_LARGE`), JSON ≤ 8 MiB (cắt + `truncated`, vẫn vượt → lỗi), timeout CLI 20 s SIGTERM rồi SIGKILL sau 5000 ms. CR-072 nói 8 MiB cho stdout: hợp đồng thắng (solution Correction 1).
Env: `quality.*` bắt đầu từ rỗng, loại biến khớp `/(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION)/i`. Hợp đồng §2.4 nói `codeintel.*` dùng `toolEnv` (toàn `process.env`), §10 nói phải xử lý: **mâu thuẫn**; test theo hướng chặt (solution Correction 5), đánh dấu chờ duyệt.
Canary §9.4: không secret trong kết quả/log/cache; `.env*`, `*.pem`, `*.key`, `id_rsa*` không bao giờ trả.
Test viết theo hợp đồng; mã bị test thuộc AG-CV-SOL-001/002/003/004/081 (chưa tồn tại): import qua hằng đường dẫn ở đầu tệp. Chưa chạy.

## Việc cần làm

1. CLI giả: in 20 MiB → dừng ở 16 MiB, `OUTPUT_TOO_LARGE` có `bytes`,`limit`; treo + sinh tiến trình cháu → hết hạn, không pid nào sống (`process.kill(pid,0)`), `CODEINTEL_TIMEOUT`.
2. Env con của `codeintel.*` và `quality.*` không có `ANTHROPIC_API_KEY, GITHUB_TOKEN, GH_TOKEN, AGENT_TOKEN, ORCA_*, SSH_AUTH_SOCK, AWS_*, MY_SECRET_THING` (kể cả khi profile xin `env.allowExtra`); cho phép `PATH, HOME, NO_COLOR=1`.
3. Canary: tệp cấu hình có `CANARY-DB-PASS-1`, `CANARY-REDIS-2`, `CANARY-API-3`, PEM giả, `ghp_`/`AKIA` giả, `https://user:CANARY-URL-4@host`; CLI giả in chúng ra stdout/stderr; quét `stderrTail`, `message`, `error.data`, log bắt được, kết quả `symbol` → không canary; không đường dẫn tuyệt đối ngoài `workspaceRoot`.
4. Kết quả `symbol` thuộc tệp nhạy cảm bị từ chối/che.

## Kiểm thử

- Như mục 2; RSS test (tuỳ chọn, ngưỡng rộng giả định) cho nhánh 20 MiB.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Tất cả xanh; nếu quyết định giữ `toolEnv` cho `codeintel.*`, sửa test và ghi lý do.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Giết cây tiến trình cần `detached` + nhóm tiến trình (POSIX); WSL/macOS chưa kiểm.
- Che regex không phủ mọi dạng; chỉ mục công cụ có thể chứa secret (ngoài kiểm soát).
