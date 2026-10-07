# AG-CV-TASK-082-03: Fingerprint v1 và chuẩn hoá `anchor`, `normMessage`

**From Solution:** [AG-CV-SOL-082-quality-parsers-and-fingerprint](../solutions/AG-CV-SOL-082-quality-parsers-and-fingerprint.md) mục 5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-finding-fingerprint.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-082-02
**Status:** [x] DONE

## Context

Hợp đồng §5.5 công thức; CR-082 2.2. Dòng nguồn chỉ băm.

## Việc cần làm

1. `normalizeAnchor(line: string | null): string`, `normalizeMessage(msg, ctx{repoRoot,home,tmp}): string` theo 5.3.
2. `assignOccurrences(items)` (nhóm theo `(tool,ruleId,file,anchor,normMessage)` sắp `(line,column)`).
3. `computeFingerprint(item, occurrence): { fingerprint: string; fpVersion: 1 }` = `sha256(["v1",tool,ruleId,file,anchor,normMessage,String(occurrence)].join("\0"))` hex 32 ký tự đầu.
4. `createSourceLineReader(repoRoot)`: `realpath` trong repo, ≤ 2 MiB, bỏ nhị phân (NUL trong 8 KiB đầu), cache LRU 64 tệp; trả `null` khi không đọc được.
5. Hàm xuất vector kiểm thử cố định (để sau này đối chiếu chéo nếu backend tái tính).

## Kiểm thử

Bảng tính chất: chèn 5 dòng trống phía trên giữ; đổi thụt đầu dòng giữ; sửa dòng đổi; hai phát hiện giống hệt khác nhau; đổi tên tệp đổi; `toolVersion` không ảnh hưởng; đổi `tool` đổi; khoá trước băm không chứa chuỗi số dòng; vector vàng (3 đầu vào → 3 hex cố định). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-finding-fingerprint.test.ts`.

## Tiêu chí hoàn thành

- [x] Mọi tính chất ở bảng có test; xác định qua nhiều lần chạy.

## Rủi ro

NFC/khoảng trắng Unicode (NBSP) có thể khác giữa công cụ: thêm ca.
