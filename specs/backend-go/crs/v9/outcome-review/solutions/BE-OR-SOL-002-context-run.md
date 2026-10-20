---
cr_id: CR-OR-002
solution_id: BE-OR-SOL-002
product: orca
service: request-service
layer: backend
language: go
status: draft
priority: P0
effort: 8–10 ngày
depends_on: BE-OR-SOL-001, skill tdd-knowledge, MCP tdd-agent trên dev server
amendments: AM-1, AM-2, AM-3, AM-4, AM-5
created: 2026-10-20
---

# BE-OR-SOL-002 — Context run: ngữ cảnh KG, thiết kế khoá, diff mức hệ thống

## 1. Phân tích (kèm bằng chứng từ KG thật, 2026-10-20)

Đo trên `vnp-qa-platform` bằng `tdd-agent -C <dự án> kg stats|search` (chỉ-đọc): 935 phần tử, 585 quan hệ, snapshot `snap-000004`; 10 `Component` (`SVC-01…08`, `SVC-BUG-09`, `SVC-JIR-13`), 17 `Decision` (`ADR-01…17`), 16 `Constraint`; `CT-01`/`CT-02` (SPA, gateway) là `Constraint`; `DS-*`, `EXT-*` không phải phần tử; 912 phần tử `needs_review`; quan hệ giữa container nằm trong văn bản SAD (mục 5.1, 5.3, 5.5, 8.x), không phải cạnh KG. `kg search` trả id và đoạn trích ngắn, nội dung đầy đủ cần `source_get`/đọc SAD.

Hệ quả thiết kế: (1) thành phần đến từ ba nguồn (AM-1); (2) trạng thái nút theo nguồn (AM-2); (3) `reviewState` (AM-3); (4) trích thiết kế một lần và khoá theo `(snapshot, sha256 SAD)` (AM-4); (5) `kind`/mức C4 từ văn bản (AM-5).

Orca không đọc KG (quyết định hiện có). Mọi nội dung KG đến qua một lượt agent chỉ-đọc dùng skill `tdd-knowledge`; không dùng `tdd-knowledge-writeback`. ĐỌC TRƯỚC: usecase Solution agent (tái dùng đường `agent.execPrompt`, khối kết quả có nonce, hồ sơ quyền chỉ-đọc), `dev_server_capability.go`, `solution_design_references.go`.

## 2. Thiết kế

### 2.1 Hai chế độ

- `mode=design`: đọc KG và `source_get` các mục SAD liên quan; **không** đọc diff, không mở mã. Trả `design.*`, các yêu cầu/ràng buộc áp dụng, mô hình sơ đồ `variant=design`.
- `mode=delivered`: dùng lại `design.*` đã khoá nếu còn hợp lệ (không gọi `kg_*`), chỉ đọc code (`baseRef`, head) để dựng `variant=delta`, `inBase/inHead`, các mục `needsDecision`.

### 2.2 Brief và lời nhắc

`usecase/outcome_review_prompt.go` (hằng `promptVersion`): rào dữ liệu `<untrusted-input>`; nêu AC/tiêu đề/tên tệp là dữ liệu; cấm `changeset_propose` và mọi công cụ ghi; yêu cầu mọi nút có `source` (`kg|sad-text|code`), mọi phần tử đọc có `reviewState`, mọi cạnh/tin nhắn thay đổi có `evidence[]` (kinds `kg|source|file|rpc|channel|migration|check`). Brief chỉ chứa AC, danh mục tệp đổi nhóm theo thư mục cấp một (gợi ý đọc), kết quả kiểm tra. Hồ sơ quyền chỉ-đọc, giới hạn lượt/tệp/thời gian từ cấu hình.

### 2.3 Hợp đồng kết quả (`outcome-review-context/1`)

Khối có nonce (mẫu kết quả task). Thêm so với CR gốc:
- `design.sourceBlocks[]`: `{doc, sha256, section, kind:"mermaid|text", text}`. **Orca parse xác định** các khối `mermaid` (`flowchart`, `sequenceDiagram`) thành cạnh/tin nhắn (AM-4); khi agent và parser lệch, ưu tiên parser và ghi `warnings:["design_edge_mismatch:<id>"]`.
- `design.containers[]`: `{kgId?, label, kind, tech, source: kg|sad-text, reviewState, evidence[]}`.
- Mỗi phần tử KG đọc: `{id, class, reviewState}`.
- `kg.snapshot`, `design.docSha256`.

### 2.4 Kiểm và hợp nhất (Orca)

`domain/outcome_review_context.go`: kiểm lược đồ; id KG khớp `^[A-Z]{2,6}(-[A-Z0-9]+)*-[0-9A-Za-z._:-]+$` (chấp nhận `SVC-BUG-09`, `ENT-02:attribute:email`); `kgId` phải ∈ danh sách phần tử agent khai đã đọc; `file` ∈ danh mục tệp đổi hoặc tồn tại ở `baseRef`; chuỗi tự do qua bộ che; trần số mục.
`domain/outcome_review_diagram_model.go`: nút/làn `level=container` (từ chối tên lớp/hàm/thư mục con, tin nhắn tự gọi; `kind` theo văn bản, không theo lớp KG — AM-5); id nút = `kgId` hoặc `sad:<mã>` (như `DS-01`) hoặc `code:<kebab(label)>`; cạnh/tin nhắn mang `inBase/inHead/inDesign`; **trạng thái do Orca suy** (`added|removed|same|deviation|missing`); `deviation`/`missing` dựa trên phần tử `needs_review` chỉ ở mức `advisory` (AM-3); `≤ 12` nút/`≤ 6` làn, gộp thành "+N more".
`unmapped` chỉ khi `source=code` (không có ở KG lẫn SAD) — AM-2.
Số liệu `verdict.architecture|flow` là phép đếm xác định trên mô hình đã kiểm, kèm `designChecked`, `reviewedRatio` (tỉ lệ phần tử thiết kế đã duyệt).

### 2.5 Khoá thiết kế và cache

- `design_digest = sha256(kg.snapshot, design.docSha256, promptVersion, schemaVersion)`; bản `design.*` đã chấp nhận lưu theo `design_digest` và **tái dùng** ở pha `delivered` và giữa các lần sinh; chỉ trích lại khi người dùng sinh lại hoặc snapshot/SAD đổi.
- `context_digest` cho phần `delivered` = `sha256(inputsDigest_F, design_digest, promptVersion)`; TTL `REQUEST_REVIEW_CONTEXT_TTL` (7 ngày).
- Thiếu snapshot trước khi chạy: chấp nhận không thăm dò; hiển thị `kg.snapshot` và nút sinh lại (Q2 của CR).

### 2.6 KG vắng

`kg.available=false`: không chạy context run mặc định (`allow_code_only=true` mới chạy, mọi mục `unverified`); `designChecked=false`; `unplanned*` = Unknown; không có sơ đồ `variant=design`.

### 2.7 Cấu trúc file

`usecase/outcome_review_context_run.go`, `outcome_review_prompt.go`, `outcome_review_design_lock.go`; `domain/outcome_review_context.go`, `outcome_review_diagram_model.go`, `outcome_review_evidence.go`, `outcome_review_sad_mermaid.go` (parser `flowchart`/`sequenceDiagram`, hàm thuần), `outcome_review_edge_state.go`.

## 3. Kiểm thử

- Agent giả: kết quả tốt; hỏng một lần rồi tốt; hỏng hai lần; thiếu nonce; cố ghi tệp/gọi `changeset_propose`; nút component; `kgId` ngoài danh sách đã đọc.
- Injection (AC/tên tệp/văn bản SAD chứa chỉ thị) — kết quả vẫn qua lược đồ, không lộ biến môi trường.
- Parser mermaid: fixture lấy từ SAD của `vnp-qa-platform` (5.5 flowchart, 8.1 sequence, 8.4 flowchart) → kỳ vọng cạnh viết tay.
- Cache: chế độ `delivered` sau `design` không gọi lại `kg_*` (đếm); hai lần sinh cùng `design_digest` cho cùng `inputDigest` sơ đồ `design`.
- Chạy thật có ghi lại (tốn tiền, cần đồng ý chi phí; chạy `workspace_probe.py` trước): một Request pilot trên `vnp-qa-platform`.

## 4. Rủi ro

- Chất lượng cạnh/luồng do LLM đọc văn bản chưa biết; parser mermaid chỉ phủ phần có sơ đồ.
- Trích SAD 933 dòng tốn token; phải đo, có thể chỉ đọc các mục 3.2, 5.1, 5.3, 5.5 và luồng liên quan.
- Công cụ MCP có trả `reviewState` của phần tử hay không: chưa kiểm chứng (spike P0).
