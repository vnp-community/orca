---
doc_id: SRS-PAY
doc_type: srs
title: SRS thanh toán
version: "1.0"
status: active
---

# SRS thanh toán

## 4. Yêu cầu chức năng

| Mã | Nội dung | priority | allocatedTo | realizedBy | satisfiedBy | Tiêu chí chấp nhận |
| --- | --- | --- | --- | --- | --- | --- |
| FR-001 | Hệ thống cho phép thanh toán đơn hàng qua cổng thanh toán | Must | MOD-Payment | SCN-01 | MOD-Payment | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch |
| FR-002 | Hệ thống cho phép xem lịch sử thanh toán | Should | MOD-Ledger | | MOD-Ledger | |

## 5. Yêu cầu phi chức năng

| Mã | category | Nội dung | targetValue | targetUnit | satisfiedBy | verifiedBy |
| --- | --- | --- | --- | --- | --- | --- |
| NFR-001 | performance_efficiency | Thời gian đáp ứng thanh toán p95 dưới 200 ms | 200 | ms | MECH-01 | TST-01 |

## 6. Vòng đời đơn hàng

| Mã | Tên | governs | initial |
| --- | --- | --- | --- |
| SM-001 | Vòng đời đơn hàng | DE-ORDER | NEW |

| Mã | Máy trạng thái | from | to | trigger | guard | effect |
| --- | --- | --- | --- | --- | --- | --- |
| TRN-001 | SM-001 | NEW | PAID | pay | số dư đủ | ghi sổ |
| TRN-002 | SM-001 | PAID | REFUNDED | refund | trong 30 ngày | hoàn tiền |
