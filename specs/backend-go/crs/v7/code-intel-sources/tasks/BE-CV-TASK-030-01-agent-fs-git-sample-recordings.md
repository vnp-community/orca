# BE-CV-TASK-030-01: Re-verify và ghi mẫu JSON-RPC thật của `fs.*`/`git.*` Part A làm fixture

**From Solution:** BE-CV-SOL-030-repo-file-access-gateway
**Priority:** P0
**Service:** `code-intel-service` (fixture) / đọc `agent/`
**File:** `backend-go/services/code-intel-service/internal/adapter/agentrepofs/testdata/agent-rpc/*.json` (mới); `.../testdata/agent-rpc/README.md` (mới, ghi nguồn từng mẫu, ngày, commit)
**Depends on:** — (chỉ cần thư mục service của BE-CV-TASK-010-*; nếu chưa có, tạo thư mục tạm và dời khi có)
**Status:** [ ] TODO

---

## Context

CR-CV-030 và hợp đồng đánh dấu "chưa kiểm chứng" ba điểm: hình dạng `git.status.entries`, đường đi của lỗi agent (phong bì nhúng trong `result_json` hay `JSONRPCError`), và hành vi `git.exec` với `-z`. Task này **chỉ đọc mã `agent/` và chạy agent trên repo mẫu cục bộ nếu có** (không sửa agent, không chạy `analyze`), rồi chốt fixture để mọi task sau dùng chung.

## Việc cần làm

1. Đọc lại và trích dòng: `agent/src/relay/fs-agent-extensions.ts` (`handleFsReadDir`, `readDirRecursive`), `fs-handler-file-read.ts` (trần 10 MiB văn bản, `isBinary`), `agent-git-handler.ts` (whitelist, `SHELL_METACHARACTERS`), `agent-rpc-dispatch-git.ts`, `git-handler-status-ops.ts`, `git-status-output-parser.ts`. Ghi vào `README.md` của fixture mọi chỗ khác SOL-030 mục 1.
2. Viết tay (từ mã, đánh dấu `"_source":"hand-written-from-code"`) các mẫu: `fs.readDir` (file thường có `size`, symlink không `size`, thư mục), `fs.readFile` (văn bản, `isBinary:true`, lỗi `FILE_TOO_LARGE`), `git.exec` (`rev-parse`, `show`, `log -z`), `git.status` (có `untracked`, `didHitLimit`), `git.branchCompare` (`ready`, `no-merge-base`), `git.branchDiff` (hai phía, binary), và **hai dạng lỗi**: JSON-RPC `error` cấp ngoài và phong bì `{jsonrpc,id,error:{code,message}}` nhúng trong `result`, `code` là **số** (`-33003`, `-32601`).
3. Nếu có agent chạy được cục bộ: ghi lại phản hồi thật (thay `_source` thành `recorded`, che đường dẫn tuyệt đối thành `/work/repo`) cho từng mẫu trên, và ghi vào README kết quả: lỗi `fs.readFile` của file 11 MiB trả dạng nào qua `Client.Exec` (đọc thêm `devserveragent/session.go` `callWithTimeout` để xác định lỗi cấp ngoài có thành `error` Go hay không).
4. Liệt kê ở README các điểm vẫn chưa kiểm chứng để SOL-030 mục 7 R1 tham chiếu.

## Kiểm thử

- Test dùng fixture ở các task sau (`go test ./services/code-intel-service/internal/adapter/agentrepofs/...`); ở task này chỉ cần mọi file JSON hợp lệ: `python3 -c 'import json,glob;[json.load(open(f)) for f in glob.glob("backend-go/services/code-intel-service/internal/adapter/agentrepofs/testdata/agent-rpc/*.json")]'` (chạy khi đã có file).

## Tiêu chí hoàn thành

- [ ] Có mẫu cho mọi method trong bảng SOL-030 mục 2.C, kèm cả hai dạng lỗi với `code` số.
- [ ] README ghi rõ mẫu nào là "viết tay từ mã" và mẫu nào "ghi từ agent thật".
- [ ] Không có đường dẫn tuyệt đối thật hay nội dung bí mật trong fixture.

## Rủi ro và lưu ý

- Không ghi mẫu từ máy dev thật của người dùng: có thể chứa đường dẫn/tên người. Dùng repo mẫu tạm.
- Mẫu viết tay có thể lệch agent thật; `BE-CV-SOL-070-collector-golden-contract` sẽ thay bằng bản ghi thật.
