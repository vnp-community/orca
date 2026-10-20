---
doc_id: SOL-PAY
doc_type: solution
title: Solution thanh toán
version: "1.0"
status: active
---

# Solution thanh toán

Change spec cho tính năng thanh toán: thêm hai endpoint REST và chuẩn hóa quy tắc đầu vào. Phần mô tả bên trên chỉ để người đọc; phần Phụ lục cuối tài liệu là đặc tả có cấu trúc mà agent sinh test specs đọc.

## 1. Thiết kế

MOD-Payment nhận yêu cầu thanh toán, kiểm tra đầu vào rồi gọi cổng thanh toán (INT-01) và ghi sổ qua MOD-Ledger.

## 2. Phụ lục: đặc tả có cấu trúc cho test specs

Endpoint do solution thêm hoặc sửa (cột `implements` là yêu cầu mà endpoint hiện thực):

| Mã | Method | Path | implements | Xác thực | `idempotent` |
| --- | --- | --- | --- | --- | --- |
| EP-PAY-CREATE | POST | /v1/payments | FR-001 | API key | true |
| EP-PAY-LIST | GET | /v1/payments | FR-002 | API key | true |

Trường của yêu cầu (một hàng một trường; bỏ trống ô nào không có ràng buộc):

| Mã | Của endpoint | name | Kiểu | required | min | max | minLength | maxLength | enum |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| FLD-PAY-AMOUNT | EP-PAY-CREATE | amount | integer | true | 1 | 50000000 | | | |
| FLD-PAY-ACCOUNT | EP-PAY-CREATE | accountId | string | true | | | 10 | 10 | |
| FLD-PAY-METHOD | EP-PAY-CREATE | method | string | true | | | | | CARD,WALLET |
| FLD-PAY-STATUS | EP-PAY-LIST | status | string | false | | | | | PENDING,DONE |

Phản hồi của endpoint:

| Mã | Phản hồi của | statusCode | errorCode | condition |
| --- | --- | --- | --- | --- |
| RSP-PAY-CREATE-201 | EP-PAY-CREATE | 201 | | Thanh toán hợp lệ, trả mã giao dịch |
| RSP-PAY-CREATE-400 | EP-PAY-CREATE | 400 | INVALID_REQUEST | Thiếu trường, sai kiểu hoặc số tiền không hợp lệ |
| RSP-PAY-CREATE-422 | EP-PAY-CREATE | 422 | LIMIT_EXCEEDED | Vượt hạn mức giao dịch |
| RSP-PAY-LIST-200 | EP-PAY-LIST | 200 | | Trả danh sách thanh toán của tài khoản |

Tiêu chí chấp nhận bổ sung (nguồn là solution, FR-002 chưa có tiêu chí trong SRS):

| Mã | Yêu cầu | Tiêu chí |
| --- | --- | --- |
| AC-PAY-FR002-1 | FR-002 | Danh sách trả mới nhất trước và chỉ gồm thanh toán của tài khoản gọi |
