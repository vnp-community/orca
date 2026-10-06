# AG-CV-TASK-072-08: Fuzz xác định không thêm phụ thuộc

**From Solution:** [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/security-seeded-fuzz.test.ts`, `agent/src/relay/codeintel/seeded-random.ts` (mới)
**Depends on:** 072-02 đến 072-05
**Status:** [ ] TODO

## Context

CR-072 §2.2 yêu cầu fuzz; câu hỏi mở 1 của CR (`fast-check`): giữ không phụ thuộc vì O12 chưa duyệt. Mẫu: `FuzzJSONRPCDecode` phía Go (khác ngôn ngữ).
Test viết theo hợp đồng; mã bị test thuộc AG-CV-SOL-001/002/003/004/081 (chưa tồn tại): import qua hằng đường dẫn ở đầu tệp. Chưa chạy.

## Việc cần làm

1. `seeded-random.ts` (mulberry32) với hạt cố định.
2. Sinh chuỗi: Unicode ngẫu nhiên, NUL, dài, khoảng trắng, `;`, `$()`, backtick, xuống dòng, U+FF0D/U+2212, tiền tố `-` sau NFKC.
3. Bất biến cho bộ dựng argv, validator tham số, bộ dựng Cypher, validator đường dẫn: từ chối có mã **hoặc** đầu ra thoả ngữ pháp; không ném ngoại lệ không bắt; thất bại in hạt.
4. `ORCA_FUZZ_ITERATIONS` (mới) tăng số vòng; mặc định nhỏ.

## Kiểm thử

- Chạy với ba hạt cố định; thủ công tăng vòng (CHƯA CHẠY).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Bất biến giữ; thời gian chạy mặc định thấp (chưa đo).
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Chưa đo thời gian; tách `*.slow.test.ts` nếu cần.
