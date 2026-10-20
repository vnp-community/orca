# Backend: Outcome Review (CR-OR-001, 002, 010 đến 013, 022 phần backend, 030)

> Giải pháp kỹ thuật và checklist triển khai phần backend của series [`docs/crs/v9`](../../../../../docs/crs/v9/README.md). Phần frontend ở [`specs/frontend/crs/v9/outcome-review`](../../../../frontend/crs/v9/outcome-review/README.md). **Trạng thái: 📝 Dự thảo, chưa triển khai** (soạn 2026-10-20). Mọi chỗ ghi "ĐỌC TRƯỚC" là điều chưa được đọc khi soạn CR; phải đọc code trước khi sửa.

## 1. Phạm vi và nơi chạm

| Thành phần | Việc | Vị trí |
|---|---|---|
| `request-service` | mô hình review, dữ kiện xác định, context run, kho artifact, vẽ, sinh lại, hồ sơ vẽ, quyết định | `backend-go/services/request-service/internal/{domain,usecase,adapter}`, `migrations/{postgres,mysql}` |
| proto | `OutcomeReview*` và RPC | `backend-go/proto/orca/request/v1/outcome_review.proto` |
| `api-gateway` | kênh `request.review.*`, push | `backend-go/services/api-gateway/internal/adapter/wscompat/` |
| MCP, OPA | tool `request_review_*`, `agent_rpcs` | theo mẫu `request.activity` (handoff 2026-10-17, mục 4.1) |
| Skill | bản ghim `diagram-design` | `skills/diagram-design/` |
| CLI | `orca review profile` | `backend-go/cmd/orca-cli/internal/command/` |
| Dev server | khả năng `reviewContext`, `reviewDraw` | `dev_server_capability.go` (ĐỌC TRƯỚC) |

## 2. Điều chỉnh CR bắt buộc (AM) rút ra từ KG thật của `vnp-qa-platform`

Giải pháp dưới đây **giả định** năm điều chỉnh sau đối với CR (chưa sửa trong `docs/crs/v9`; việc sửa là task BE-OR-P0-05):

| AM | Nội dung | Lý do (đo ngày 2026-10-20) |
|---|---|---|
| AM-1 | Nguồn thành phần thiết kế = phần tử KG lớp `Component`, `Constraint` mã `CT-*`, **và** văn bản SAD qua `source_get`. Thêm loại bằng chứng `source` (`{doc, section, line}`) | KG có 10 `Component`; gateway/SPA là `CT-02`/`CT-01` thuộc `Constraint`; cạnh và luồng chỉ có trong văn bản SAD |
| AM-2 | Mỗi nút/làn mang `source: kg\|sad-text\|code`. `unmapped` chỉ cho thành phần **không có trong cả KG lẫn SAD** (`source=code`). Datastore, hệ ngoài chỉ có trong SAD là `sad-text`, không bị gắn "Not in design" | `DS-*`, `EXT-*` không phải phần tử KG |
| AM-3 | Mỗi phần tử KG đọc mang `reviewState` (`approved\|needs_review\|unknown`). Lệch dựa trên phần tử chưa duyệt chỉ là `advisory` (không `needsDecision`); UI hiện dải "Design not reviewed" | 912/935 phần tử `needs_review` |
| AM-4 | Bản trích thiết kế (`design.*`) **khoá** theo `(kg.snapshot, sha256 của tài liệu SAD)` và tái dùng; khối mermaid trong SAD được **parse xác định** ở Orca, agent chỉ bổ sung phần văn bản | Tránh dao động digest, tiết kiệm token; SAD đã có mermaid cho container/sequence/flowchart |
| AM-5 | `kind` và mức C4 do văn bản SAD/agent kết luận, **không** suy từ lớp KG | `SVC-BUG-09`, `SVC-JIR-13` (level 3) cùng lớp `Component` với service |

## 3. Giải pháp

| Solution | CR | Nội dung |
|---|---|---|
| [BE-OR-SOL-001](./solutions/BE-OR-SOL-001-model-and-facts.md) | CR-OR-001 | Mô hình, hai pha, tầng F, bảng, RPC, cờ |
| [BE-OR-SOL-002](./solutions/BE-OR-SOL-002-context-run.md) | CR-OR-002 + AM | Context run hai chế độ, bằng chứng, mô hình sơ đồ, trích thiết kế khoá |
| [BE-OR-SOL-003](./solutions/BE-OR-SOL-003-artifact-store-drawing-regenerate-profile.md) | CR-OR-010, 011, 012, 013 | Kho artifact, vẽ bằng skill, sinh lại, hồ sơ vẽ |
| [BE-OR-SOL-004](./solutions/BE-OR-SOL-004-decisions-gateway-security-rollout.md) | CR-OR-022 (backend), 030 | Quyết định/duyệt, gateway/MCP/OPA, bảo mật, số liệu, rollout |

## 4. Checklist

| Phase | File | Mục tiêu | Phụ thuộc |
|---|---|---|---|
| P0 | [phase-0-spikes-and-amendments.md](./tasks/phase-0-spikes-and-amendments.md) | Đọc code, spike S1 đến S4, sửa CR (AM) | — |
| P1 | [phase-1-model-and-facts.md](./tasks/phase-1-model-and-facts.md) | Tầng F, hai pha, RPC `Estimate`/`Generate`/`Get`/`List` | P0 |
| P2 | [phase-2-context-run.md](./tasks/phase-2-context-run.md) | Context run `design` và `delivered` | P1 |
| P3 | [phase-3-artifact-store-and-drawing.md](./tasks/phase-3-artifact-store-and-drawing.md) | Kho, vẽ, kiểm artifact, skill ghim | P1, P2 |
| P4 | [phase-4-regenerate-profile-decisions.md](./tasks/phase-4-regenerate-profile-decisions.md) | Sinh lại, hồ sơ vẽ, quyết định, duyệt | P3 |
| P5 | [phase-5-gateway-security-rollout.md](./tasks/phase-5-gateway-security-rollout.md) | Gateway, MCP, OPA, số liệu, e2e, tài liệu, rollout | P1 đến P4 (kênh có thể làm sớm theo từng RPC) |

## 5. Quy ước chung cho mọi task

- **CLAUDE.md:** trước khi sửa symbol đã có chạy `gitnexus_impact({target, direction:"upstream"})`; trước commit chạy `detect_changes()`. Nếu công cụ GitNexus không có trong phiên, ghi rõ vào mô tả PR (handoff 2026-10-17 mục 2 đã gặp trường hợp này).
- Hai dialect (Postgres, MySQL), `tenant_id` mọi bảng, không FK chéo service; migration có file `down`; số migration = số kế tiếp sau `0104` **tại thời điểm triển khai**.
- File Go quá ngưỡng dòng thì tách; không `max-lines` disable; không đặt tên `helpers/utils/common`.
- Chú thích chỉ ghi "vì sao", 1 đến 2 dòng.
- Lệnh git mới theo baseline Git 2.25 (`guides/reference/git-compatibility.md`).
- Không in secret vào log, audit, DB; chuỗi tự do đi qua bộ che trước khi lưu.
- Lệnh kiểm chung:
  ```bash
  cd backend-go && go build ./services/request-service/... ./services/api-gateway/... \
    && go test ./services/request-service/... ./services/api-gateway/internal/adapter/wscompat/...
  cd backend-go && go test -tags=integration ./services/request-service/internal/adapter/... -run 'OutcomeReview|ReviewArtifact' -v
  ```
- Mỗi task: `[ ]` chưa làm, `[~]` đang làm, `[x]` xong **và đã chạy lệnh Verify** (ghi ngày và kết quả).
