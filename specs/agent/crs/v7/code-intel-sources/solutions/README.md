# code-intel-sources (v7) solutions: index (agent)

> ✅ **Đã triển khai (1/1 solution, 7/7 tasks [x] DONE).** Đã hoàn tất triển khai mã nguồn, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận. Feature `code-intel-sources` ở khu vực agent chỉ có một CR: [CR-CV-037](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-037-structure-analysis.md) (method `codeintel.structuralFacts`). Các CR 030-036, 038 không có việc ở `agent/` (hợp đồng §8.2).

| CR | Solution | Task |
|---|---|---|
| CR-CV-037 | [AG-CV-SOL-037-structural-facts](./AG-CV-SOL-037-structural-facts.md) | 037-01 đến 07 |

Hợp đồng: [`CONTRACT-codeintel-agent-rpc.md` §4.9](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md). Điều kiện trước: `AG-CV-SOL-001` (nền, whitelist, envelope) và `AG-CV-SOL-002` (runner Cypher, `SymbolRef`) trong `../agent-codeintel/solutions/`. Phía backend: `BE-CV-SOL-037-structure-findings-and-dismissals`. Đợt 5 (hợp đồng §7.3). Chỗ cần sửa hợp đồng: tên hàm dài nhất cho `fileSizes` (`longest.name`) chưa có cách lấy chắc chắn; mã `warnings` tạm.
