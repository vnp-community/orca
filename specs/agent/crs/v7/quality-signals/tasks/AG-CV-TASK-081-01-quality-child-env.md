# AG-CV-TASK-081-01: Env tiến trình con từ rỗng: allowlist và deny pattern (`quality-child-env.ts`)

**From Solution:** [AG-CV-SOL-081-quality-runner-core](../solutions/AG-CV-SOL-081-quality-runner-core.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-child-env.ts` (mới), `agent/src/relay/quality-child-env.test.ts` (mới)
**Depends on:** không
**Status:** [x] DONE

## Context

Hợp đồng §2.4 "Quality" và README v7 mục 8 điểm 22: `config.toolEnv` (đã đọc `agent-config.ts:81-88`) là `{...process.env, ANTHROPIC_API_KEY, GITHUB_TOKEN, GH_TOKEN}`; tiến trình kiểm tra không được nhận nó. Env con xây từ rỗng.

## Việc cần làm

1. `export function buildQualityChildEnv(input: { sourceEnv: NodeJS.ProcessEnv; qualityToolPath: string; tmpDir: string; profileEnv: { set: Record<string,string>; allowExtra: string[] }; limits: { gomaxprocs: number; gomemlimitBytes?: number; nodeOldSpaceMb?: number }; nodeOptions?: string }): { env: NodeJS.ProcessEnv; rejectedExtra: string[] }` (`sourceEnv` = `process.env` của agent, tiêm để test; KHÔNG nhận `config.toolEnv`).
2. Chỉ sao chép các tên: `HOME, USER, LOGNAME, LANG, LC_ALL, TZ, SHELL`; `PATH = qualityToolPath`; `TMPDIR = tmpDir`; cố định `CI=1, NO_COLOR=1, FORCE_COLOR=0, TERM=dumb`; Go: `GOTOOLCHAIN=local`, `GOMAXPROCS`, `GOMEMLIMIT`, `GOFLAGS` (từ profile), `GOCACHE/GOPATH/GOMODCACHE` chỉ khi có ở `sourceEnv`; Node: `NODE_OPTIONS` (từ profile), `PNPM_HOME`, `npm_config_cache` nếu có.
3. `SECRET_ENV_NAME_PATTERN = /(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION)/i`; mọi tên khớp bị loại **kể cả** khi `profileEnv.allowExtra` xin (đưa vào `rejectedExtra`, caller log). Luôn loại `ORCA_*`, `AWS_*`, `GOOGLE_*`, `SSH_AUTH_SOCK`, `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `GH_TOKEN`, `AGENT_TOKEN`.
4. `profileEnv.set` cũng bị kiểm tên bằng pattern trên (cấu hình L2 không được nhét secret).
5. `NODE_OPTIONS` bị từ chối nếu chứa `--require`, `-r `, `--import`, `--loader` (tránh nạp mã tuỳ ý; ràng buộc thêm so với CR).

## Kiểm thử

Test với `sourceEnv` chứa `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `GH_TOKEN`, `AGENT_TOKEN`, `FOO_SECRET`, `ORCA_URL`, `AWS_ACCESS_KEY_ID`, `SSH_AUTH_SOCK`, `XDG_SESSION_ID`, `HOME`, `LANG`: chỉ `HOME`, `LANG` và biến cố định xuất hiện; `allowExtra:["MY_API_KEY","FOO"]` → `MY_API_KEY` bị từ chối, `FOO` được chép nếu có; `GOCACHE` chỉ khi đặt; `NODE_OPTIONS=--require x` bị loại. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-child-env.test.ts`.

## Tiêu chí hoàn thành

- [x] Không biến nào khớp pattern/danh sách cấm lọt vào `env`.
- [x] Hàm thuần, không đọc `process.env` trực tiếp; file < 300 dòng.

## Rủi ro

Allowlist có thể thiếu biến công cụ cần (vd `XDG_CACHE_HOME`, `GOPROXY`); thêm theo profile (`env.set`), không nới allowlist chung. Việc không truyền secret không ngăn mã test đọc `~/.ssh` (CR-081 2.10).
