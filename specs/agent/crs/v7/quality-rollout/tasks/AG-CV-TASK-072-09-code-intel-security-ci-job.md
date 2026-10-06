# AG-CV-TASK-072-09: Job CI `code-intel-security` và bước khẳng định số test

**From Solution:** [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `.github/workflows/code-intel-security.yml` (mới) hoặc job trong `code-intel-contract.yml`; sửa `agent/vitest.code-intel-contract.config.ts` (include `security-*.test.ts`)
**Depends on:** 072-02 đến 072-08; AG-CV-TASK-070-09
**Status:** [ ] TODO

## Context

CR-072 §2.11: job chặn PR; tầng live đêm cho ca Cypher độc trên công cụ thật. Hạ tầng workflow/vitest do AG-CV-TASK-070-09 dựng; BE-CV-TASK-072-09 có workflow bảo mật riêng của Go (tên khác, tránh trùng).

## Việc cần làm

1. Mở rộng `include` vitest hợp đồng; thêm job (hoặc tệp) chạy toàn bộ `security-*.test.ts`.
2. Bước khẳng định: `vitest list` ≥ N test security (N đặt trong workflow, sửa khi thêm).
3. Job đêm (không chặn): ca Cypher độc trên `gitnexus` thật (dùng hạ tầng live của 070).
4. Ghi ngắn danh sách rủi ro còn lại (solution mục 6) vào mô tả PR; tài liệu threat model nằm ở BE-CV-TASK-072-09.

## Kiểm thử

- PR thử cố ý làm đỏ một vector → job đỏ (CHƯA CHẠY).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Job chặn PR chạy; số test được khẳng định.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Thời gian chạy chưa đo (SIGKILL grace 5 s, 20 MiB stdout).
