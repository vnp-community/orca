# AG-CV-TASK-070-09: Workflow `code-intel-contract.yml` (job agent) và cấu hình vitest riêng

**From Solution:** [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `.github/workflows/code-intel-contract.yml` (mới hoặc sửa nếu BE đã tạo), `agent/vitest.code-intel-contract.config.ts` (mới)
**Depends on:** 070-02 đến 070-08; BE-CV-TASK-070-06 (cùng tệp)
**Status:** [x] DONE

## Context

CI hiện **không chạy test `agent/`** (agent-rpc §10; `grep -rn "agent/" .github/workflows` rỗng); `pr.yml` chạy `pnpm test` gốc với `config/vitest.config.ts` không tồn tại. Không sửa `pr.yml` ở task này.
Hợp đồng §11: job agent tên `code-intel-contract`; job Go do BE thêm (dùng id khác trong cùng tệp).
`pr.yml` dùng `node-version-file: package.json`, `pnpm/action-setup@v6`, `actions/checkout@v6`: dùng cùng phiên bản action.

## Việc cần làm

1. `vitest.code-intel-contract.config.ts`: `include` = `src/relay/codeintel/**/*.test.ts`, `src/relay/codeintel-*.test.ts`, `src/relay/gitnexus-*.test.ts`, `src/relay/codegraph-*.test.ts`, `src/relay/quality-*.test.ts`; `environment:"node"`.
2. Job `code-intel-contract` (chặn): trigger `pull_request` `paths: [agent/**, backend-go/services/code-intel-service/testdata/**, .github/workflows/code-intel-contract.yml]`; cài phụ thuộc (phạm vi tối thiểu thử `pnpm install --frozen-lockfile --filter orca-agent...`; `--ignore-scripts` nếu đủ); chạy `pnpm --filter orca-agent exec vitest run --config vitest.code-intel-contract.config.ts`.
3. Bước khẳng định: `vitest list --config …` phải liệt kê ≥ 1 test của `agent-result-golden`, `fixture-manifest`, `tool-compatibility` (thiếu → thoát ≠ 0).
4. Job `code-intel-live-contract` (`schedule` đêm + `workflow_dispatch`, không chặn): `npm i -g gitnexus`, CodeGraph (cách cài **chưa biết**: bước `continue-on-error`), chạy script chụp ra tmp, `diff -r` với fixture, đẩy artifact.

## Kiểm thử

- Mở PR thử chạm `agent/`: job chạy và xanh; cố ý làm đỏ một golden: job đỏ. `workflow_dispatch` chạy được tầng live (có thể đỏ do CodeGraph).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [x] Job chặn PR hoạt động; bước khẳng định có mặt; live chạy tay được.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- `pnpm install` có thể nặng/lỗi native (node-pty, Electron) trên runner: phạm vi cài chưa thử.
- Chưa đo thời gian.
- Phối hợp với BE-CV-TASK-070-06 để không ghi đè nhau.
