# BE-CV-TASK-070-01: Bố cục `testdata/agent-results/`, `MANIFEST.json` và quét vệ sinh

**From Solution:** BE-CV-SOL-070
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/testdata/agent-results/**` (mới, do `AG-CV-SOL-070` sinh), `backend-go/services/code-intel-service/internal/contracttest/agent_results_manifest_test.go` (mới), `.../internal/contracttest/agent_results_hygiene_test.go` (mới)
**Depends on:** BE-CV-SOL-010 (module `code-intel-service`, cổng G0); `AG-CV-SOL-070` (script chụp ghi tệp)
**Status:** `[ ] TODO`

---

## Context

- Cổng G1 (`CONTRACT-codeintel-proto-and-data-map.md` §7.1) cần tệp vàng ở `testdata/agent-results/*.json`; bố cục chi tiết ở BE-CV-SOL-070 mục 5.1.
- Mẫu golden trong repo: `api-gateway/internal/adapter/mcpserver/tools/catalog_test.go` (cờ `-update`) và `testdata/tools_list.golden.json`. Ở đây **không** dùng `-update` phía Go (D2).
- Thư mục `internal/contracttest/` là tên đề xuất theo nội dung (không `helpers`/`utils`); gói chỉ có `_test.go`.

## Việc cần làm

1. Tạo cây thư mục theo SOL-070 mục 5.1 (nếu agent chưa sinh, đặt tệp tổng hợp tay có `"_synthetic": true` trong `MANIFEST.json`; xem SOL-070 Q1).
2. `MANIFEST.json`: `{ "contract": "agent-rpc §2.2", "tools": {"gitnexus":"1.6.9","codegraph":"1.4.1"}, "sourceCommit": "<sha mini-repo>", "files": { "<đường dẫn tương đối>": {"sha256": "...", "bytes": N} } }`.
3. `TestAgentResultsManifest`: duyệt thư mục, so `sha256`/`bytes` với manifest; từ chối tệp không có trong manifest và mục manifest không có tệp; ngân sách ≤ 20 KiB/tệp, ≤ 300 KiB/thư mục phiên bản.
4. `TestAgentResultsHygiene`: quét mọi tệp tìm `/home/`, `/opt/`, `C:\`, `-----BEGIN`, `ghp_`, `AKIA`, `fileHashes`, `CANARY-`, `Bearer `; mỗi lần khớp báo tên tệp và dòng (không in nội dung khớp).
5. Mỗi tệp vàng thành công phải parse được thành object có `sources`, `headCommit`, `stale`, `truncated`, `totalCount`, `data` (kiểm sơ bộ tại đây; kiểm sâu ở task 02, 03).

## Kiểm thử

- Hai test trên; thử tay: đổi một byte một tệp thì `TestAgentResultsManifest` đỏ, thêm chuỗi `/home/x` thì `TestAgentResultsHygiene` đỏ (ghi vào PR).
- Lệnh: `go test ./internal/contracttest/...`. Không cần Docker.

## Tiêu chí hoàn thành

- [ ] Manifest, ngân sách, vệ sinh xanh; không tệp mồ côi.
- [ ] Không có tệp nào lấy từ repo Orca thật.
- [ ] Không `max-lines` disable.

## Rủi ro và lưu ý

- Ngân sách kích thước là giả định chưa đo; nới bằng PR có lý do.
- Nếu `AG-CV-SOL-070` đổi bố cục, sửa SOL-070 mục 5.1 trước.
