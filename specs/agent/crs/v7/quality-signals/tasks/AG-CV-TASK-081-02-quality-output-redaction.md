# AG-CV-TASK-081-02: Che secret và đường dẫn tuyệt đối trong log/message (`quality-output-redaction.ts`)

**From Solution:** [AG-CV-SOL-081-quality-runner-core](../solutions/AG-CV-SOL-081-quality-runner-core.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-output-redaction.ts` (mới), `.test.ts` (mới)
**Depends on:** không
**Status:** [ ] TODO

## Context

Hợp đồng §9.4 và CR-081 2.8: message, log, `stderrTail` đi qua bộ che trước khi lưu/gửi. SOL-082 (pipeline) và SOL-091 dùng lại.

## Việc cần làm

1. `export function createRedactor(opts: { repoRoot: string; home: string; tmpRoot: string; secretEnvValues: string[] }): (text: string) => string`.
2. Thay: giá trị chính xác (độ dài ≥ 8) của biến env khớp `SECRET_ENV_NAME_PATTERN` bằng `***`; mẫu token `gh[pousr]_[A-Za-z0-9]{20,}`, `github_pat_…`, `AKIA[0-9A-Z]{16}`, `sk-[A-Za-z0-9_-]{20,}`, JWT `eyJ…\.…\.…`, khối `-----BEGIN … PRIVATE KEY-----…END`, `scheme://user:pass@`; đường dẫn tuyệt đối `repoRoot` → `<repo>`, `home` → `~`, `tmpRoot` → `<tmp>`.
3. `export function secretEnvValuesFrom(env: NodeJS.ProcessEnv): string[]` lấy giá trị cần che (không log chúng).
4. Chuẩn hoá `\` → `/` cho so khớp đường dẫn trên mọi nền.
5. Hàm `redactTail(text, maxBytes)` cắt ở ranh giới UTF-8 rồi che.

## Kiểm thử

Bảng ca: mỗi mẫu token, nhiều lần trong một dòng, giá trị env ngắn (<8 không che), đường dẫn có khoảng trắng, CRLF, chuỗi 1 MiB (thời gian tuyến tính). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-output-redaction.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không còn token/đường dẫn tuyệt đối trong đầu ra ở mọi ca.
- [ ] Regex không backtracking thảm hoạ (test chuỗi 1 MiB < 200 ms).

## Rủi ro

Che chỉ ở mức tối thiểu, không đảm bảo bắt hết secret tự do (CR-081 2.8).
