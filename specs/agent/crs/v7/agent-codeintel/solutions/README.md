# agent-codeintel (v7) solutions: index (agent)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi nhận định "đã đọc" là đọc code ở `agent/`, `desktop/src/relay/` và hợp đồng; chưa chạy hệ thống, chưa chạy `gitnexus`/`codegraph`.

Solution phía `agent/` cho feature `agent-codeintel` của series v7 "Xem code và kiểm soát chất lượng". Nguồn: sáu CR [CR-CV-001..006](../../../../../../docs/crs/v7/agent-codeintel/README.md) và hợp đồng chuẩn `specs/backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md` (+ `CONTRACT-codeintel-proto-and-data-map.md` PQ-xx). Khi CR và hợp đồng khác nhau thì theo hợp đồng; mỗi solution có mục "Lệch giữa CR và hợp đồng". Mỗi CR một solution (tên chốt ở hợp đồng §8.2).

## Bảng CR → Solution → Task

| CR | Solution | Tasks | Ưu tiên |
|---|---|---|---|
| CR-CV-001 | [AG-CV-SOL-001-codeintel-agent-foundation](./AG-CV-SOL-001-codeintel-agent-foundation.md) | 001-01…09 | P0 |
| CR-CV-002 | [AG-CV-SOL-002-gitnexus-extraction](./AG-CV-SOL-002-gitnexus-extraction.md) | 002-01…09 | P0 |
| CR-CV-003 | [AG-CV-SOL-003-codegraph-extraction](./AG-CV-SOL-003-codegraph-extraction.md) | 003-01…08 | P1 |
| CR-CV-004 | [AG-CV-SOL-004-reindex-and-index-notifications](./AG-CV-SOL-004-reindex-and-index-notifications.md) | 004-01…09 | P1 |
| CR-CV-005 | [AG-CV-SOL-005-detect-changes](./AG-CV-SOL-005-detect-changes.md) | 005-01…08 | P0 |
| CR-CV-006 | [AG-CV-SOL-006-relay-ssh-part-b-handlers](./AG-CV-SOL-006-relay-ssh-part-b-handlers.md) | 006-01…07 | P2 |

Tổng: 6 solution, 50 task. Danh sách task đầy đủ: [../tasks/README.md](../tasks/README.md).

## Bảng method hợp đồng → solution → task

(Method `codeintel.*` của `CONTRACT-codeintel-agent-rpc.md` §4; mọi method nhận `workspaceRoot`.)

| Method / thông báo | Hợp đồng | Solution | Task chính |
|---|---|---|---|
| `codeintel.status` | §4.1 | 001 (nền; trường CR-080 do `AG-CV-SOL-080`; khối GitNexus 002, CodeGraph 003) | 001-09; 002-07; 003-02, 003-04 |
| `codeintel.overview` | §4.2 | 002 | 002-03, 002-04, 002-08 |
| `codeintel.processes` | §4.3 | 002 | 002-08 |
| `codeintel.process` | §4.3 | 002 | 002-08 |
| `codeintel.subgraph` | §4.4 | 002 (+ làm giàu 003) | 002-09; 003-07 |
| `codeintel.impact` | §4.5 | 002 (+ `testsCovering` 003) | 002-09; 003-06, 003-07 |
| `codeintel.symbol` | §4.6 | 002 (+ `includeTrail`, `codegraphId` 003) | 002-09; 003-07 |
| `codeintel.routes` | §4.7 | 002 | 002-08 |
| `codeintel.detectChanges` | §4.8 | 005 | 005-01…07 (re-verify 005-08) |
| `codeintel.structuralFacts` | §4.9 | **ngoài feature này**: `AG-CV-SOL-037-structural-facts` (feature `code-intel-sources`); khe trong bảng method và `check --cycles` ở whitelist do 001 chừa | 001-03, 001-09 |
| `codeintel.reindex` | §4.10 | 004 | 004-02, 004-04, 004-05, 004-08 |
| `codeintel.reindexStatus` | §4.11 | 004 | 004-04, 004-08 |
| `codeintel.reindexCancel` | §4.12 | 004 | 004-05, 004-08 |
| `codeintel.watch` | §4.13 | 004 | 004-07, 004-08 |
| `codeintel.codegraphSearch` | §4.14 | 003 | 003-01, 003-05 |
| `codeintel.files` | §4.14 | 003 | 003-01, 003-05 |
| `codeintel.node` (không công khai) | §4.15 | 003 (nội bộ cho `includeTrail`) | 003-07 |
| thông báo `codeintel.indexChanged` | §6.1 | 004 | 004-01, 004-07 |
| thông báo `codeintel.reindexProgress` | §6.2 | 004 | 004-01, 004-02, 004-05 |
| capability `codeintel*` ở handshake | §1.3 | 001 | 001-07 |
| mã lỗi, phong bì, giới hạn, whitelist, an toàn | §2, §3, §9 | 001 | 001-01…08 |
| Part B (`RelayDispatcher`) | §8 | 006 | 006-01…06 |
| `quality.*` và thông báo `quality.progress|finished` | §5, §6.3–6.4 | **ngoài feature này**: `AG-CV-SOL-081…084, 091` (sink thông báo của 004-01 dùng chung) | — |

## Thứ tự thực thi và phụ thuộc (hợp đồng §7.2)

```
001 ─► 002 ─► 005
  │      └──► 004
  └──► 003            (003 dùng codeintel-symbol-ref.ts của 002; 004 cần probe của 002 và 003)
006 (P2) sau 001–005; 006-07 độc lập
```
Đợt (hợp đồng §7.3): đợt 1 = 001, 002; đợt 2 = 005; đợt 4 = 003, 004; đợt 6 = 006. Sau 001: `AG-CV-SOL-070`, `072`, `071`, `073`, `081` có thể bắt đầu.

## Phụ thuộc chéo khu vực (tóm tắt; chi tiết ở từng solution)

| Khu vực | Solution đối ứng | Liên quan |
|---|---|---|
| BE | `BE-CV-SOL-023-infra-fleet-codeintel-transport` | timeout Go 30/90 s, `AgentRPCError` + trailer `x-orca-agent-error-data-bin`, `Tools`, chuyển thông báo (cần cho 001, 004, 005) |
| BE | `BE-CV-SOL-021-agent-collector`, `BE-CV-SOL-020-canonical-graph-model` | tiêu thụ 002/003/005; vector chuẩn hoá khoá chung |
| BE | `BE-CV-SOL-036-change-overlay-pipeline`, `BE-CV-SOL-024-event-distribution`, `BE-CV-SOL-012-…` | 005, 004 |
| AG | `AG-CV-SOL-070` (fixture/job CI), `071`, `072`, `073`, `080`, `037`, `081` | cùng nhóm agent |
| FE | `FE-CV-SOL-051`, `053` | nút làm mới, nhãn "ước lượng", Cmd+K |

## Quyết định chung của feature

1. Lõi `codeintel-*.ts` trung lập truyền tải; chỉ `agent-rpc-dispatch-codeintel.ts` (Part A) và `codeintel-relay-handlers.ts` (Part B) biết transport.
2. Mọi lệnh CLI là đối tượng có kiểu; không API argv tự do; `analyze|sync|index` chỉ ở `codeintel-reindex-commands.ts` và luôn `--index-only`; Cypher chỉ-đọc theo mẫu hằng.
3. Tên file theo khái niệm cụ thể (không `helpers/utils/common/misc`); không `max-lines` disable mới.
4. Chỉ lệnh Git ≤ 2.24 trong đường codeintel (baseline 2.25); không mạng; `GitCapabilityCache` cho `rev-parse --path-format`.
5. Không thêm dependency (SQLite qua `node:sqlite` tuỳ chọn). Windows trả `unsupported_platform`; SSH: agent chạy trên chính host nên đúng cho SSH/WSL.
6. Không tự đoán: `percent:null`, `warnings`, độ tin cậy theo tệp; mẫu/định dạng chưa chạy ghi "chưa kiểm chứng" và có task re-verify (002-04, 003-08, 004-09, 005-08).

## Lệch giữa CR và hợp đồng, điểm hợp đồng thiếu/mâu thuẫn (tổng hợp, không sửa hợp đồng)

1. Env của con codeintel: hợp đồng §2.4 chỉ nói `config.toolEnv + NO_COLOR=1` trong khi `toolEnv` chứa toàn bộ `process.env` (README v7 mục 8 điểm 22). 001-03 lọc biến theo mẫu bí mật: cần xác nhận.
2. Whitelist §9 mục 1 liệt kê verb rộng hơn (`query, trace, list, status`; CodeGraph `explore`) so với verb thực sự dùng; chỉ cài tập dùng.
3. `ORCA_CODEINTEL_DISABLED=1` không có `reason` trong bảng `TOOL_UNAVAILABLE`; `status` trên Windows (§1.1 lỗi) mâu thuẫn "luôn thành công" (§4.1).
4. §11 liệt kê sửa `branchCompare`; solution 005 không sửa (tự có `codeintel-merge-base-resolution.ts`).
5. Thứ tự giao các trường `indexScope/freshness/...` của `status` (CR-080) không nêu; 001 chừa điểm cắm.
6. Tên biến giới hạn riêng lẻ (`ORCA_CODEINTEL_TOOL_TIMEOUT_MS`…) là đề xuất; hợp đồng chỉ chốt tiền tố.
7. Điểm cắm `desktop/relay.ts` và vị trí script parity chưa có trong hợp đồng.

## Điểm chưa kiểm chứng (toàn feature)

Hành vi thật `gitnexus 1.6.9`, `codegraph 1.4.1` (cụt pipe, `%` tiến độ, huỷ giữa chừng, `affected --stdin`); 6 mẫu Cypher chưa chạy; `node:sqlite` trên Node đích; hiệu năng (mọi số là một lần đo); Windows/WSL; Part B thật; CI không chạy test `agent/` (`code-intel-contract` thuộc AG-CV-SOL-070); `npx tsc --noEmit` (53 lỗi có sẵn theo v4, chưa chạy lại).

## Tài liệu liên quan

- CR: [README feature](../../../../../../docs/crs/v7/agent-codeintel/README.md), `docs/crs/v7/README.md` mục 2, 3.2, 3.10, 6, 8.
- Hợp đồng: `specs/backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md`, `CONTRACT-codeintel-proto-and-data-map.md` (§1 PQ, §7, §8, §9, §10), `CONTRACT-codeintel-ui-api.md`.
- TDD: [v5/00-index](../../../../tdd/v5/00-index.md), [v5/07](../../../../tdd/v5/07-jsonrpc-dispatch.md), [v5/04](../../../../tdd/v5/04-handshake-session.md), [v5/05](../../../../tdd/v5/05-tool-registry.md); API: `specs/agent/api/gaps-and-findings.md`.
- Tasks: [../tasks/README.md](../tasks/README.md). Mẫu: `specs/agent/crs/v6/agent-capabilities/solutions/README.md`.
