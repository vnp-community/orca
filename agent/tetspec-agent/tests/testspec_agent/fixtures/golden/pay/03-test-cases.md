---
spec: TSD-PAY-TestCases
feature: PAY
docKind: TestCases
version: 1
status: Draft
supersedes: none
baseline: b-c845d6c1c614
changes: []
llm: unknown
llm_model: ""
contentHash: 6635d0a1e24ad1e7c49916fb918f36de66ae9581c24a61b68ac41b892f50de87
---
# Test Cases: PAY

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-PAY-TestCases_v1 |
| Feature | PAY |
| Phiên bản | 1 |
| Trạng thái | Draft |
| Baseline TDD | TDD-PAY (b-c845d6c1c614) |
| CR của phiên bản này | — |
| Ontology | tso v1 |

## 1. Quy ước

- Bước: given (chuẩn bị), when (hành động), then (quan sát), cleanup (dọn).
- Kết quả mong đợi của assertion lấy từ oracle; dữ liệu nhạy cảm chỉ ghi `secretRef:<tên>`.
- Mỗi test case kèm một khối TCIR (`tcir/v1`) cho agent sinh test script: mã kết quả mong đợi `<TC>.E<n>`, mã bước `<TC>.S<n>`; dữ liệu chỉ ghi lớp giá trị, test script tự sinh dữ liệu khi chạy.

## 2. Test Cases theo mức

### 2.1 L1: Nghiệp vụ đầu-cuối

#### TC-L1-PAY-SCN-01-errorHandling-fail-INT-01-c

Chứng minh: Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L1-SCN-01-errorHandling-fail-INT-01-c` | `TSET-PAY-L1-SCN-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng |  | SCN-01 |
| 3 | then | Quan sát kết quả: FB-01 |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | FB-01 |  | `TORC-PAY-FB-01-fail-SCN-01` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L1-PAY-SCN-01-errorHandling-fail-INT-01-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng",
 "level": "L1",
 "priority": "P1",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L1-PAY-SCN-01-errorHandling-fail-INT-01-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L1-PAY-SCN-01-errorHandling-fail-INT-01-c.S2",
   "kind": "when",
   "action": "Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng",
   "target": "SCN-01"
  },
  {
   "id": "TC-L1-PAY-SCN-01-errorHandling-fail-INT-01-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: FB-01"
  }
 ],
 "expected": [
  {
   "id": "TC-L1-PAY-SCN-01-errorHandling-fail-INT-01-c.E1",
   "assertion": "TAS-L1-PAY-SCN-01-errorHandling-fail-INT-01-c-1",
   "kind": "errorCode",
   "op": "==",
   "value": "FB-01",
   "description": "Kết quả khớp oracle TORC-PAY-FB-01-fail-SCN-01",
   "oracle": {
    "code": "TORC-PAY-FB-01-fail-SCN-01",
    "kind": "errorContract",
    "expected": "FB-01"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L1-SCN-01-errorHandling-fail-INT-01-c"
  ],
  "conditions": [
   "TCND-PAY-SCN-01-errorHandling-fail-INT-01"
  ],
  "requirements": [
   "TR-PAY-L1-SCN-01-reliability"
  ],
  "basis": [
   "SCN-01"
  ],
  "testSet": [
   "TSET-PAY-L1-SCN-01"
  ]
 }
}
```

#### TC-L1-PAY-SCN-01-functional-main-c

Chứng minh: Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L1-SCN-01-functional-main-c` | `TSET-PAY-L1-SCN-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ |  | SCN-01 |
| 3 | then | Quan sát kết quả: MOD-Ledger ghi sổ |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | MOD-Ledger ghi sổ |  | `TORC-PAY-SCN-01-main` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L1-PAY-SCN-01-functional-main-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ",
 "level": "L1",
 "priority": "P1",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L1-PAY-SCN-01-functional-main-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L1-PAY-SCN-01-functional-main-c.S2",
   "kind": "when",
   "action": "Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ",
   "target": "SCN-01"
  },
  {
   "id": "TC-L1-PAY-SCN-01-functional-main-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: MOD-Ledger ghi sổ"
  }
 ],
 "expected": [
  {
   "id": "TC-L1-PAY-SCN-01-functional-main-c.E1",
   "assertion": "TAS-L1-PAY-SCN-01-functional-main-c-1",
   "kind": "state",
   "op": "==",
   "value": "MOD-Ledger ghi sổ",
   "description": "Kết quả khớp oracle TORC-PAY-SCN-01-main",
   "oracle": {
    "code": "TORC-PAY-SCN-01-main",
    "kind": "postcondition",
    "expected": "MOD-Ledger ghi sổ"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L1-SCN-01-functional-main-c"
  ],
  "conditions": [
   "TCND-PAY-SCN-01-functional-main"
  ],
  "requirements": [
   "TR-PAY-L1-SCN-01-functional"
  ],
  "basis": [
   "SCN-01"
  ],
  "testSet": [
   "TSET-PAY-L1-SCN-01"
  ]
 }
}
```

#### TC-L1-PAY-SCN-01-timing-budget-c

Chứng minh: Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L1-SCN-01-timing-budget-c` | `TSET-PAY-L1-SCN-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms |  | SCN-01 |
| 3 | then | Quan sát kết quả: ≤ 30000 ms |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | ≤ 30000 ms | ms | `TORC-PAY-SCN-01-budget` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L1-PAY-SCN-01-timing-budget-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms",
 "level": "L1",
 "priority": "P1",
 "testTypes": [
  "performance"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L1-PAY-SCN-01-timing-budget-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L1-PAY-SCN-01-timing-budget-c.S2",
   "kind": "when",
   "action": "Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms",
   "target": "SCN-01"
  },
  {
   "id": "TC-L1-PAY-SCN-01-timing-budget-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: ≤ 30000 ms"
  }
 ],
 "expected": [
  {
   "id": "TC-L1-PAY-SCN-01-timing-budget-c.E1",
   "assertion": "TAS-L1-PAY-SCN-01-timing-budget-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "≤ 30000 ms",
   "unit": "ms",
   "description": "Kết quả khớp oracle TORC-PAY-SCN-01-budget",
   "oracle": {
    "code": "TORC-PAY-SCN-01-budget",
    "kind": "threshold",
    "expected": "≤ 30000 ms"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L1-SCN-01-timing-budget-c"
  ],
  "conditions": [
   "TCND-PAY-SCN-01-timing-budget"
  ],
  "requirements": [
   "TR-PAY-L1-SCN-01-performance"
  ],
  "basis": [
   "SCN-01"
  ],
  "testSet": [
   "TSET-PAY-L1-SCN-01"
  ]
 }
}
```

#### TC-L1-PAY-SCN-01-timing-worst-INT-01-c

Chứng minh: Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L1-SCN-01-timing-worst-INT-01-c` | `TSET-PAY-L1-SCN-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms |  | SCN-01 |
| 3 | then | Quan sát kết quả: 75000 ms ≤ 30000 ms (ngân sách của SCN-01) |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | ms | `TORC-PAY-SCN-01-worst-INT-01` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L1-PAY-SCN-01-timing-worst-INT-01-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms",
 "level": "L1",
 "priority": "P1",
 "testTypes": [
  "performance"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L1-PAY-SCN-01-timing-worst-INT-01-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L1-PAY-SCN-01-timing-worst-INT-01-c.S2",
   "kind": "when",
   "action": "Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms",
   "target": "SCN-01"
  },
  {
   "id": "TC-L1-PAY-SCN-01-timing-worst-INT-01-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: 75000 ms ≤ 30000 ms (ngân sách của SCN-01)"
  }
 ],
 "expected": [
  {
   "id": "TC-L1-PAY-SCN-01-timing-worst-INT-01-c.E1",
   "assertion": "TAS-L1-PAY-SCN-01-timing-worst-INT-01-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "75000 ms ≤ 30000 ms (ngân sách của SCN-01)",
   "unit": "ms",
   "description": "Kết quả khớp oracle TORC-PAY-SCN-01-worst-INT-01",
   "oracle": {
    "code": "TORC-PAY-SCN-01-worst-INT-01",
    "kind": "threshold",
    "expected": "75000 ms ≤ 30000 ms (ngân sách của SCN-01)"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L1-SCN-01-timing-worst-INT-01-c"
  ],
  "conditions": [
   "TCND-PAY-SCN-01-timing-worst-INT-01"
  ],
  "requirements": [
   "TR-PAY-L1-SCN-01-performance"
  ],
  "basis": [
   "SCN-01"
  ],
  "testSet": [
   "TSET-PAY-L1-SCN-01"
  ]
 }
}
```

### 2.2 L2: Nghiệp vụ

#### TC-L2-PAY-AC-FR-001-1-functional-c

Chứng minh: Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-AC-FR-001-1-functional-c` | `TSET-PAY-L2-FR-001` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch |  | FR-001 |
| 3 | then | Quan sát kết quả: Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch |  | `TORC-PAY-AC-FR-001-1` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L2-PAY-AC-FR-001-1-functional-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch",
 "level": "L2",
 "priority": "P1",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L2-PAY-AC-FR-001-1-functional-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L2-PAY-AC-FR-001-1-functional-c.S2",
   "kind": "when",
   "action": "Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch",
   "target": "FR-001"
  },
  {
   "id": "TC-L2-PAY-AC-FR-001-1-functional-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch"
  }
 ],
 "expected": [
  {
   "id": "TC-L2-PAY-AC-FR-001-1-functional-c.E1",
   "assertion": "TAS-L2-PAY-AC-FR-001-1-functional-c-1",
   "kind": "value",
   "op": "==",
   "value": "Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch",
   "description": "Kết quả khớp oracle TORC-PAY-AC-FR-001-1",
   "oracle": {
    "code": "TORC-PAY-AC-FR-001-1",
    "kind": "acceptanceCriterion",
    "expected": "Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L2-AC-FR-001-1-functional-c"
  ],
  "conditions": [
   "TCND-PAY-AC-FR-001-1-functional"
  ],
  "requirements": [
   "TR-PAY-L2-FR-001-functional"
  ],
  "basis": [
   "AC-FR-001-1"
  ],
  "testSet": [
   "TSET-PAY-L2-FR-001"
  ]
 }
}
```

#### TC-L2-PAY-NFR-001-performance-target-c

Chứng minh: Thời gian đáp ứng thanh toán p95 dưới 200 ms

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-PAY-L2-NFR-001-performance-target-c` | `TSET-PAY-L2-NFR-001` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Thời gian đáp ứng thanh toán p95 dưới 200 ms |  | NFR-001 |
| 3 | then | Quan sát kết quả: Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms) |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms) | ms | `TORC-PAY-NFR-001` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L2-PAY-NFR-001-performance-target-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Thời gian đáp ứng thanh toán p95 dưới 200 ms",
 "level": "L2",
 "priority": "P2",
 "testTypes": [
  "performance"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L2-PAY-NFR-001-performance-target-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L2-PAY-NFR-001-performance-target-c.S2",
   "kind": "when",
   "action": "Thời gian đáp ứng thanh toán p95 dưới 200 ms",
   "target": "NFR-001"
  },
  {
   "id": "TC-L2-PAY-NFR-001-performance-target-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms)"
  }
 ],
 "expected": [
  {
   "id": "TC-L2-PAY-NFR-001-performance-target-c.E1",
   "assertion": "TAS-L2-PAY-NFR-001-performance-target-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms)",
   "unit": "ms",
   "description": "Kết quả khớp oracle TORC-PAY-NFR-001",
   "oracle": {
    "code": "TORC-PAY-NFR-001",
    "kind": "threshold",
    "expected": "Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms)"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L2-NFR-001-performance-target-c"
  ],
  "conditions": [
   "TCND-PAY-NFR-001-performance-target"
  ],
  "requirements": [
   "TR-PAY-L2-NFR-001-performance"
  ],
  "basis": [
   "NFR-001"
  ],
  "testSet": [
   "TSET-PAY-L2-NFR-001"
  ]
 }
}
```

#### TC-L2-PAY-TRN-001-guardViolation-guard-c

Chứng minh: pay khi không thỏa điều kiện: số dư đủ

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-TRN-001-guardViolation-guard-c` | `TSET-PAY-L2-SM-001` | rule |

Fixture: `TFX-PAY-TRN-001-5dce5f` Đưa hệ thống về: Trạng thái NEW, không thỏa số dư đủ / dọn: Xóa dữ liệu do fixture tạo

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị: Trạng thái NEW, không thỏa số dư đủ |  |  |
| 2 | when | pay |  | TRN-001 |
| 3 | then | Quan sát kết quả: Không chuyển sang PAID; giữ NEW |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Không chuyển sang PAID; giữ NEW |  | `TORC-PAY-TRN-001-guard` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L2-PAY-TRN-001-guardViolation-guard-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: pay khi không thỏa điều kiện: số dư đủ",
 "level": "L2",
 "priority": "P1",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [
  {
   "code": "TFX-PAY-TRN-001-5dce5f",
   "kind": "state",
   "setup": "Đưa hệ thống về: Trạng thái NEW, không thỏa số dư đủ",
   "teardown": "Xóa dữ liệu do fixture tạo"
  }
 ],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L2-PAY-TRN-001-guardViolation-guard-c.S1",
   "kind": "given",
   "action": "Chuẩn bị: Trạng thái NEW, không thỏa số dư đủ"
  },
  {
   "id": "TC-L2-PAY-TRN-001-guardViolation-guard-c.S2",
   "kind": "when",
   "action": "pay",
   "target": "TRN-001"
  },
  {
   "id": "TC-L2-PAY-TRN-001-guardViolation-guard-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Không chuyển sang PAID; giữ NEW"
  }
 ],
 "expected": [
  {
   "id": "TC-L2-PAY-TRN-001-guardViolation-guard-c.E1",
   "assertion": "TAS-L2-PAY-TRN-001-guardViolation-guard-c-1",
   "kind": "state",
   "op": "==",
   "value": "Không chuyển sang PAID; giữ NEW",
   "description": "Kết quả khớp oracle TORC-PAY-TRN-001-guard",
   "oracle": {
    "code": "TORC-PAY-TRN-001-guard",
    "kind": "stateMachine",
    "expected": "Không chuyển sang PAID; giữ NEW"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L2-TRN-001-guardViolation-guard-c"
  ],
  "conditions": [
   "TCND-PAY-TRN-001-guardViolation-guard"
  ],
  "requirements": [
   "TR-PAY-L2-SM-001-functional"
  ],
  "basis": [
   "TRN-001"
  ],
  "testSet": [
   "TSET-PAY-L2-SM-001"
  ]
 }
}
```

#### TC-L2-PAY-TRN-001-stateTransition-c

Chứng minh: NEW --pay--> PAID

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-TRN-001-stateTransition-c` | `TSET-PAY-L2-SM-001` | rule |

Fixture: `TFX-PAY-TRN-001-c5c90d` Đưa hệ thống về: Trạng thái NEW, số dư đủ / dọn: Xóa dữ liệu do fixture tạo

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị: Trạng thái NEW, số dư đủ |  |  |
| 2 | when | pay |  | TRN-001 |
| 3 | then | Quan sát kết quả: NEW → PAID; ghi sổ |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | NEW → PAID; ghi sổ |  | `TORC-PAY-TRN-001` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L2-PAY-TRN-001-stateTransition-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: NEW --pay--> PAID",
 "level": "L2",
 "priority": "P1",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [
  {
   "code": "TFX-PAY-TRN-001-c5c90d",
   "kind": "state",
   "setup": "Đưa hệ thống về: Trạng thái NEW, số dư đủ",
   "teardown": "Xóa dữ liệu do fixture tạo"
  }
 ],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L2-PAY-TRN-001-stateTransition-c.S1",
   "kind": "given",
   "action": "Chuẩn bị: Trạng thái NEW, số dư đủ"
  },
  {
   "id": "TC-L2-PAY-TRN-001-stateTransition-c.S2",
   "kind": "when",
   "action": "pay",
   "target": "TRN-001"
  },
  {
   "id": "TC-L2-PAY-TRN-001-stateTransition-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: NEW → PAID; ghi sổ"
  }
 ],
 "expected": [
  {
   "id": "TC-L2-PAY-TRN-001-stateTransition-c.E1",
   "assertion": "TAS-L2-PAY-TRN-001-stateTransition-c-1",
   "kind": "state",
   "op": "==",
   "value": "NEW → PAID; ghi sổ",
   "description": "Kết quả khớp oracle TORC-PAY-TRN-001",
   "oracle": {
    "code": "TORC-PAY-TRN-001",
    "kind": "stateMachine",
    "expected": "NEW → PAID; ghi sổ"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L2-TRN-001-stateTransition-c"
  ],
  "conditions": [
   "TCND-PAY-TRN-001-stateTransition"
  ],
  "requirements": [
   "TR-PAY-L2-SM-001-functional"
  ],
  "basis": [
   "TRN-001"
  ],
  "testSet": [
   "TSET-PAY-L2-SM-001"
  ]
 }
}
```

#### TC-L2-PAY-TRN-002-guardViolation-guard-c

Chứng minh: refund khi không thỏa điều kiện: trong 30 ngày

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-TRN-002-guardViolation-guard-c` | `TSET-PAY-L2-SM-001` | rule |

Fixture: `TFX-PAY-TRN-002-c873c0` Đưa hệ thống về: Trạng thái PAID, không thỏa trong 30 ngày / dọn: Xóa dữ liệu do fixture tạo

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị: Trạng thái PAID, không thỏa trong 30 ngày |  |  |
| 2 | when | refund |  | TRN-002 |
| 3 | then | Quan sát kết quả: Không chuyển sang REFUNDED; giữ PAID |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Không chuyển sang REFUNDED; giữ PAID |  | `TORC-PAY-TRN-002-guard` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L2-PAY-TRN-002-guardViolation-guard-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: refund khi không thỏa điều kiện: trong 30 ngày",
 "level": "L2",
 "priority": "P1",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [
  {
   "code": "TFX-PAY-TRN-002-c873c0",
   "kind": "state",
   "setup": "Đưa hệ thống về: Trạng thái PAID, không thỏa trong 30 ngày",
   "teardown": "Xóa dữ liệu do fixture tạo"
  }
 ],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L2-PAY-TRN-002-guardViolation-guard-c.S1",
   "kind": "given",
   "action": "Chuẩn bị: Trạng thái PAID, không thỏa trong 30 ngày"
  },
  {
   "id": "TC-L2-PAY-TRN-002-guardViolation-guard-c.S2",
   "kind": "when",
   "action": "refund",
   "target": "TRN-002"
  },
  {
   "id": "TC-L2-PAY-TRN-002-guardViolation-guard-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Không chuyển sang REFUNDED; giữ PAID"
  }
 ],
 "expected": [
  {
   "id": "TC-L2-PAY-TRN-002-guardViolation-guard-c.E1",
   "assertion": "TAS-L2-PAY-TRN-002-guardViolation-guard-c-1",
   "kind": "state",
   "op": "==",
   "value": "Không chuyển sang REFUNDED; giữ PAID",
   "description": "Kết quả khớp oracle TORC-PAY-TRN-002-guard",
   "oracle": {
    "code": "TORC-PAY-TRN-002-guard",
    "kind": "stateMachine",
    "expected": "Không chuyển sang REFUNDED; giữ PAID"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L2-TRN-002-guardViolation-guard-c"
  ],
  "conditions": [
   "TCND-PAY-TRN-002-guardViolation-guard"
  ],
  "requirements": [
   "TR-PAY-L2-SM-001-functional"
  ],
  "basis": [
   "TRN-002"
  ],
  "testSet": [
   "TSET-PAY-L2-SM-001"
  ]
 }
}
```

#### TC-L2-PAY-TRN-002-stateTransition-c

Chứng minh: PAID --refund--> REFUNDED

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-TRN-002-stateTransition-c` | `TSET-PAY-L2-SM-001` | rule |

Fixture: `TFX-PAY-TRN-002-f90e96` Đưa hệ thống về: Trạng thái PAID, trong 30 ngày / dọn: Xóa dữ liệu do fixture tạo

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị: Trạng thái PAID, trong 30 ngày |  |  |
| 2 | when | refund |  | TRN-002 |
| 3 | then | Quan sát kết quả: PAID → REFUNDED; hoàn tiền |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | PAID → REFUNDED; hoàn tiền |  | `TORC-PAY-TRN-002` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L2-PAY-TRN-002-stateTransition-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: PAID --refund--> REFUNDED",
 "level": "L2",
 "priority": "P1",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [
  {
   "code": "TFX-PAY-TRN-002-f90e96",
   "kind": "state",
   "setup": "Đưa hệ thống về: Trạng thái PAID, trong 30 ngày",
   "teardown": "Xóa dữ liệu do fixture tạo"
  }
 ],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L2-PAY-TRN-002-stateTransition-c.S1",
   "kind": "given",
   "action": "Chuẩn bị: Trạng thái PAID, trong 30 ngày"
  },
  {
   "id": "TC-L2-PAY-TRN-002-stateTransition-c.S2",
   "kind": "when",
   "action": "refund",
   "target": "TRN-002"
  },
  {
   "id": "TC-L2-PAY-TRN-002-stateTransition-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: PAID → REFUNDED; hoàn tiền"
  }
 ],
 "expected": [
  {
   "id": "TC-L2-PAY-TRN-002-stateTransition-c.E1",
   "assertion": "TAS-L2-PAY-TRN-002-stateTransition-c-1",
   "kind": "state",
   "op": "==",
   "value": "PAID → REFUNDED; hoàn tiền",
   "description": "Kết quả khớp oracle TORC-PAY-TRN-002",
   "oracle": {
    "code": "TORC-PAY-TRN-002",
    "kind": "stateMachine",
    "expected": "PAID → REFUNDED; hoàn tiền"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L2-TRN-002-stateTransition-c"
  ],
  "conditions": [
   "TCND-PAY-TRN-002-stateTransition"
  ],
  "requirements": [
   "TR-PAY-L2-SM-001-functional"
  ],
  "basis": [
   "TRN-002"
  ],
  "testSet": [
   "TSET-PAY-L2-SM-001"
  ]
 }
}
```

#### TC-L2-PAY-UR-001-functional-trace-c

Chứng minh: Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001)

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-UR-001-functional-trace-c` | `TSET-PAY-L2-UR-001` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001) |  | UR-001 |
| 3 | then | Quan sát kết quả: Các FR truy vết đều đạt: FR-001 |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Các FR truy vết đều đạt: FR-001 |  | `TORC-PAY-UR-001-trace` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L2-PAY-UR-001-functional-trace-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001)",
 "level": "L2",
 "priority": "P1",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L2-PAY-UR-001-functional-trace-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L2-PAY-UR-001-functional-trace-c.S2",
   "kind": "when",
   "action": "Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001)",
   "target": "UR-001"
  },
  {
   "id": "TC-L2-PAY-UR-001-functional-trace-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Các FR truy vết đều đạt: FR-001"
  }
 ],
 "expected": [
  {
   "id": "TC-L2-PAY-UR-001-functional-trace-c.E1",
   "assertion": "TAS-L2-PAY-UR-001-functional-trace-c-1",
   "kind": "value",
   "op": "==",
   "value": "Các FR truy vết đều đạt: FR-001",
   "description": "Kết quả khớp oracle TORC-PAY-UR-001-trace",
   "oracle": {
    "code": "TORC-PAY-UR-001-trace",
    "kind": "reference",
    "expected": "Các FR truy vết đều đạt: FR-001"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L2-UR-001-functional-trace-c"
  ],
  "conditions": [
   "TCND-PAY-UR-001-functional-trace"
  ],
  "requirements": [
   "TR-PAY-L2-UR-001-functional"
  ],
  "basis": [
   "UR-001"
  ],
  "testSet": [
   "TSET-PAY-L2-UR-001"
  ]
 }
}
```

### 2.3 L3: Cô lập service

#### TC-L3-PAY-CB-01-stateTransition-close-c

Chứng minh: Lượt thử khi nửa mở thành công

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-CB-01-stateTransition-close-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Lượt thử khi nửa mở thành công |  | CB-01 |
| 3 | then | Quan sát kết quả: Breaker đóng, lời gọi đi bình thường |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Breaker đóng, lời gọi đi bình thường |  | `TORC-PAY-CB-01-close` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-CB-01-stateTransition-close-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Lượt thử khi nửa mở thành công",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-close-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-close-c.S2",
   "kind": "when",
   "action": "Lượt thử khi nửa mở thành công",
   "target": "CB-01"
  },
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-close-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Breaker đóng, lời gọi đi bình thường"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-close-c.E1",
   "assertion": "TAS-L3-PAY-CB-01-stateTransition-close-c-1",
   "kind": "state",
   "op": "==",
   "value": "Breaker đóng, lời gọi đi bình thường",
   "description": "Kết quả khớp oracle TORC-PAY-CB-01-close",
   "oracle": {
    "code": "TORC-PAY-CB-01-close",
    "kind": "stateMachine",
    "expected": "Breaker đóng, lời gọi đi bình thường"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-CB-01-stateTransition-close-c"
  ],
  "conditions": [
   "TCND-PAY-CB-01-stateTransition-close"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-functional"
  ],
  "basis": [
   "CB-01"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-CB-01-stateTransition-halfopen-c

Chứng minh: Hết 60 giây mở

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-CB-01-stateTransition-halfopen-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Hết 60 giây mở |  | CB-01 |
| 3 | then | Quan sát kết quả: Breaker nửa mở, cho một lượt thử |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Breaker nửa mở, cho một lượt thử |  | `TORC-PAY-CB-01-halfopen` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-CB-01-stateTransition-halfopen-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Hết 60 giây mở",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-halfopen-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-halfopen-c.S2",
   "kind": "when",
   "action": "Hết 60 giây mở",
   "target": "CB-01"
  },
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-halfopen-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Breaker nửa mở, cho một lượt thử"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-halfopen-c.E1",
   "assertion": "TAS-L3-PAY-CB-01-stateTransition-halfopen-c-1",
   "kind": "state",
   "op": "==",
   "value": "Breaker nửa mở, cho một lượt thử",
   "description": "Kết quả khớp oracle TORC-PAY-CB-01-halfopen",
   "oracle": {
    "code": "TORC-PAY-CB-01-halfopen",
    "kind": "stateMachine",
    "expected": "Breaker nửa mở, cho một lượt thử"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-CB-01-stateTransition-halfopen-c"
  ],
  "conditions": [
   "TCND-PAY-CB-01-stateTransition-halfopen"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-functional"
  ],
  "basis": [
   "CB-01"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-CB-01-stateTransition-open-c

Chứng minh: Lỗi đạt 50% lỗi trong tối thiểu 10 lượt

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-CB-01-stateTransition-open-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Lỗi đạt 50% lỗi trong tối thiểu 10 lượt |  | CB-01 |
| 3 | then | Quan sát kết quả: Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán |  | `TORC-PAY-CB-01-open` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-CB-01-stateTransition-open-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Lỗi đạt 50% lỗi trong tối thiểu 10 lượt",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-open-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-open-c.S2",
   "kind": "when",
   "action": "Lỗi đạt 50% lỗi trong tối thiểu 10 lượt",
   "target": "CB-01"
  },
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-open-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-CB-01-stateTransition-open-c.E1",
   "assertion": "TAS-L3-PAY-CB-01-stateTransition-open-c-1",
   "kind": "state",
   "op": "==",
   "value": "Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán",
   "description": "Kết quả khớp oracle TORC-PAY-CB-01-open",
   "oracle": {
    "code": "TORC-PAY-CB-01-open",
    "kind": "stateMachine",
    "expected": "Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-CB-01-stateTransition-open-c"
  ],
  "conditions": [
   "TCND-PAY-CB-01-stateTransition-open"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-functional"
  ],
  "basis": [
   "CB-01"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-ERR-01-429-errorHandling-c

Chứng minh: Cổng thanh toán trả HTTP 429

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-ERR-01-429-errorHandling-c` | `TSET-PAY-L3-INT-01` | rule |

Stub: `TSTUB-PAY-INT-01-errorTransient-429` INT-01 → errorTransient (429)

Tiêm lỗi: `TFI-PAY-INT-01-err-ERR-01-429` Stub INT-01 tạo lỗi transient — Tại INT-01, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: transient tại INT-01 (before) |  |  |
| 3 | when | Cổng thanh toán trả HTTP 429 |  | ERR-01-429 |
| 4 | then | Quan sát kết quả: chờ Retry-After rồi thử lại |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | chờ Retry-After rồi thử lại |  | `TORC-PAY-ERR-01-429` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-ERR-01-429-errorHandling-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Cổng thanh toán trả HTTP 429",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [
  {
   "code": "TSTUB-PAY-INT-01-errorTransient-429",
   "target": "INT-01",
   "behavior": "errorTransient",
   "statusCode": 429
  }
 ],
 "faultInjections": [
  {
   "code": "TFI-PAY-INT-01-err-ERR-01-429",
   "mechanism": "Stub INT-01 tạo lỗi transient",
   "trigger": "Tại INT-01, trước khi thao tác chính ghi dữ liệu"
  }
 ],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-ERR-01-429-errorHandling-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-ERR-01-429-errorHandling-c.S2",
   "kind": "given",
   "action": "Bật tiêm lỗi: transient tại INT-01 (before)"
  },
  {
   "id": "TC-L3-PAY-ERR-01-429-errorHandling-c.S3",
   "kind": "when",
   "action": "Cổng thanh toán trả HTTP 429",
   "target": "ERR-01-429"
  },
  {
   "id": "TC-L3-PAY-ERR-01-429-errorHandling-c.S4",
   "kind": "then",
   "action": "Quan sát kết quả: chờ Retry-After rồi thử lại"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-ERR-01-429-errorHandling-c.E1",
   "assertion": "TAS-L3-PAY-ERR-01-429-errorHandling-c-1",
   "kind": "errorCode",
   "op": "==",
   "value": "chờ Retry-After rồi thử lại",
   "description": "Kết quả khớp oracle TORC-PAY-ERR-01-429",
   "oracle": {
    "code": "TORC-PAY-ERR-01-429",
    "kind": "errorContract",
    "expected": "chờ Retry-After rồi thử lại"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-ERR-01-429-errorHandling-c"
  ],
  "conditions": [
   "TCND-PAY-ERR-01-429-errorHandling"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-reliability"
  ],
  "basis": [
   "ERR-01-429"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-ERR-01-4xx-errorHandling-c

Chứng minh: Cổng thanh toán trả HTTP 400

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-ERR-01-4xx-errorHandling-c` | `TSET-PAY-L3-INT-01` | rule |

Stub: `TSTUB-PAY-INT-01-errorPermanent-400` INT-01 → errorPermanent (400)

Tiêm lỗi: `TFI-PAY-INT-01-err-ERR-01-4xx` Stub INT-01 tạo lỗi permanent — Tại INT-01, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: permanent tại INT-01 (before) |  |  |
| 3 | when | Cổng thanh toán trả HTTP 400 |  | ERR-01-4xx |
| 4 | then | Quan sát kết quả: trả lỗi cho người dùng |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | trả lỗi cho người dùng |  | `TORC-PAY-ERR-01-4xx` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-ERR-01-4xx-errorHandling-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Cổng thanh toán trả HTTP 400",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [
  {
   "code": "TSTUB-PAY-INT-01-errorPermanent-400",
   "target": "INT-01",
   "behavior": "errorPermanent",
   "statusCode": 400
  }
 ],
 "faultInjections": [
  {
   "code": "TFI-PAY-INT-01-err-ERR-01-4xx",
   "mechanism": "Stub INT-01 tạo lỗi permanent",
   "trigger": "Tại INT-01, trước khi thao tác chính ghi dữ liệu"
  }
 ],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-ERR-01-4xx-errorHandling-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-ERR-01-4xx-errorHandling-c.S2",
   "kind": "given",
   "action": "Bật tiêm lỗi: permanent tại INT-01 (before)"
  },
  {
   "id": "TC-L3-PAY-ERR-01-4xx-errorHandling-c.S3",
   "kind": "when",
   "action": "Cổng thanh toán trả HTTP 400",
   "target": "ERR-01-4xx"
  },
  {
   "id": "TC-L3-PAY-ERR-01-4xx-errorHandling-c.S4",
   "kind": "then",
   "action": "Quan sát kết quả: trả lỗi cho người dùng"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-ERR-01-4xx-errorHandling-c.E1",
   "assertion": "TAS-L3-PAY-ERR-01-4xx-errorHandling-c-1",
   "kind": "errorCode",
   "op": "==",
   "value": "trả lỗi cho người dùng",
   "description": "Kết quả khớp oracle TORC-PAY-ERR-01-4xx",
   "oracle": {
    "code": "TORC-PAY-ERR-01-4xx",
    "kind": "errorContract",
    "expected": "trả lỗi cho người dùng"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-ERR-01-4xx-errorHandling-c"
  ],
  "conditions": [
   "TCND-PAY-ERR-01-4xx-errorHandling"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-reliability"
  ],
  "basis": [
   "ERR-01-4xx"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-ERR-01-5xx-errorHandling-c

Chứng minh: Cổng thanh toán trả HTTP 503

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-ERR-01-5xx-errorHandling-c` | `TSET-PAY-L3-INT-01` | rule |

Stub: `TSTUB-PAY-INT-01-errorTransient-503` INT-01 → errorTransient (503)

Tiêm lỗi: `TFI-PAY-INT-01-err-ERR-01-5xx` Stub INT-01 tạo lỗi transient — Tại INT-01, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: transient tại INT-01 (before) |  |  |
| 3 | when | Cổng thanh toán trả HTTP 503 |  | ERR-01-5xx |
| 4 | then | Quan sát kết quả: backoff rồi thử lại |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | backoff rồi thử lại |  | `TORC-PAY-ERR-01-5xx` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-ERR-01-5xx-errorHandling-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Cổng thanh toán trả HTTP 503",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [
  {
   "code": "TSTUB-PAY-INT-01-errorTransient-503",
   "target": "INT-01",
   "behavior": "errorTransient",
   "statusCode": 503
  }
 ],
 "faultInjections": [
  {
   "code": "TFI-PAY-INT-01-err-ERR-01-5xx",
   "mechanism": "Stub INT-01 tạo lỗi transient",
   "trigger": "Tại INT-01, trước khi thao tác chính ghi dữ liệu"
  }
 ],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-ERR-01-5xx-errorHandling-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-ERR-01-5xx-errorHandling-c.S2",
   "kind": "given",
   "action": "Bật tiêm lỗi: transient tại INT-01 (before)"
  },
  {
   "id": "TC-L3-PAY-ERR-01-5xx-errorHandling-c.S3",
   "kind": "when",
   "action": "Cổng thanh toán trả HTTP 503",
   "target": "ERR-01-5xx"
  },
  {
   "id": "TC-L3-PAY-ERR-01-5xx-errorHandling-c.S4",
   "kind": "then",
   "action": "Quan sát kết quả: backoff rồi thử lại"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-ERR-01-5xx-errorHandling-c.E1",
   "assertion": "TAS-L3-PAY-ERR-01-5xx-errorHandling-c-1",
   "kind": "errorCode",
   "op": "==",
   "value": "backoff rồi thử lại",
   "description": "Kết quả khớp oracle TORC-PAY-ERR-01-5xx",
   "oracle": {
    "code": "TORC-PAY-ERR-01-5xx",
    "kind": "errorContract",
    "expected": "backoff rồi thử lại"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-ERR-01-5xx-errorHandling-c"
  ],
  "conditions": [
   "TCND-PAY-ERR-01-5xx-errorHandling"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-reliability"
  ],
  "basis": [
   "ERR-01-5xx"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-ERR-01-timeout-errorHandling-c

Chứng minh: Cổng thanh toán trả readMs hết hạn

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-ERR-01-timeout-errorHandling-c` | `TSET-PAY-L3-INT-01` | rule |

Stub: `TSTUB-PAY-INT-01-timeout` INT-01 → timeout

Tiêm lỗi: `TFI-PAY-INT-01-err-ERR-01-timeout` Stub INT-01 tạo lỗi unknownOutcome — Tại INT-01, ngay sau khi giao dịch đã commit, trước khi trả kết quả

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: unknownOutcome tại INT-01 (afterCommit) |  |  |
| 3 | when | Cổng thanh toán trả readMs hết hạn |  | ERR-01-timeout |
| 4 | then | Quan sát kết quả: tra cứu trạng thái giao dịch |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | tra cứu trạng thái giao dịch |  | `TORC-PAY-ERR-01-timeout` |
| 2 | count | == | Hiệu ứng phụ sau commit đúng một lần: tra cứu trạng thái giao dịch |  | `TORC-PAY-ERR-01-timeout` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-ERR-01-timeout-errorHandling-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Cổng thanh toán trả readMs hết hạn",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [
  {
   "code": "TSTUB-PAY-INT-01-timeout",
   "target": "INT-01",
   "behavior": "timeout"
  }
 ],
 "faultInjections": [
  {
   "code": "TFI-PAY-INT-01-err-ERR-01-timeout",
   "mechanism": "Stub INT-01 tạo lỗi unknownOutcome",
   "trigger": "Tại INT-01, ngay sau khi giao dịch đã commit, trước khi trả kết quả"
  }
 ],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-ERR-01-timeout-errorHandling-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-ERR-01-timeout-errorHandling-c.S2",
   "kind": "given",
   "action": "Bật tiêm lỗi: unknownOutcome tại INT-01 (afterCommit)"
  },
  {
   "id": "TC-L3-PAY-ERR-01-timeout-errorHandling-c.S3",
   "kind": "when",
   "action": "Cổng thanh toán trả readMs hết hạn",
   "target": "ERR-01-timeout"
  },
  {
   "id": "TC-L3-PAY-ERR-01-timeout-errorHandling-c.S4",
   "kind": "then",
   "action": "Quan sát kết quả: tra cứu trạng thái giao dịch"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-ERR-01-timeout-errorHandling-c.E1",
   "assertion": "TAS-L3-PAY-ERR-01-timeout-errorHandling-c-1",
   "kind": "errorCode",
   "op": "==",
   "value": "tra cứu trạng thái giao dịch",
   "description": "Kết quả khớp oracle TORC-PAY-ERR-01-timeout",
   "oracle": {
    "code": "TORC-PAY-ERR-01-timeout",
    "kind": "errorContract",
    "expected": "tra cứu trạng thái giao dịch"
   }
  },
  {
   "id": "TC-L3-PAY-ERR-01-timeout-errorHandling-c.E2",
   "assertion": "TAS-L3-PAY-ERR-01-timeout-errorHandling-c-2",
   "kind": "count",
   "op": "==",
   "value": "Hiệu ứng phụ sau commit đúng một lần: tra cứu trạng thái giao dịch",
   "description": "Không trùng, không mất hiệu ứng phụ sau commit",
   "oracle": {
    "code": "TORC-PAY-ERR-01-timeout",
    "kind": "errorContract",
    "expected": "tra cứu trạng thái giao dịch"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-ERR-01-timeout-errorHandling-c"
  ],
  "conditions": [
   "TCND-PAY-ERR-01-timeout-errorHandling"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-reliability"
  ],
  "basis": [
   "ERR-01-timeout"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-ERR-02-bad-errorHandling-c

Chứng minh: API thanh toán trả HTTP 401

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-ERR-02-bad-errorHandling-c` | `TSET-PAY-L3-INT-02` | rule |

Stub: `TSTUB-PAY-INT-02-errorPermanent-401` INT-02 → errorPermanent (401)

Tiêm lỗi: `TFI-PAY-INT-02-err-ERR-02-bad` Stub INT-02 tạo lỗi permanent — Tại INT-02, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: permanent tại INT-02 (before) |  |  |
| 3 | when | API thanh toán trả HTTP 401 |  | ERR-02-bad |
| 4 | then | Quan sát kết quả: client sửa thông tin xác thực |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | client sửa thông tin xác thực |  | `TORC-PAY-ERR-02-bad` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-ERR-02-bad-errorHandling-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: API thanh toán trả HTTP 401",
 "level": "L3",
 "priority": "P1",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [
  {
   "code": "TSTUB-PAY-INT-02-errorPermanent-401",
   "target": "INT-02",
   "behavior": "errorPermanent",
   "statusCode": 401
  }
 ],
 "faultInjections": [
  {
   "code": "TFI-PAY-INT-02-err-ERR-02-bad",
   "mechanism": "Stub INT-02 tạo lỗi permanent",
   "trigger": "Tại INT-02, trước khi thao tác chính ghi dữ liệu"
  }
 ],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-ERR-02-bad-errorHandling-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-ERR-02-bad-errorHandling-c.S2",
   "kind": "given",
   "action": "Bật tiêm lỗi: permanent tại INT-02 (before)"
  },
  {
   "id": "TC-L3-PAY-ERR-02-bad-errorHandling-c.S3",
   "kind": "when",
   "action": "API thanh toán trả HTTP 401",
   "target": "ERR-02-bad"
  },
  {
   "id": "TC-L3-PAY-ERR-02-bad-errorHandling-c.S4",
   "kind": "then",
   "action": "Quan sát kết quả: client sửa thông tin xác thực"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-ERR-02-bad-errorHandling-c.E1",
   "assertion": "TAS-L3-PAY-ERR-02-bad-errorHandling-c-1",
   "kind": "errorCode",
   "op": "==",
   "value": "client sửa thông tin xác thực",
   "description": "Kết quả khớp oracle TORC-PAY-ERR-02-bad",
   "oracle": {
    "code": "TORC-PAY-ERR-02-bad",
    "kind": "errorContract",
    "expected": "client sửa thông tin xác thực"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-ERR-02-bad-errorHandling-c"
  ],
  "conditions": [
   "TCND-PAY-ERR-02-bad-errorHandling"
  ],
  "requirements": [
   "TR-PAY-L3-INT-02-reliability"
  ],
  "basis": [
   "ERR-02-bad"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-02"
  ]
 }
}
```

#### TC-L3-PAY-ERR-02-busy-errorHandling-c

Chứng minh: API thanh toán trả HTTP 503

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-ERR-02-busy-errorHandling-c` | `TSET-PAY-L3-INT-02` | rule |

Stub: `TSTUB-PAY-INT-02-errorTransient-503` INT-02 → errorTransient (503)

Tiêm lỗi: `TFI-PAY-INT-02-err-ERR-02-busy` Stub INT-02 tạo lỗi transient — Tại INT-02, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: transient tại INT-02 (before) |  |  |
| 3 | when | API thanh toán trả HTTP 503 |  | ERR-02-busy |
| 4 | then | Quan sát kết quả: client thử lại có backoff |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | client thử lại có backoff |  | `TORC-PAY-ERR-02-busy` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-ERR-02-busy-errorHandling-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: API thanh toán trả HTTP 503",
 "level": "L3",
 "priority": "P1",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [
  {
   "code": "TSTUB-PAY-INT-02-errorTransient-503",
   "target": "INT-02",
   "behavior": "errorTransient",
   "statusCode": 503
  }
 ],
 "faultInjections": [
  {
   "code": "TFI-PAY-INT-02-err-ERR-02-busy",
   "mechanism": "Stub INT-02 tạo lỗi transient",
   "trigger": "Tại INT-02, trước khi thao tác chính ghi dữ liệu"
  }
 ],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-ERR-02-busy-errorHandling-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-ERR-02-busy-errorHandling-c.S2",
   "kind": "given",
   "action": "Bật tiêm lỗi: transient tại INT-02 (before)"
  },
  {
   "id": "TC-L3-PAY-ERR-02-busy-errorHandling-c.S3",
   "kind": "when",
   "action": "API thanh toán trả HTTP 503",
   "target": "ERR-02-busy"
  },
  {
   "id": "TC-L3-PAY-ERR-02-busy-errorHandling-c.S4",
   "kind": "then",
   "action": "Quan sát kết quả: client thử lại có backoff"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-ERR-02-busy-errorHandling-c.E1",
   "assertion": "TAS-L3-PAY-ERR-02-busy-errorHandling-c-1",
   "kind": "errorCode",
   "op": "==",
   "value": "client thử lại có backoff",
   "description": "Kết quả khớp oracle TORC-PAY-ERR-02-busy",
   "oracle": {
    "code": "TORC-PAY-ERR-02-busy",
    "kind": "errorContract",
    "expected": "client thử lại có backoff"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-ERR-02-busy-errorHandling-c"
  ],
  "conditions": [
   "TCND-PAY-ERR-02-busy-errorHandling"
  ],
  "requirements": [
   "TR-PAY-L3-INT-02-reliability"
  ],
  "basis": [
   "ERR-02-busy"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-02"
  ]
 }
}
```

#### TC-L3-PAY-ERR-02-lost-errorHandling-c

Chứng minh: API thanh toán trả client hết thời gian chờ

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-ERR-02-lost-errorHandling-c` | `TSET-PAY-L3-INT-02` | rule |

Stub: `TSTUB-PAY-INT-02-timeout` INT-02 → timeout

Tiêm lỗi: `TFI-PAY-INT-02-err-ERR-02-lost` Stub INT-02 tạo lỗi unknownOutcome — Tại INT-02, ngay sau khi giao dịch đã commit, trước khi trả kết quả

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: unknownOutcome tại INT-02 (afterCommit) |  |  |
| 3 | when | API thanh toán trả client hết thời gian chờ |  | ERR-02-lost |
| 4 | then | Quan sát kết quả: tra cứu trạng thái |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | tra cứu trạng thái |  | `TORC-PAY-ERR-02-lost` |
| 2 | count | == | Hiệu ứng phụ sau commit đúng một lần: tra cứu trạng thái |  | `TORC-PAY-ERR-02-lost` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-ERR-02-lost-errorHandling-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: API thanh toán trả client hết thời gian chờ",
 "level": "L3",
 "priority": "P1",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [
  {
   "code": "TSTUB-PAY-INT-02-timeout",
   "target": "INT-02",
   "behavior": "timeout"
  }
 ],
 "faultInjections": [
  {
   "code": "TFI-PAY-INT-02-err-ERR-02-lost",
   "mechanism": "Stub INT-02 tạo lỗi unknownOutcome",
   "trigger": "Tại INT-02, ngay sau khi giao dịch đã commit, trước khi trả kết quả"
  }
 ],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-ERR-02-lost-errorHandling-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-ERR-02-lost-errorHandling-c.S2",
   "kind": "given",
   "action": "Bật tiêm lỗi: unknownOutcome tại INT-02 (afterCommit)"
  },
  {
   "id": "TC-L3-PAY-ERR-02-lost-errorHandling-c.S3",
   "kind": "when",
   "action": "API thanh toán trả client hết thời gian chờ",
   "target": "ERR-02-lost"
  },
  {
   "id": "TC-L3-PAY-ERR-02-lost-errorHandling-c.S4",
   "kind": "then",
   "action": "Quan sát kết quả: tra cứu trạng thái"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-ERR-02-lost-errorHandling-c.E1",
   "assertion": "TAS-L3-PAY-ERR-02-lost-errorHandling-c-1",
   "kind": "errorCode",
   "op": "==",
   "value": "tra cứu trạng thái",
   "description": "Kết quả khớp oracle TORC-PAY-ERR-02-lost",
   "oracle": {
    "code": "TORC-PAY-ERR-02-lost",
    "kind": "errorContract",
    "expected": "tra cứu trạng thái"
   }
  },
  {
   "id": "TC-L3-PAY-ERR-02-lost-errorHandling-c.E2",
   "assertion": "TAS-L3-PAY-ERR-02-lost-errorHandling-c-2",
   "kind": "count",
   "op": "==",
   "value": "Hiệu ứng phụ sau commit đúng một lần: tra cứu trạng thái",
   "description": "Không trùng, không mất hiệu ứng phụ sau commit",
   "oracle": {
    "code": "TORC-PAY-ERR-02-lost",
    "kind": "errorContract",
    "expected": "tra cứu trạng thái"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-ERR-02-lost-errorHandling-c"
  ],
  "conditions": [
   "TCND-PAY-ERR-02-lost-errorHandling"
  ],
  "requirements": [
   "TR-PAY-L3-INT-02-reliability"
  ],
  "basis": [
   "ERR-02-lost"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-02"
  ]
 }
}
```

#### TC-L3-PAY-INT-01-concurrency-concurrent-dup-c

Chứng minh: Hai yêu cầu cùng khóa idempotency gửi đồng thời tới Cổng thanh toán

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-INT-01-concurrency-concurrent-dup-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Hai yêu cầu cùng khóa idempotency gửi đồng thời tới Cổng thanh toán |  | INT-01 |
| 3 | then | Quan sát kết quả: Cùng khóa idempotency không tạo hiệu ứng lặp |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Cùng khóa idempotency không tạo hiệu ứng lặp |  | `TORC-PAY-INT-01-idem` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-INT-01-concurrency-concurrent-dup-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Hai yêu cầu cùng khóa idempotency gửi đồng thời tới Cổng thanh toán",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-INT-01-concurrency-concurrent-dup-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-INT-01-concurrency-concurrent-dup-c.S2",
   "kind": "when",
   "action": "Hai yêu cầu cùng khóa idempotency gửi đồng thời tới Cổng thanh toán",
   "target": "INT-01"
  },
  {
   "id": "TC-L3-PAY-INT-01-concurrency-concurrent-dup-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Cùng khóa idempotency không tạo hiệu ứng lặp"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-INT-01-concurrency-concurrent-dup-c.E1",
   "assertion": "TAS-L3-PAY-INT-01-concurrency-concurrent-dup-c-1",
   "kind": "count",
   "op": "==",
   "value": "Cùng khóa idempotency không tạo hiệu ứng lặp",
   "description": "Kết quả khớp oracle TORC-PAY-INT-01-idem",
   "oracle": {
    "code": "TORC-PAY-INT-01-idem",
    "kind": "invariant",
    "expected": "Cùng khóa idempotency không tạo hiệu ứng lặp"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-INT-01-concurrency-concurrent-dup-c"
  ],
  "conditions": [
   "TCND-PAY-INT-01-concurrency-concurrent-dup"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-reliability"
  ],
  "basis": [
   "INT-01"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-INT-01-idempotency-payload-mismatch-c

Chứng minh: Cùng khóa idempotency, khác payload tới Cổng thanh toán

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-INT-01-idempotency-payload-mismatch-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Cùng khóa idempotency, khác payload tới Cổng thanh toán |  | INT-01 |
| 3 | then | Quan sát kết quả: Cùng khóa idempotency không tạo hiệu ứng lặp |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Cùng khóa idempotency không tạo hiệu ứng lặp |  | `TORC-PAY-INT-01-idem` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-INT-01-idempotency-payload-mismatch-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Cùng khóa idempotency, khác payload tới Cổng thanh toán",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-INT-01-idempotency-payload-mismatch-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-INT-01-idempotency-payload-mismatch-c.S2",
   "kind": "when",
   "action": "Cùng khóa idempotency, khác payload tới Cổng thanh toán",
   "target": "INT-01"
  },
  {
   "id": "TC-L3-PAY-INT-01-idempotency-payload-mismatch-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Cùng khóa idempotency không tạo hiệu ứng lặp"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-INT-01-idempotency-payload-mismatch-c.E1",
   "assertion": "TAS-L3-PAY-INT-01-idempotency-payload-mismatch-c-1",
   "kind": "count",
   "op": "==",
   "value": "Cùng khóa idempotency không tạo hiệu ứng lặp",
   "description": "Kết quả khớp oracle TORC-PAY-INT-01-idem",
   "oracle": {
    "code": "TORC-PAY-INT-01-idem",
    "kind": "invariant",
    "expected": "Cùng khóa idempotency không tạo hiệu ứng lặp"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-INT-01-idempotency-payload-mismatch-c"
  ],
  "conditions": [
   "TCND-PAY-INT-01-idempotency-payload-mismatch"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-reliability"
  ],
  "basis": [
   "INT-01"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-INT-01-idempotency-replay-c

Chứng minh: Gửi lại cùng yêu cầu (cùng khóa idempotency) tới Cổng thanh toán

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-INT-01-idempotency-replay-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Gửi lại cùng yêu cầu (cùng khóa idempotency) tới Cổng thanh toán |  | INT-01 |
| 3 | then | Quan sát kết quả: Cùng khóa idempotency không tạo hiệu ứng lặp |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Cùng khóa idempotency không tạo hiệu ứng lặp |  | `TORC-PAY-INT-01-idem` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-INT-01-idempotency-replay-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Gửi lại cùng yêu cầu (cùng khóa idempotency) tới Cổng thanh toán",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-INT-01-idempotency-replay-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-INT-01-idempotency-replay-c.S2",
   "kind": "when",
   "action": "Gửi lại cùng yêu cầu (cùng khóa idempotency) tới Cổng thanh toán",
   "target": "INT-01"
  },
  {
   "id": "TC-L3-PAY-INT-01-idempotency-replay-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Cùng khóa idempotency không tạo hiệu ứng lặp"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-INT-01-idempotency-replay-c.E1",
   "assertion": "TAS-L3-PAY-INT-01-idempotency-replay-c-1",
   "kind": "count",
   "op": "==",
   "value": "Cùng khóa idempotency không tạo hiệu ứng lặp",
   "description": "Kết quả khớp oracle TORC-PAY-INT-01-idem",
   "oracle": {
    "code": "TORC-PAY-INT-01-idem",
    "kind": "invariant",
    "expected": "Cùng khóa idempotency không tạo hiệu ứng lặp"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-INT-01-idempotency-replay-c"
  ],
  "conditions": [
   "TCND-PAY-INT-01-idempotency-replay"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-reliability"
  ],
  "basis": [
   "INT-01"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-INT-01-timing-worst-INT-01-c

Chứng minh: Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-INT-01-timing-worst-INT-01-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms |  | INT-01 |
| 3 | then | Quan sát kết quả: 75000 ms ≤ 30000 ms (ngân sách của SCN-01) |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | ms | `TORC-PAY-INT-01-worst-INT-01` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-INT-01-timing-worst-INT-01-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "performance"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-INT-01-timing-worst-INT-01-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-INT-01-timing-worst-INT-01-c.S2",
   "kind": "when",
   "action": "Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms",
   "target": "INT-01"
  },
  {
   "id": "TC-L3-PAY-INT-01-timing-worst-INT-01-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: 75000 ms ≤ 30000 ms (ngân sách của SCN-01)"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-INT-01-timing-worst-INT-01-c.E1",
   "assertion": "TAS-L3-PAY-INT-01-timing-worst-INT-01-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "75000 ms ≤ 30000 ms (ngân sách của SCN-01)",
   "unit": "ms",
   "description": "Kết quả khớp oracle TORC-PAY-INT-01-worst-INT-01",
   "oracle": {
    "code": "TORC-PAY-INT-01-worst-INT-01",
    "kind": "threshold",
    "expected": "75000 ms ≤ 30000 ms (ngân sách của SCN-01)"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-INT-01-timing-worst-INT-01-c"
  ],
  "conditions": [
   "TCND-PAY-INT-01-timing-worst-INT-01"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-performance"
  ],
  "basis": [
   "INT-01"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-LIM-01-performance-requestsPerSecond-at-c

Chứng minh: Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-LIM-01-performance-requestsPerSecond-at-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây |  | LIM-01 |
| 3 | then | Quan sát kết quả: requestsPerSecond ≤ 50 request/giây |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | requestsPerSecond ≤ 50 request/giây | request/giây | `TORC-PAY-LIM-01-requestsPerSecond` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-at-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "performance"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-at-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-at-c.S2",
   "kind": "when",
   "action": "Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây",
   "target": "LIM-01"
  },
  {
   "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-at-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: requestsPerSecond ≤ 50 request/giây"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-at-c.E1",
   "assertion": "TAS-L3-PAY-LIM-01-performance-requestsPerSecond-at-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "requestsPerSecond ≤ 50 request/giây",
   "unit": "request/giây",
   "description": "Kết quả khớp oracle TORC-PAY-LIM-01-requestsPerSecond",
   "oracle": {
    "code": "TORC-PAY-LIM-01-requestsPerSecond",
    "kind": "threshold",
    "expected": "requestsPerSecond ≤ 50 request/giây"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-LIM-01-performance-requestsPerSecond-at-c"
  ],
  "conditions": [
   "TCND-PAY-LIM-01-performance-requestsPerSecond-at"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-performance"
  ],
  "basis": [
   "LIM-01"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-LIM-01-performance-requestsPerSecond-over-c

Chứng minh: Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-LIM-01-performance-requestsPerSecond-over-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây |  | LIM-01 |
| 3 | then | Quan sát kết quả: requestsPerSecond ≤ 50 request/giây |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | requestsPerSecond ≤ 50 request/giây | request/giây | `TORC-PAY-LIM-01-requestsPerSecond` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-over-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây",
 "level": "L3",
 "priority": "P0",
 "testTypes": [
  "performance"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-over-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-over-c.S2",
   "kind": "when",
   "action": "Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây",
   "target": "LIM-01"
  },
  {
   "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-over-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: requestsPerSecond ≤ 50 request/giây"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-LIM-01-performance-requestsPerSecond-over-c.E1",
   "assertion": "TAS-L3-PAY-LIM-01-performance-requestsPerSecond-over-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "requestsPerSecond ≤ 50 request/giây",
   "unit": "request/giây",
   "description": "Kết quả khớp oracle TORC-PAY-LIM-01-requestsPerSecond",
   "oracle": {
    "code": "TORC-PAY-LIM-01-requestsPerSecond",
    "kind": "threshold",
    "expected": "requestsPerSecond ≤ 50 request/giây"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-LIM-01-performance-requestsPerSecond-over-c"
  ],
  "conditions": [
   "TCND-PAY-LIM-01-performance-requestsPerSecond-over"
  ],
  "requirements": [
   "TR-PAY-L3-INT-01-performance"
  ],
  "basis": [
   "LIM-01"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-01"
  ]
 }
}
```

#### TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-at-c

Chứng minh: API thanh toán ở đúng giới hạn maxPayloadKB=64 KB

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-LIM-02-inputValidation-maxPayloadKB-at-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | API thanh toán ở đúng giới hạn maxPayloadKB=64 KB |  | LIM-02 |
| 3 | then | Quan sát kết quả: maxPayloadKB ≤ 64 KB |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | maxPayloadKB ≤ 64 KB | KB | `TORC-PAY-LIM-02-maxPayloadKB` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-at-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: API thanh toán ở đúng giới hạn maxPayloadKB=64 KB",
 "level": "L3",
 "priority": "P1",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-at-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-at-c.S2",
   "kind": "when",
   "action": "API thanh toán ở đúng giới hạn maxPayloadKB=64 KB",
   "target": "LIM-02"
  },
  {
   "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-at-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: maxPayloadKB ≤ 64 KB"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-at-c.E1",
   "assertion": "TAS-L3-PAY-LIM-02-inputValidation-maxPayloadKB-at-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "maxPayloadKB ≤ 64 KB",
   "unit": "KB",
   "description": "Kết quả khớp oracle TORC-PAY-LIM-02-maxPayloadKB",
   "oracle": {
    "code": "TORC-PAY-LIM-02-maxPayloadKB",
    "kind": "threshold",
    "expected": "maxPayloadKB ≤ 64 KB"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-LIM-02-inputValidation-maxPayloadKB-at-c"
  ],
  "conditions": [
   "TCND-PAY-LIM-02-inputValidation-maxPayloadKB-at"
  ],
  "requirements": [
   "TR-PAY-L3-INT-02-functional"
  ],
  "basis": [
   "LIM-02"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-02"
  ]
 }
}
```

#### TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-over-c

Chứng minh: API thanh toán vượt giới hạn maxPayloadKB=64 KB

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-LIM-02-inputValidation-maxPayloadKB-over-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | API thanh toán vượt giới hạn maxPayloadKB=64 KB |  | LIM-02 |
| 3 | then | Quan sát kết quả: maxPayloadKB ≤ 64 KB |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | maxPayloadKB ≤ 64 KB | KB | `TORC-PAY-LIM-02-maxPayloadKB` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-over-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: API thanh toán vượt giới hạn maxPayloadKB=64 KB",
 "level": "L3",
 "priority": "P1",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-over-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-over-c.S2",
   "kind": "when",
   "action": "API thanh toán vượt giới hạn maxPayloadKB=64 KB",
   "target": "LIM-02"
  },
  {
   "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-over-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: maxPayloadKB ≤ 64 KB"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-over-c.E1",
   "assertion": "TAS-L3-PAY-LIM-02-inputValidation-maxPayloadKB-over-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "maxPayloadKB ≤ 64 KB",
   "unit": "KB",
   "description": "Kết quả khớp oracle TORC-PAY-LIM-02-maxPayloadKB",
   "oracle": {
    "code": "TORC-PAY-LIM-02-maxPayloadKB",
    "kind": "threshold",
    "expected": "maxPayloadKB ≤ 64 KB"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-LIM-02-inputValidation-maxPayloadKB-over-c"
  ],
  "conditions": [
   "TCND-PAY-LIM-02-inputValidation-maxPayloadKB-over"
  ],
  "requirements": [
   "TR-PAY-L3-INT-02-functional"
  ],
  "basis": [
   "LIM-02"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-02"
  ]
 }
}
```

#### TC-L3-PAY-LIM-02-performance-requestsPerSecond-at-c

Chứng minh: API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-LIM-02-performance-requestsPerSecond-at-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây |  | LIM-02 |
| 3 | then | Quan sát kết quả: requestsPerSecond ≤ 20 request/giây |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | requestsPerSecond ≤ 20 request/giây | request/giây | `TORC-PAY-LIM-02-requestsPerSecond` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-at-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây",
 "level": "L3",
 "priority": "P1",
 "testTypes": [
  "performance"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-at-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-at-c.S2",
   "kind": "when",
   "action": "API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây",
   "target": "LIM-02"
  },
  {
   "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-at-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: requestsPerSecond ≤ 20 request/giây"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-at-c.E1",
   "assertion": "TAS-L3-PAY-LIM-02-performance-requestsPerSecond-at-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "requestsPerSecond ≤ 20 request/giây",
   "unit": "request/giây",
   "description": "Kết quả khớp oracle TORC-PAY-LIM-02-requestsPerSecond",
   "oracle": {
    "code": "TORC-PAY-LIM-02-requestsPerSecond",
    "kind": "threshold",
    "expected": "requestsPerSecond ≤ 20 request/giây"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-LIM-02-performance-requestsPerSecond-at-c"
  ],
  "conditions": [
   "TCND-PAY-LIM-02-performance-requestsPerSecond-at"
  ],
  "requirements": [
   "TR-PAY-L3-INT-02-performance"
  ],
  "basis": [
   "LIM-02"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-02"
  ]
 }
}
```

#### TC-L3-PAY-LIM-02-performance-requestsPerSecond-over-c

Chứng minh: API thanh toán vượt giới hạn requestsPerSecond=20 request/giây

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-LIM-02-performance-requestsPerSecond-over-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | API thanh toán vượt giới hạn requestsPerSecond=20 request/giây |  | LIM-02 |
| 3 | then | Quan sát kết quả: requestsPerSecond ≤ 20 request/giây |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | requestsPerSecond ≤ 20 request/giây | request/giây | `TORC-PAY-LIM-02-requestsPerSecond` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-over-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: API thanh toán vượt giới hạn requestsPerSecond=20 request/giây",
 "level": "L3",
 "priority": "P1",
 "testTypes": [
  "performance"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-over-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-over-c.S2",
   "kind": "when",
   "action": "API thanh toán vượt giới hạn requestsPerSecond=20 request/giây",
   "target": "LIM-02"
  },
  {
   "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-over-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: requestsPerSecond ≤ 20 request/giây"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-LIM-02-performance-requestsPerSecond-over-c.E1",
   "assertion": "TAS-L3-PAY-LIM-02-performance-requestsPerSecond-over-c-1",
   "kind": "threshold",
   "op": "<=",
   "value": "requestsPerSecond ≤ 20 request/giây",
   "unit": "request/giây",
   "description": "Kết quả khớp oracle TORC-PAY-LIM-02-requestsPerSecond",
   "oracle": {
    "code": "TORC-PAY-LIM-02-requestsPerSecond",
    "kind": "threshold",
    "expected": "requestsPerSecond ≤ 20 request/giây"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-LIM-02-performance-requestsPerSecond-over-c"
  ],
  "conditions": [
   "TCND-PAY-LIM-02-performance-requestsPerSecond-over"
  ],
  "requirements": [
   "TR-PAY-L3-INT-02-performance"
  ],
  "basis": [
   "LIM-02"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-02"
  ]
 }
}
```

#### TC-L3-PAY-MECH-01-invariant-holds-c

Chứng minh: MECH-01

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-PAY-L3-MECH-01-invariant-holds-c` | `TSET-PAY-L3-MECH-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | MECH-01 |  | MECH-01 |
| 3 | then | Quan sát kết quả: MECH-01 |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | MECH-01 |  | `TORC-PAY-MECH-01-holds` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-MECH-01-invariant-holds-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: MECH-01",
 "level": "L3",
 "priority": "P2",
 "testTypes": [
  "functional"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-MECH-01-invariant-holds-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-MECH-01-invariant-holds-c.S2",
   "kind": "when",
   "action": "MECH-01",
   "target": "MECH-01"
  },
  {
   "id": "TC-L3-PAY-MECH-01-invariant-holds-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: MECH-01"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-MECH-01-invariant-holds-c.E1",
   "assertion": "TAS-L3-PAY-MECH-01-invariant-holds-c-1",
   "kind": "count",
   "op": "==",
   "value": "MECH-01",
   "description": "Kết quả khớp oracle TORC-PAY-MECH-01-holds",
   "oracle": {
    "code": "TORC-PAY-MECH-01-holds",
    "kind": "invariant",
    "expected": "MECH-01"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-MECH-01-invariant-holds-c"
  ],
  "conditions": [
   "TCND-PAY-MECH-01-invariant-holds"
  ],
  "requirements": [
   "TR-PAY-L3-MECH-01-functional"
  ],
  "basis": [
   "MECH-01"
  ],
  "testSet": [
   "TSET-PAY-L3-MECH-01"
  ]
 }
}
```

#### TC-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c

Chứng minh: Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-PAY-L3-MOD-Payment-errorHandling-dep-MOD-Ledger-c` | `TSET-PAY-L3-MOD-Payment` | rule |

Stub: `TSTUB-PAY-INT-01-success` INT-01 → success; `TSTUB-PAY-MOD-Ledger-errorTransient` MOD-Ledger → errorTransient

Tiêm lỗi: `TFI-PAY-MOD-Payment-dep-MOD-Ledger` Stub phụ thuộc trả lỗi hoặc không phản hồi — Tại MOD-Ledger, trong khi thao tác chính đang chạy

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: dependencyFailure tại MOD-Ledger (during) |  |  |
| 3 | when | Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi |  | MOD-Payment |
| 4 | then | Quan sát kết quả: Thanh toán xử lý lỗi của Sổ cái theo thiết kế |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Thanh toán xử lý lỗi của Sổ cái theo thiết kế |  | `TORC-PAY-MOD-Ledger-dep-MOD-Payment` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi",
 "level": "L3",
 "priority": "P2",
 "testTypes": [
  "reliability"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [
  {
   "code": "TSTUB-PAY-INT-01-success",
   "target": "INT-01",
   "behavior": "success"
  },
  {
   "code": "TSTUB-PAY-MOD-Ledger-errorTransient",
   "target": "MOD-Ledger",
   "behavior": "errorTransient"
  }
 ],
 "faultInjections": [
  {
   "code": "TFI-PAY-MOD-Payment-dep-MOD-Ledger",
   "mechanism": "Stub phụ thuộc trả lỗi hoặc không phản hồi",
   "trigger": "Tại MOD-Ledger, trong khi thao tác chính đang chạy"
  }
 ],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c.S2",
   "kind": "given",
   "action": "Bật tiêm lỗi: dependencyFailure tại MOD-Ledger (during)"
  },
  {
   "id": "TC-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c.S3",
   "kind": "when",
   "action": "Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi",
   "target": "MOD-Payment"
  },
  {
   "id": "TC-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c.S4",
   "kind": "then",
   "action": "Quan sát kết quả: Thanh toán xử lý lỗi của Sổ cái theo thiết kế"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c.E1",
   "assertion": "TAS-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c-1",
   "kind": "errorCode",
   "op": "==",
   "value": "Thanh toán xử lý lỗi của Sổ cái theo thiết kế",
   "description": "Kết quả khớp oracle TORC-PAY-MOD-Ledger-dep-MOD-Payment",
   "oracle": {
    "code": "TORC-PAY-MOD-Ledger-dep-MOD-Payment",
    "kind": "errorContract",
    "expected": "Thanh toán xử lý lỗi của Sổ cái theo thiết kế"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-MOD-Payment-errorHandling-dep-MOD-Ledger-c"
  ],
  "conditions": [
   "TCND-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger"
  ],
  "requirements": [
   "TR-PAY-L3-MOD-Payment-reliability"
  ],
  "basis": [
   "MOD-Payment"
  ],
  "testSet": [
   "TSET-PAY-L3-MOD-Payment"
  ]
 }
}
```

#### TC-L3-PAY-SEC-02-access-allow-c

Chứng minh: Gọi API thanh toán với thông tin SEC-02 hợp lệ

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-SEC-02-access-allow-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Gọi API thanh toán với thông tin SEC-02 hợp lệ |  | SEC-02 |
| 3 | then | Quan sát kết quả: Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ |  | `TORC-PAY-SEC-02-access` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-SEC-02-access-allow-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Gọi API thanh toán với thông tin SEC-02 hợp lệ",
 "level": "L3",
 "priority": "P1",
 "testTypes": [
  "security"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-SEC-02-access-allow-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-SEC-02-access-allow-c.S2",
   "kind": "when",
   "action": "Gọi API thanh toán với thông tin SEC-02 hợp lệ",
   "target": "SEC-02"
  },
  {
   "id": "TC-L3-PAY-SEC-02-access-allow-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-SEC-02-access-allow-c.E1",
   "assertion": "TAS-L3-PAY-SEC-02-access-allow-c-1",
   "kind": "value",
   "op": "==",
   "value": "Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ",
   "description": "Kết quả khớp oracle TORC-PAY-SEC-02-access",
   "oracle": {
    "code": "TORC-PAY-SEC-02-access",
    "kind": "reference",
    "expected": "Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-SEC-02-access-allow-c"
  ],
  "conditions": [
   "TCND-PAY-SEC-02-access-allow"
  ],
  "requirements": [
   "TR-PAY-L3-INT-02-security"
  ],
  "basis": [
   "SEC-02"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-02"
  ]
 }
}
```

#### TC-L3-PAY-SEC-02-access-deny-c

Chứng minh: Gọi API thanh toán thiếu hoặc sai SEC-02

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-SEC-02-access-deny-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Gọi API thanh toán thiếu hoặc sai SEC-02 |  | SEC-02 |
| 3 | then | Quan sát kết quả: Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ |  | `TORC-PAY-SEC-02-access` |

TCIR:

```json
{
 "schemaVersion": "tcir/v1",
 "id": "TC-L3-PAY-SEC-02-access-deny-c",
 "feature": "PAY",
 "specVersion": 1,
 "objective": "Chứng minh: Gọi API thanh toán thiếu hoặc sai SEC-02",
 "level": "L3",
 "priority": "P1",
 "testTypes": [
  "security"
 ],
 "automatable": true,
 "review": {
  "derivation": "rule",
  "status": "proposed"
 },
 "preconditions": [],
 "fixtures": [],
 "stubs": [],
 "faultInjections": [],
 "data": {
  "policy": "generate",
  "items": []
 },
 "steps": [
  {
   "id": "TC-L3-PAY-SEC-02-access-deny-c.S1",
   "kind": "given",
   "action": "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp"
  },
  {
   "id": "TC-L3-PAY-SEC-02-access-deny-c.S2",
   "kind": "when",
   "action": "Gọi API thanh toán thiếu hoặc sai SEC-02",
   "target": "SEC-02"
  },
  {
   "id": "TC-L3-PAY-SEC-02-access-deny-c.S3",
   "kind": "then",
   "action": "Quan sát kết quả: Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ"
  }
 ],
 "expected": [
  {
   "id": "TC-L3-PAY-SEC-02-access-deny-c.E1",
   "assertion": "TAS-L3-PAY-SEC-02-access-deny-c-1",
   "kind": "value",
   "op": "==",
   "value": "Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ",
   "description": "Kết quả khớp oracle TORC-PAY-SEC-02-access",
   "oracle": {
    "code": "TORC-PAY-SEC-02-access",
    "kind": "reference",
    "expected": "Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ"
   }
  }
 ],
 "trace": {
  "coverageItems": [
   "TCOV-PAY-L3-SEC-02-access-deny-c"
  ],
  "conditions": [
   "TCND-PAY-SEC-02-access-deny"
  ],
  "requirements": [
   "TR-PAY-L3-INT-02-security"
  ],
  "basis": [
   "SEC-02"
  ],
  "testSet": [
   "TSET-PAY-L3-INT-02"
  ]
 }
}
```

## 3. Lịch sử phiên bản và phạm vi hồi quy

| Phiên bản | Trạng thái | CR | Thêm | Sửa | Bỏ | Số nút |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Draft | — | 441 | 0 | 0 | 441 |

Hồi quy (0 test case đã baseline, thuộc phần bị ảnh hưởng nhưng không đổi): không có
