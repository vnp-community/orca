"""T3.2: each rule of the v1 md template on the new parser."""
import pytest

from tsagent.ingest.md import parse_documents
from tsagent.ingest.md.vocab import classify, coerce

pytestmark = pytest.mark.unit

TDD = """---
doc_id: TDD-X
doc_type: tdd
title: TDD X
version: "1"
status: approved
---

# TDD X

## 3. Module

| Mã | Tên | Trách nhiệm | Phụ thuộc |
| --- | --- | --- | --- |
| MOD-A | A | làm A | MOD-B |
| MOD-B | B | làm B | |

### 6.1 INT-01: Cổng ngoài

| Mã | Lớp | Thuộc tính |
| --- | --- | --- |
| TO-01 | TimeoutPolicy | connectMs=100; readMs=1_000; totalMs=2000 |
| RT-01 | RetryPolicy | maxAttempts=3; idempotentOnly=true |

## 7. Luồng

### 8.1 SCN-01: Luồng chính

`responseBudgetMs`=5000; `invokes`=INT-01

| # | Bước |
| --- | --- |
| 1 | Gọi A |
| 2 | Gọi B |

## 8. Không phải thực thể

| Mã lỗi | Mô tả |
| --- | --- |
| E1 | không có tiền tố đã biết |
"""

SRS = """| Mã | Nội dung | priority | allocatedTo | Tiêu chí chấp nhận |
| --- | --- | --- | --- | --- |
| FR-001 | Tạo đơn | Must | MOD-A | Tạo hợp lệ trả 201; thiếu trường trả 400 |
| NFR-001 | Nhanh | Must | | |
"""

INT = """| Mã | integrationType | direction | interactionStyle | idempotent | Module tích hợp |
| --- | --- | --- | --- | --- | --- |
| INT-01 | ThirdParty | outbound | sync | có (Idempotency-Key) | MOD-A |
"""


def parse(**docs):
    return parse_documents([(k + ".md", v) for k, v in docs.items()], "X")


def test_front_matter_document_and_status():
    pkg, _ = parse(tdd=TDD)
    d = pkg.get("TDD-X")
    assert d.cls == "Document" and d.base["status"] == "Baselined" and d.props["docType"] == "tdd"
    assert pkg.meta["primaryDoc"] == "TDD-X"
    assert [n.code for n in pkg.targets("MOD-A", "describedIn")] == ["TDD-X"]


def test_tables_kv_and_integration_context():
    pkg, diags = parse(tdd=TDD, int=INT)
    assert not [d for d in diags if d.severity == "error"], diags
    assert pkg.get("TO-01").props["readMs"] == 1000 and pkg.get("RT-01").props["idempotentOnly"] is True
    assert {n.code for n in pkg.targets("INT-01", "hasTimeoutPolicy")} == {"TO-01"}
    assert {n.code for n in pkg.targets("INT-01", "hasRetryPolicy")} == {"RT-01"}
    assert [n.code for n in pkg.targets("MOD-A", "dependsOn")] == ["MOD-B"]
    assert [n.code for n in pkg.targets("MOD-A", "integratesVia")] == ["INT-01"]  # reverse column "Module tích hợp"
    assert pkg.get("INT-01").props["idempotent"] is True


def test_heading_entity_kv_line_and_steps():
    pkg, _ = parse(tdd=TDD, int=INT)
    s = pkg.get("SCN-01")
    assert s.props["responseBudgetMs"] == 5000 and s.props["steps"] == ["Gọi A", "Gọi B"] and s.props["name"] == "Luồng chính"
    assert [n.code for n in pkg.targets("SCN-01", "invokes")] == ["INT-01"]
    assert s.base["sourceSection"] == "§8.1"


def test_rows_without_known_prefix_are_not_entities():
    pkg, _ = parse(tdd=TDD)
    assert "E1" not in pkg.nodes


def test_inline_acceptance_criteria_split():
    pkg, _ = parse(tdd=TDD, srs=SRS)
    acs = [n.code for n in pkg.targets("FR-001", "hasAcceptance")]
    assert acs == ["AC-FR-001-1", "AC-FR-001-2"]
    assert pkg.get("AC-FR-001-2").props["statement"] == "thiếu trường trả 400" and pkg.get("AC-FR-001-1").props["derivation"] == "stated"


def test_empty_document_is_error():
    _, diags = parse(x="# Chỉ có tiêu đề\n\nVăn bản tự do.\n")
    assert [d.rule for d in diags if d.severity == "error"] == ["MD-EMPTY"]


@pytest.mark.parametrize("header,expected", [
    ("Mã", ("code", None)), ("Phụ thuộc", ("rel", "dependsOn")), ("`allocatedTo`", ("rel", "allocatedTo")),
    ("Của endpoint", ("rev", "hasField")), ("Thuộc tính", ("kv", None)), ("Lớp", ("skip", None)),
    ("Method", ("attr", "method")), ("Tiêu chí chấp nhận", ("ac", None)), ("Trách nhiệm", ("attr", "responsibility")),
])
def test_column_classification(header, expected):
    assert classify(header) == expected


@pytest.mark.parametrize("key,text,expected", [
    ("required", "có", True), ("required", "không", False), ("max", "50000000", 50000000), ("max", "abc", "abc"),
    ("enum", "ONCE, DAILY", ["ONCE", "DAILY"]), ("note", "—", None), ("targetValue", "0.5", 0.5),
])
def test_coercion(key, text, expected):
    assert coerce(key, text) == expected
