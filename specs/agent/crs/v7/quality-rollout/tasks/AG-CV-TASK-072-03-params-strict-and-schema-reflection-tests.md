# AG-CV-TASK-072-03: Tham số nghiêm ngặt và phản chiếu schema

**From Solution:** [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/security-params-and-schema.test.ts` (mới)
**Depends on:** 072-01; AG-CV-SOL-001, 081
**Status:** [ ] TODO

## Context

Agent-rpc §2.1: tham số lạ → `-32602 CODEINTEL_INVALID_PARAMS` (`data.field`), ngoại lệ `_trace`; không bao giờ nhận `args, argv, command, cmd, cwd, env, repo, cypher, shell, timeout, tool`; chuỗi 1..512, không NUL/điều khiển, không bắt đầu `-` (NFKC); proto-map §8.3 mục 5 (test phản chiếu schema `validate`); PQ-21 (16 method `codeintel.*` + 6 `quality.*`, `codeintel.node` không công khai).
Hợp đồng chưa nêu nơi xuất bảng method/schema (solution câu hỏi 3).
Test viết theo hợp đồng; mã bị test thuộc AG-CV-SOL-001/002/003/004/081 (chưa tồn tại): import qua hằng đường dẫn ở đầu tệp. Chưa chạy.

## Việc cần làm

1. `TestParamsStrict` lặp theo `CODEINTEL_METHODS`/`QUALITY_METHODS`: khoá lạ, kiểu sai, thiếu `workspaceRoot`, chuỗi 513 ký tự, NUL.
2. Phản chiếu schema: không khoá cấm; nếu 001 không xuất schema duyệt được, thêm yêu cầu xuất `CODEINTEL_METHOD_SCHEMAS`.
3. `TestPublicMethodSetMatchesContract`: tập method bằng danh sách hợp đồng; không `codeintel.node`.
4. Không `data`/`error.data` nào chứa `args/argv/env/cwd`.

## Kiểm thử

- Như mục 2.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Khoá cấm không xuất hiện trong schema nào; method lạ → đỏ.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Phụ thuộc vào việc 001 xuất bảng method.
