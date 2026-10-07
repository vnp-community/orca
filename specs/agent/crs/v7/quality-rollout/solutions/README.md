# quality-rollout (v7) solutions: index (agent)

> ✅ **Đã triển khai (4/4 solutions, 31/31 tasks [x] DONE).** Đã hoàn tất triển khai mã nguồn, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận.

Solution phía `agent/` cho feature `quality-rollout` của series v7 "Xem code và kiểm soát chất lượng": [README feature](../../../../../../docs/crs/v7/quality-rollout/README.md), CR-CV-070 đến 073. Hợp đồng chuẩn tắc (thắng CR khi khác): [`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md) (chính), [`...-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (§8.2 tên solution), [`...-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md). Đối ứng backend: [`../../../../../backend-go/crs/v7/quality-rollout/`](../../../../../backend-go/crs/v7/quality-rollout/solutions/README.md).

## Bảng CR → Solution → Task

| CR | Solution | Nội dung phía agent | Task | Đối ứng |
|---|---|---|---|---|
| CR-CV-070 (P0) | [AG-CV-SOL-070-golden-fixtures-and-parsers](./AG-CV-SOL-070-golden-fixtures-and-parsers.md) | `mini-repo`, fixture `gitnexus 1.6.9`/`codegraph 1.4.1`, script chụp, test parser, guard phiên bản, tệp vàng C2, workflow `code-intel-contract` | 070-01 đến 09 | BE-CV-SOL-070-collector-golden-contract |
| CR-CV-071 (P1) | [AG-CV-SOL-071-perf-block-and-bench](./AG-CV-SOL-071-perf-block-and-bench.md) | khối `perf`, lấy mẫu RSS, test đồng thời, ngân sách, benchmark | 071-01 đến 07 | BE-CV-SOL-071-metrics-tracing-and-budgets |
| CR-CV-072 (P0) | [AG-CV-SOL-072-security-tests-agent](./AG-CV-SOL-072-security-tests-agent.md) | whitelist, Cypher, đường dẫn, phân giải repo, env/canary, fuzz xác định, CI | 072-01 đến 09 | BE-CV-SOL-072-security-tests-service-gateway |
| CR-CV-073 (P0) | [AG-CV-SOL-073-agent-kill-switch](./AG-CV-SOL-073-agent-kill-switch.md) | `ORCA_CODEINTEL_DISABLED` (+ `REINDEX`, `QUALITY_RUN`), cổng dispatcher, capability, runbook | 073-01 đến 06 | BE-CV-SOL-073-settings-flag-and-rollout; FE-CV-SOL-073-flag-gating-and-web-e2e |

Task: [../tasks/README.md](../tasks/README.md) (31 task).

## Thứ tự

`070` (ngay sau AG-CV-SOL-002/003, README v7 §7.2) → `071` dùng bộ chạy entry TS của 070-03 và workflow của 070-09; `072` dùng hạ tầng CI của 070; `073` độc lập, cắm vào dispatcher của AG-CV-SOL-001/081. Cả bốn cần mã của AG-CV-SOL-001 (khung `codeintel.*`) để cắm; nếu chưa có, viết phần thuần (kiểu, bộ đọc, hàm so sánh) trước.

## Quyết định chung

1. Test hợp đồng nằm trong `agent/src/relay/codeintel/` và chạy bằng `agent/vitest.code-intel-contract.config.ts` trong workflow riêng; **không sửa `pr.yml`** (đã trỏ `config/vitest.config.ts` không tồn tại; CI hiện không chạy test `agent/`).
2. Không thêm phụ thuộc (O12): không `fast-check`, `semver`, `tsx`, `tinybench`; script `.mjs` bundle entry TS bằng `esbuild` (đã dùng ở `agent/build.mjs`).
3. Hợp đồng thắng CR: hàng đợi 10 s `CODEINTEL_TIMEOUT queue_wait` (không 5 s/`RATE_LIMITED`), stdout 16 MiB (không 8 MiB), `reason` (không `detail`), whitelist theo hợp đồng §9.1.
4. Tệp vàng C2 do agent ghi (`ORCA_UPDATE_GOLDEN=1`), Go chỉ đọc; bố cục theo BE-CV-SOL-070.
5. `perf`, `startedAt` luôn bị loại khỏi so sánh vàng; `perf` không vào cache.
6. Công tắc tắt cứng fail closed; phương thức vẫn đăng ký (không `-32601`).
7. Chạy cho SSH (`--stdio`, Part A cùng mã); Windows trả `unsupported_platform`; không `max-lines` disable.

## Điểm hợp đồng thiếu hoặc mâu thuẫn (không tự sửa hợp đồng)

| # | Điểm | Nơi |
|---|---|---|
| 1 | `codeintel.status` thiếu `compatibility` (verified/untested/incompatible) và `warnings` ổn định: `tool_version_untested`, `index_built_with_old_extraction`, `codeintel_disabled` (BE-CV-SOL-070 cũng báo) | §2.2, §4.1 |
| 2 | `data` của `CODEINTEL_TOOL_FAILED` không có `command`; tập `perf.command` thiếu `check, trace, list, analyze, sync` (BE báo) | §2.2, §3.2 |
| 3 | Không có `reason` cho tắt cứng (`codeintel_disabled`) | §3.2 |
| 4 | Mô-đun phẳng `agent/src/relay/codeintel-*.ts` (§11) so với `agent/src/relay/codeintel/__fixtures__` | §11 |
| 5 | Env con `codeintel.*`: §2.4 (`toolEnv` toàn `process.env`) mâu thuẫn §10 và README v7 điểm 22 | §2.4, §10 |
| 6 | Hành vi khi hàng đợi đầy; tên env cho thời gian chờ hàng đợi; `perf` khi cache hit/lỗi | §2.2, §2.3 |
| 7 | Nơi xuất `CODEINTEL_METHODS`/`QUALITY_METHODS` và schema `validate` | §8, §9.1 |
| 8 | Registry GitNexus trùng: hợp đồng "indexedAt mới nhất" so với CR-072 "từ chối"; cách huỷ cache khi registry đổi | §9.2 |
| 9 | Từ cấm của `assertReadOnlyCypher` nằm trong literal (symbol tên `Set`) | §2.4 |
| 10 | `quality.*` có bị công tắc `ORCA_CODEINTEL_DISABLED` tắt không | §2.4 |

## Điểm chưa kiểm chứng (toàn feature)

- Mọi hình dạng đầu ra GitNexus 1.6.9/CodeGraph 1.4.1; `analyze --index-only` và `HOME` cô lập có giữ registry sạch; cách cài CodeGraph không tương tác trong CI.
- `pnpm install` phạm vi tối thiểu trên runner; thời gian chạy test; mọi ngưỡng hiệu năng/RSS (một lần đo, một máy).
- Cách biến `ORCA_*` tới tiến trình agent (systemd, `start.sh` không có trong repo, `--stdio`).
- Không có runner chuyên dụng cho benchmark đêm.
- Tool cũ `gitnexus`/`codegraph`/`shell` qua `tools/call` và `fs.*` tuyệt đối vẫn là bề mặt ngoài phạm vi v7.
