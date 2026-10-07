# BE-CV-SOL-036-reading-order-and-risk: Thứ tự đọc (callee trước), rủi ro cộng điểm minh bạch, gom component, giới hạn, `GetReadingOrder`

> **📋 Proposed.** Chưa triển khai, chưa chạy test. P0. Là nửa **logic thuần** của CR-CV-036; nửa điều phối/I/O ở [`BE-CV-SOL-036-change-overlay-pipeline`](./BE-CV-SOL-036-change-overlay-pipeline.md) (cùng proto `codeintel_change_overlay.proto`). Các hằng số điểm là giá trị khởi điểm của CR, chưa hiệu chỉnh (O-7).

**CR:** [CR-CV-036](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md) (§2.3, §2.4, §2.7)
**Service:** `code-intel-service` (mới)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md): **PQ-09** (`RiskAssessment` 4 mức + `incomplete`), **PQ-30** (`TouchedContract.compatibility`, `CONTRACT_BREAKING`), **PQ-31** (`ReadingStep`, `stepKey` là khoá tiến độ), **PQ-32** (`risk.level` HOA), **PQ-06** (`ViolationRef.status ∈ introduced|touched`), **PQ-14** (≤ 2 MiB), **PQ-19** (`impact`: không cạnh); §3.1 `GetReadingOrder`. `CONTRACT-codeintel-agent-rpc.md` §4.5 (`levels[].symbols[{via, direct}]`, `testsCovering`), §4.8 (`affectedFlows`, `riskHint`). `CONTRACT-codeintel-ui-api.md` §4.3.
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần, test không mock), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (metric thời gian tính), [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md` §2.3–2.7; hợp đồng agent §4.5 (`impact`), §4.8 (`detectChanges`); `backend-go/services/code-intel-service` **không tồn tại**; `backend-go/policy/` có `orca-authz` (đường dẫn nhạy cảm `backend-go/policy` có thật); `backend-go/common/{jwtauth,secrets}` có thật (đã `ls`); `backend-go/services/{auth-service,credential-broker-service}` có thật. Chưa có mã nào của thuật toán; không có thư viện đồ thị nào trong module (tự viết Tarjan/Kahn, không thêm dependency).

### Correction relative to CR-CV-036

| # | CR nói | Hợp đồng / thực tế | Xử lý |
|---|--------|--------------------|-------|
| C1 | Cạnh `A → B` lấy bằng Cypher `IN $ids` hoặc `context.outgoing.calls` | Agent không nhận Cypher; `impact` không có cạnh toàn cục nhưng `levels[0]` (độ sâu 1, `direct:true`, `via`) cho **caller trực tiếp** của symbol đã phân tích | Cạnh = `S → C` (S đứng trước C) khi `C ∈ changedSymbols`, `C ∈ levels[0](S)` và `via ∈ {calls, accesses}`; chỉ có cho ≤ K symbol đã chạy `impact`; còn lại `no-edges` |
| C2 | Fallback `IMPORTS` giữa tệp khi thiếu cạnh | `codeintel.subgraph{center:{file}}` có nhưng 1 lệnh/tệp | Không làm ở MVP (Q2); bước sắp theo `layerRank` + đường dẫn |
| C3 | `FLOWS` +1 nếu có luồng `cross_community` | `affectedFlows` không có `processType` | Bỏ +1; `modelVersion "1"` ghi rõ |
| C4 | `uncoveredSymbols` cần cạnh `CALLS` từ tệp test độ sâu ≤ 2 | `impact` với `includeTests` trả `testsCovering` (lọc theo mẫu tên ở agent) | Dùng `testsCovering`; symbol chưa chạy `impact` ⇒ `tested:"unknown"` |
| C5 | `RiskReason.evidence: (SymbolRef|path|findingKey)[]` | UI §4.3 `evidence: (SymbolRef|string)[]` | `path`/`findingKey` là string |
| C6 | `stepKey` "hash ổn định" | PQ-31 chốt: khoá tiến độ là `stepKey` | Công thức cố định ở 2.B |

## 2. Giải pháp

### 2.A Cây file (mới)

```
backend-go/services/code-intel-service/internal/domain/readingorder/
    reading_graph.go          # nút (symbol thực thi), cạnh từ impact, SCC nén
    strongly_connected.go     # Tarjan lặp (không đệ quy)
    dependency_ordering.go    # Kahn + hàng đợi ưu tiên (layerRank, path, startLine)
    reading_steps.go          # gộp theo tệp, bước generated/overflow, stepKey, reason
    layer_rank.go             # layer theo đường dẫn / component
internal/domain/overlayrisk/
    risk_rules.go             # bảng quy tắc điểm, hằng số, modelVersion "1"
    risk_scoring.go           # ScoreRisk -> RiskAssessment
internal/domain/changeoverlay/{component_grouping.go, overlay_limits.go}
internal/usecase/get_reading_order.go
internal/adapter/grpc/reading_order_handler.go
```

Tất cả `domain/` chỉ import stdlib. Tên gói theo khái niệm (không `helpers/utils`).

### 2.B Thứ tự đọc

1. **Nút**: `changedSymbols` thực thi (function, method, type; không test/doc/generated/`deleted`). **Cạnh** (C1): với mỗi symbol `S` đã chạy `impact`, mỗi `C ∈ levels[0].symbols` thuộc tập nút và `via ∈ {calls, accesses}`: cạnh `S → C`. Cạnh loại tự trỏ, khử trùng.
2. **Vòng**: Tarjan lặp; SCC cỡ > 1 nén thành một nút `cycleGroup` (`reason: cycle`); thành viên sắp theo `(path, startLine)`. Đồ thị nén là DAG.
3. **Kahn có hàng đợi ưu tiên**: nút được phát khi mọi tiền nhiệm đã phát; khoá `(layerRank, path, startLine)` với `layerRank`: 0 hợp đồng (`.proto`, migration), 1 `domain`, 2 `usecase`, 3 `adapter`, 4 gateway/frontend/agent/khác, 5 test, 6 tài liệu/cấu hình (từ đường dẫn `internal/{domain,usecase,adapter}` hoặc component C4 nếu có). **Xác định**: cùng đầu vào cùng thứ tự (không `map` iteration, không `time`, không `rand`).
4. **Gộp bước theo tệp**: symbol liên tiếp cùng tệp ⇒ một `ReadingStep`; tệp không symbol (proto, migration, cấu hình, tài liệu) là bước riêng theo `layerRank`; tệp `isGenerated` gộp **một** bước cuối `reason: generated`, không tính vào rủi ro dòng.
5. **`stepKey`** = `hex(sha256(file + "\n" + join(sorted(symbol.key), "\n")))[:16]`; bước không symbol: `sha256(file)[:16]` (PQ-31). Thêm/bớt symbol cùng tệp đổi `stepKey` của **tệp đó** (chấp nhận); các bước khác không đổi.
6. **`reason`** ∈ `contract|dependency-of|leaf|cycle|no-edges|test|doc|generated|overflow` (UI §4.3); `dependency-of` kèm `reasonParams{caller: <stepKey>}`; `no-edges` khi symbol chưa có dữ liệu cạnh. Không LLM; văn bản do frontend dựng từ khoá.
7. `dependsOn` = `stepKey` của bước chứa tiền nhiệm; `tests` = `testsCovering` của các symbol trong bước (khử trùng, ≤ 10); `layer` ∈ `contract|domain|usecase|adapter|edge|test|doc`.
8. **Hạn mức**: ≤ 300 bước; vượt ⇒ gom phần còn lại vào một bước `overflow` (mang số lượng ở `reasonParams.count`) và `limits.truncated.steps=true`.

### 2.C Rủi ro (`overlayrisk`, `modelVersion "1"`)

Tổng `score = Σ reasons[].points`; mỗi dòng hiển thị được. Bảng (CR §2.4, sửa C3):

| Mã | Điều kiện | Điểm |
|---|---|---|
| `SIZE_MEDIUM`/`SIZE_LARGE` | tổng dòng thêm+xoá của tệp mã thật (không test/doc/generated) > 200 / > 800 | 1 / 2 |
| `IMPACT_MEDIUM`/`HIGH`/`CRITICAL` | `risk` cao nhất trong các `impact` đã chạy | 1 / 3 / 5 (mức cao nhất) |
| `FLOWS` | số luồng ≥ 1 / ≥ 5 | 1 / 2 |
| `CLUSTERS` | số cụm ≥ 3 / ≥ 6 | 1 / 2 |
| `MIGRATION` | có tệp trong `backend-go/services/*/migrations/**` | 3 |
| `MIGRATION_DESTRUCTIVE` | `ContractDiff` báo `sql.drop-*`/đổi kiểu/`NOT NULL` không default | +2 |
| `CONTRACT` | có `.proto`/`wscompat/channels_*.go`/route bị chạm | 2 |
| `CONTRACT_BREAKING` | `TouchedContract.compatibility=="breaking"` | +4 |
| `UNTESTED` | ≥ 3 symbol `tested:"no"` **và** ≥ 50% symbol thực thi đã đổi | 2 |
| `UNTESTED_HIGH_IMPACT` | có symbol `tested:"no"` mà `impact` ≥ HIGH | +1 |
| `VIOLATION` | mỗi finding `introduced` (tối đa 2 finding tính) | 2 mỗi cái |
| `SENSITIVE_PATH` | chạm `backend-go/services/auth-service`, `backend-go/services/credential-broker-service`, `backend-go/common/jwtauth`, `backend-go/common/secrets`, `backend-go/policy` | 2 |

Ngưỡng: `0–2 LOW`, `3–5 MEDIUM`, `6–9 HIGH`, `≥ 10 CRITICAL`. **Trung thực**: bất kỳ nguồn thiếu/lỗi (impact `UNKNOWN`/lỗi/quá hạn, detectChanges cảnh báo, index cũ, nguồn mềm vắng) ⇒ `incomplete=true`, thêm `DATA_MISSING` (`points=0`, `params.source`); **không** hạ mức vì thiếu. `confidence`: `high` khi index khớp HEAD và không tệp `approx|none`; `medium` lệch nhẹ; `low` khi index cũ nhiều hoặc detectChanges lỗi/impact dưới 50% số đã chọn. `toolRisk` tham chiếu, không vào điểm. `reasons` sắp `points` giảm dần rồi `code`. Mỗi reason (trừ `DATA_MISSING`) có `messageKey` và `evidence` không rỗng. Rủi ro tính trên **toàn bộ** dữ liệu trước khi cắt.

### 2.D Gom component và giới hạn

`GroupComponents`: tệp → component bằng khớp **tiền tố đường dẫn dài nhất** trên `C4Component.packagePaths|path` (từ `ComponentIndex`); thiếu C4 ⇒ nhóm theo đường dẫn (`backend-go/services/<svc>/internal/<domain|usecase|adapter/<tên>>`, `agent/src/<relay|main|shared>`, `frontend/src/renderer/src/<cấp 1>`), `componentId:"path:<prefix>"`, nhãn "suy luận". `riskPoints` nhóm = tổng điểm các `reasons` có `evidence` trong nhóm (không cộng lần hai vào tổng). `ApplyLimits`: `changedFiles ≤ 2 000` (giữ theo `|added|+|removed|`), `changedSymbols ≤ 2 000`, `affectedFlows ≤ 100`, bước ≤ 300; đếm `totalCounts` **trước** khi cắt.

### 2.E RPC `GetReadingOrder`

Dùng lại snapshot overlay (`view="readingOrder"` hoặc suy từ `changeOverlay` cùng khoá); nếu overlay đã có trong cache chỉ trả `steps` + `components`. Quyền `read`. Mã lỗi như `GetChangeOverlay`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Cạnh từ `impact` độ sâu 1, không Cypher | Giữ H3 (agent không nhận Cypher), dữ liệu đã có sẵn; đánh đổi: chỉ ≤ K symbol có cạnh |
| Tarjan lặp, Kahn xác định | 2 000 nút; tránh đệ quy sâu; test lặp 100 lần |
| Điểm cộng thay vì một số công cụ | Giải thích được từng dòng; đổi hằng ⇒ tăng `modelVersion` |
| Không hạ mức khi thiếu dữ liệu | "≥ mức" trung thực hơn điểm đẹp mà sai |
| Cắt sau khi đếm | Rủi ro không giảm do hiển thị |
| Tiến độ theo `stepKey` | PQ-31 |
| Không `time.Now()`/`rand` trong domain | Tính xác định |

## 4. Lệch giữa CR và hợp đồng

| # | CR | Hợp đồng | Theo |
|---|----|----------|------|
| L1 | `FLOWS` +1 cross_community | agent không cho `processType` | Bỏ (C3); báo `AG-CV-SOL-005` nếu muốn thêm |
| L2 | `RiskAssessment.level` | PQ-09/32 HOA, 4 mức | Hợp đồng |
| L3 | `reasonKey` | UI `messageKey` | Hợp đồng |
| L4 | Cạnh bằng Cypher | PQ-21 | Hợp đồng (C1) |
| L5 | `ReasonCode` 8 giá trị | UI §4.3 thêm `overflow` | Hợp đồng |

## 5. Phụ thuộc chéo khu vực và thứ tự

Cùng proto với `BE-CV-SOL-036-change-overlay-pipeline` (TASK-036-02 làm trước). Đầu vào: `impact` (`AG-CV-SOL-002-gitnexus-extraction`, `AG-CV-SOL-003`), `detectChanges` (`AG-CV-SOL-005-detect-changes`), `ViolationRef` từ `BE-CV-SOL-037-structure-findings-and-dismissals`, `TouchedContract` từ `BE-CV-SOL-038-contract-diff`, component từ `BE-CV-SOL-033-c4-component-view` (đều **mềm**). Tiêu thụ: pipeline (TASK-036-05), `FE-CV-SOL-052-reading-order-and-progress`, `FE-CV-SOL-053`. Thứ tự: 036-06 → 07 → 08 → 09 → (pipeline 036-05) → 036-10.

## 6. Kiểm thử

Unit thuần, không I/O: cây, kim cương, vòng, tự gọi, đồ thị rỗng, 2 000 nút; chuỗi A→B→C ra C,B,A; vòng A↔B + C gọi A ra {A,B} liền nhau trước C; `stepKey` không đổi khi thêm symbol ở tệp khác; bảng điểm từng dòng và biên 2/3, 5/6, 9/10; `incomplete`/`DATA_MISSING`; cắt 3 000 tệp; 100 lần chạy cùng đầu ra. Lệnh: `go test ./services/code-intel-service/internal/domain/readingorder/... ./internal/domain/overlayrisk/... ./internal/domain/changeoverlay/... -race`. Tenant/dialect: không có DB ở solution này (test tenant ở pipeline); ghi rõ. Chưa chạy test nào.

## 7. Rủi ro và điểm chưa kiểm chứng

- Chỉ ≤ K symbol có cạnh ⇒ thứ tự kém chính xác khi > 25 symbol; UI cần hiện "dựa trên 25/N".
- Ngưỡng/điểm chưa hiệu chỉnh (cần 10–20 PR lịch sử, CR-070).
- Phát hiện test phủ qua tên tệp, chưa kiểm chứng với TypeScript.
- Số liệu thời gian chưa đo (CR-071).
- `layerRank` từ đường dẫn có thể sai ở service layout khác.

## 8. Câu hỏi mở

1. Danh sách `SENSITIVE_PATH` do ai duyệt; đưa vào `c4.yaml`?
2. Bổ sung cạnh `IMPORTS` mức tệp bằng `subgraph{file}` sau MVP?
3. Điểm rủi ro theo tệp/component hiển thị ở lens?
4. Thêm `IMPLEMENTS`/`METHOD_IMPLEMENTS` vào thứ tự (interface trước)?

## 9. Tiêu chí chấp nhận

- [x] Thứ tự đúng cho A→B→C và vòng; ổn định qua 100 lần; mọi bước có `reason` hợp lệ.
- [x] `stepKey` ổn định theo PQ-31.
- [x] `score == Σ points`; mức đúng ngưỡng; mỗi reason có `messageKey` và `evidence` (trừ `DATA_MISSING`).
- [x] `impact` lỗi toàn bộ ⇒ `incomplete=true`, mức không thấp hơn mức tính từ nguồn còn lại.
- [x] Doc/test/generated không vào `SIZE_*`/`uncoveredSymbols`; `Section` markdown không vào symbol thực thi.
- [x] Giới hạn: 3 000 tệp ⇒ `truncated`, `totalCounts` đúng, rủi ro tính đủ.
- [x] Không phụ thuộc ngoài stdlib.

## 10. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md`; hợp đồng 3 tệp ở `/opt/repos/orca/specs/backend-go/crs/v7/`
- Series: `BE-CV-SOL-036-change-overlay-pipeline`, `BE-CV-SOL-033-c4-component-view`, `BE-CV-SOL-037-*`, `BE-CV-SOL-038-contract-diff`, `AG-CV-SOL-002/005`, `FE-CV-SOL-052/053`
