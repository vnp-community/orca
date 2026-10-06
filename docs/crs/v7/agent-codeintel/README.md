# Agent Code Intelligence — Change Requests (v7)

> Nhóm RPC `codeintel.*` trên agent TypeScript: lấy dữ liệu từ GitNexus và CodeGraph đang cài trên dev server và trả về dạng JSON gọn, có giới hạn. Bối cảnh, quyết định D1-D7, mặc định O1-O8 và hợp đồng chung ở [README v7](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-CV-001](./CR-CV-001-codeintel-agent-foundation.md) | Agent chưa có bề mặt hẹp cho code-intelligence: chỉ có tool CLI thô (`args` tự do), không có phân giải repo, giới hạn, mã lỗi | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-002](./CR-CV-002-gitnexus-extraction.md) | Lấy dữ liệu GitNexus an toàn: Cypher chỉ-đọc, parser markdown, `ambiguous`, giới hạn, cache ngắn hạn | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-003](./CR-CV-003-codegraph-extraction.md) | Lấy dữ liệu CodeGraph: `--json`, ánh xạ id/kind, SQLite chỉ-đọc tuỳ chọn | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-004](./CR-CV-004-codeintel-reindex-and-index-notifications.md) | Làm mới chỉ mục chạy nền có tiến trình, chặn đồng thời, huỷ; thông báo `codeintel.indexChanged` | 🟠 P1 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-005](./CR-CV-005-codeintel-detect-changes.md) | Ánh xạ diff theo merge-base → symbol → luồng bị ảnh hưởng (CLI `detect-changes` chỉ in 15 symbol) | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-006](./CR-CV-006-codeintel-relay-ssh-part-b.md) | Đăng ký `codeintel.*` trên Part B (`RelayDispatcher`), khác biệt wire A/B, xác nhận đường `relay-ssh` của Go | ⚪ P2 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-CV-001 (khung, whitelist, repo, giới hạn, lỗi, runner qua tệp tạm)
   ├──▶ CR-CV-002 (GitNexus) ──┬──▶ CR-CV-005 (detectChanges)
   │                           └──▶ CR-CV-004 (reindex, indexChanged; cần probe của 002 và 003)
   └──▶ CR-CV-003 (CodeGraph; dùng codeintel-symbol-ref.ts của 002)
CR-CV-006 (Part B) sau khi 001-005 ổn định
```

CR-CV-001 trước hết, vì mọi CR còn lại gắn vào `codeintel-method-table.ts`, `runCodeIntelTool` và bộ mã lỗi của nó. CR-CV-002 và CR-CV-003 có thể làm song song nếu thống nhất file `codeintel-symbol-ref.ts` (CR-CV-002 tạo; nếu CR-CV-003 vào trước thì tạo với bảng kind đầy đủ). CR-CV-004 cần probe chỉ mục của cả hai nguồn. CR-CV-005 cần mẫu `FILE_SYMBOLS_BATCH`/`SYMBOL_FLOWS` của CR-CV-002. Theo bảng đợt ở README v7 mục 5: đợt 1 (001, 002), đợt 2 (005), đợt 4 (003, 004), đợt 6 (006).

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Chỉ Part A (`agent-rpc-dispatch*.ts`) ở các CR 001-005; Part B ở CR-006 (P2) | Đường `relay-ssh` do Go khởi tạo chạy `agent.js --stdio`/`--detach` với cùng `createSession` của Part A (đã đọc `agent-entry.ts`, `agent-connection-stdio.ts`), nên tự có method mới |
| F2 | Mã mới nằm phẳng trong `agent/src/relay/` với tên khái niệm (`codeintel-*.ts`, `gitnexus-*.ts`, `codegraph-*.ts`); dispatcher riêng `agent-rpc-dispatch-codeintel.ts` | Quy ước AGENTS.md (không `helpers/utils/common`); mẫu các `agent-rpc-dispatch-*.ts`; thư mục `relay/` đã phẳng |
| F3 | Mọi lệnh CLI là đối tượng có kiểu (`GitNexusCommand`, `CodeGraphCommand`), không có API nhận argv; hai lệnh làm mới (`analyze`, `sync`/`index`) chỉ ở `codeintel-reindex-commands.ts` | D5; chặn `analyze/clean/remove` và chèn cờ |
| F4 | `-r <đường dẫn tuyệt đối từ registry>` cho GitNexus, `-p <projectPath>` cho CodeGraph; không bao giờ nhận tên repo/đường dẫn công cụ từ client | O4; đã chạy thử `-r` nhận đường dẫn |
| F5 | Stdout của `gitnexus` đi qua tệp tạm (thư mục `0700`), không qua pipe | Đã đo: pipe cắt cụt ở 65 536 byte (pipe shell) hoặc 146 176 byte (`spawn` pipe), JSON hỏng, `exit 0` |
| F6 | Cypher: mẫu hằng + bốn bộ mã hoá giá trị + guard, vì CLI không có tham số ràng buộc; mỗi mẫu phải được chạy thử trên repo thật trước khi merge, đầu ra thành fixture | Đã chứng minh `OR 1=1` chạy nếu ghép chuỗi thô; lỗi truy vấn trả `exit 0` |
| F7 | Số dòng chuẩn hoá về 1-based tại agent (GitNexus +1) | Đã đo: GitNexus 0-based, CodeGraph/Git 1-based; `context --content` lệch một dòng nên mã nguồn đọc từ tệp |
| F8 | Độ cũ: `stale` = `indexedCommit ≠ HEAD` hoặc `worktreeMismatch`; CodeGraph (không có commit) dựa `pendingChanges` | Cả hai CLI tự gắn worktree liên kết vào chỉ mục của checkout chính (đã chạy) |
| F9 | Trong lúc làm mới chỉ mục, đọc GitNexus trả `CODEINTEL_REINDEX_IN_PROGRESS`; job tồn tại qua mất kết nối; không bao giờ xoá hay sửa tệp trong `.gitnexus/`, `.codegraph/` | Có `lbug.wal.missing-shadow.*` và nhiều tiến trình MCP giữ DB; `analyze` chưa `--index-only` làm bẩn `AGENTS.md`/`CLAUDE.md` |
| F10 | Trung thực trong dữ liệu và UI: `percent: null` khi không biết, cờ `warnings`, mức tin cậy theo tệp, nhãn "theo chỉ mục tại <commit>" | Số liệu công cụ là heuristic, có thể cũ hoặc lệch phiên bản |

## Phạm vi ngoài feature này

- `code-intel-service`, collector, cache bền, phân quyền (nhóm `code-intel-service-foundation`, `code-intel-graph-pipeline`).
- Phía Go: timeout dài hơn cho `codeintel.*`, consumer thông báo, `StreamCodeIntelEvents` (CR-CV-023). Các CR ở đây **yêu cầu** timeout ≥ 60 s cho `codeintel.detectChanges` (CR-CV-005 2.7) nhưng không sửa Go.
- Phiên MCP stdio dài hạn với GitNexus (phương án B của nghiên cứu 02), `gitnexus query/trace/wiki/group`, tìm kiếm ngữ nghĩa (embeddings), PDG: không dùng.
- Hỗ trợ Windows dev server (CR-CV-001 chặn bằng `unsupported_platform`).
- Đọc SQL migration, proto, compose (nhóm `code-intel-sources`).
- Gỡ hay chặn tool `gitnexus`/`codegraph` cũ của `tools/call`: CR-CV-001 đề xuất chặn động từ ghi (Q3), không xoá tool.

## Việc phải kiểm chứng trước khi viết mã (tổng hợp từ các CR)

1. Chạy từng mẫu Cypher chưa chạy ở CR-CV-002 2.4 (`STEP_EDGES`, `MEMBER_CLUSTER` theo `IN`, `SG_EDGES_AROUND`, `RT_LIST` có `SKIP`, `FILE_SYMBOLS_BATCH` với `MATCH (n)`), lưu fixture, ghi thời gian.
2. Thử `gitnexus analyze --index-only` và `codegraph index` trên **bản sao nhỏ** của repo để chốt định dạng tiến độ, thời gian, và hiệu ứng huỷ giữa chừng (CR-CV-004); chưa ai chạy `analyze` khi soạn các CR này.
3. Đo `analyze` Orca (thời gian, RAM) và nhiều tiến trình `gitnexus mcp` đang mở cùng DB.
4. Kiểm tra `codegraph telemetry status` trên dev server và `EXPLAIN QUERY PLAN` cho các truy vấn SQLite (CR-CV-003).
5. Xác nhận Node/`node:sqlite` trên dev server mục tiêu (Node ≥ 22.5; `engines.node` của agent là 24, `build.mjs` target `node22`).
6. Chạy `gitnexus impact` cho `runToolCommand`, `branchCompare`, `RelayDispatcher.handleRequest` trước khi sửa (CR-CV-001, 005, 006); rà lại ai gọi `tools/call` với `gitnexus`/`codegraph`.

## Điểm lệch giữa README v7 / nghiên cứu và code, đã phát hiện khi viết feature này

Số liệu và hành vi dưới đây do chạy thật `gitnexus 1.6.9`, `codegraph 1.4.1`, Git 2.43 và đọc code ngày 2026-10-05.

**README v7 mục 3.2-3.3 và mô hình**

- "`workspaceRoot` đã nằm trong workspace root đăng ký": agent **không có** registry thư mục làm việc; `fs.*` nhận mọi đường dẫn tuyệt đối, `RelayContext.registerRoot` là no-op (`context.ts`). Ranh giới thật là registry GitNexus + phân quyền backend (CR-CV-001 2.4).
- `tools[]` trong `agent.handshake` là mảng **tên** (không phải đối tượng) và chưa có khả năng `codeintel` (CR-CV-001 2.7).
- Thiếu method: `codeintel.watch`, `codeintel.reindexStatus`, `codeintel.reindexCancel` (CR-CV-004), `codeintel.codegraphSearch`, `codeintel.files` (CR-CV-003, đề xuất).
- `codeintel.reindexProgress.percent` phải cho phép `null`; `codeintel.indexChanged` cần thêm `reason`, `headCommit`, `stale` và `tool: "git"` cho thay đổi HEAD.
- Mã lỗi: bổ sung `CODEINTEL_SYMBOL_NOT_FOUND` (hiện gộp vào `CODEINTEL_INVALID_PARAMS (reason = not_found)`).
- Chấp nhận timeout riêng cho `codeintel.detectChanges` (≥ 60 s; Go hiện cắt mọi method ngoài `agent.execPrompt` ở 30 s).

**Nghiên cứu 02-05**

- `cypher` không có tham số ràng buộc (`Parameter id not found`); mẫu "có tham số" là thay thế chuỗi có mã hoá.
- `cypher` trả ba dạng: `{markdown,row_count}`, `[]`, `{error}`; lỗi truy vấn/symbol không tồn tại/ghi bị chặn đều **thoát mã 0**; thiếu `-r` thì Node ném lỗi ra stderr.
- Markdown **không thoát** ` | ` và gộp xuống dòng thành dấu cách; ô rỗng gồm cả `null`; mảng là chuỗi JSON có nháy đơn thừa. Mẫu `ca<>cb` ở nghiên cứu 04 chưa chạy được; bản chạy được là `ca.id <> cb.id`.
- Stdout `gitnexus` bị **cắt cụt** khi đi qua pipe (nghiên cứu 02 §2 coi `runToolCommand` dùng được).
- GitNexus **0-based** về dòng; `context --content` lệch một dòng và mất xuống dòng.
- `File -DEFINES-> symbol` **bỏ sót phương thức** (cần `MATCH (n) WHERE n.filePath ...`); khoá `qualifiedName`: GitNexus dùng `.`, CodeGraph `::`; GitNexus có hậu tố `#n`.
- `gitnexus list` chỉ in văn bản; JSON nằm ở `~/.gitnexus/registry.json`; `-r` nhận cả đường dẫn tuyệt đối; chạy trong worktree liên kết trả chỉ mục của checkout chính (CodeGraph báo `worktreeMismatch`).
- `gitnexus detect-changes` chỉ in văn bản, tối đa 15 symbol và 10 luồng (không tăng được bằng `-l`); không đủ cho 2 000 symbol (nghiên cứu 05 §2.7 và 06 giai đoạn 3).
- `gitnexus impact`: `affected_processes` là điểm vào, không phải process; số nút có thể bị đếm thiếu do các `handler` vô danh bị gộp.
- `communities`: `meta.json`/registry ghi 10 252, đồ thị có 9 039 (nguyên nhân chưa rõ, nghiên cứu 04 đã ghi chờ xác minh).
- CodeGraph: `callers`/`callees` theo **tên** và nhiễu (người gọi là nút `file`, lẫn tệp test); lỗi in dạng văn bản ANSI và thoát mã 0; `status` nhận đường dẫn vị trí (không có `-p`); `status --json` không có commit; `lastIndexed` không đổi khi daemon tự đồng bộ; số nút/cạnh lệch số trong README v7 (295 910 / 959 095 / 15 773 so với 295 761 / 958 441 / 15 759) vì chỉ mục tự cập nhật.
- `gitnexus analyze` không có `--index-only` sẽ ghi `AGENTS.md`/`CLAUDE.md` và cài skill vào repo.
- `agent-tool-registry.ts` còn tool `shell` (`bash -c`); whitelist `codeintel.*` không làm agent an toàn hơn về tổng thể (CR-CV-001 2.9).

**Part A/Part B (`specs/agent/api/*`)**

- Spec mô tả "`relay-ssh` = Stack B (`RelayDispatcher`)" theo backend TypeScript cũ; backend Go chạy `agent.js --stdio`/`--detach` (Part A). Bản Part B được build thật nằm ở `desktop/src/relay/`, không phải `agent/src/relay/` (đã xác nhận `build-relay.mjs`); hai cây lệch nhau.
- Part B làm rơi `error.data` ở `RelayDispatcher.handleRequest` (CR-CV-006 1.2).
