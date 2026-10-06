# AG-CV-TASK-070-02: Schema `MANIFEST.json`, ngân sách kích thước, quét rò rỉ

**From Solution:** [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/fixture-manifest.ts` (mới), `agent/src/relay/codeintel/fixture-manifest.test.ts` (mới)
**Depends on:** 070-01
**Status:** [ ] TODO

## Context

CR-070 §2.3, D8: không lưu `fileHashes`, token, đường dẫn thật. Ngân sách ≤ 20 KiB/tệp, ≤ 300 KiB/thư mục phiên bản (giả định).
Mẫu token dùng chung với bộ che stderr của AG-CV-SOL-001 (agent-rpc §9.4: `gh[pousr]_…`, `github_pat_…`, `AKIA…`, `sk-…`, JWT, PEM, `scheme://user:pass@`): nếu 001 xuất danh sách mẫu thì import, không chép.

## Việc cần làm

1. Cài `FixtureManifest`, `FIXTURE_BUDGET`, `LeakFinding`, `scanTextForLeaks`, `loadFixtureManifest`, `listFixtureVersionDirs` đúng chữ ký ở solution mục 2.3.
2. `loadFixtureManifest` kiểm schema thủ công (không thêm zod mới ở đây; `zod` đã có trong `agent/package.json` nên **được dùng** nếu muốn, ghi quyết định) và ném lỗi có tên tệp.
3. Quét: `/home/`, `/opt/`, `/Users/`, `C:\`, khoá `fileHashes`/`cacheKeys`, mẫu token, PEM.

## Kiểm thử

- `TestFixtureBudget`: tệp/thư mục vượt ngân sách (dựng bằng thư mục tạm) → lỗi.
- `TestFixtureHasNoLeaks`: mỗi mẫu rò rỉ phát hiện đúng `rule`; chuỗi sạch không báo.
- `TestManifestHashes`: sha256/bytes khớp; tệp mồ côi hoặc thiếu → đỏ.
- Chạy thật trên mọi thư mục `listFixtureVersionDirs()` (rỗng lúc đầu thì test bỏ qua có ghi chú, bật khi 070-03 xong).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Ba test xanh; thêm tệp 21 KiB vào thư mục tạm làm đỏ.
- [ ] Hàm quét tái dùng được bởi script chụp (task 03) và `mini-repo-shape`.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Mẫu quét `/opt/` có thể báo nhầm với nội dung hợp lệ của mini-repo: cho phép danh sách ngoại lệ có lý do trong test, không trong hàm.
