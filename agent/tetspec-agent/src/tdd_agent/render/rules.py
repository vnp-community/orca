"""Placement and extraction rules (pure functions over knowledge-graph elements)."""
from __future__ import annotations

import re
from typing import Any

# class -> part of the TDD (template key); classes not listed fall to the keyword rules or the appendix
CLASS_PART = {
    "Requirement": "scope", "RequirementVersion": "scope", "Capability": "scope",
    "Component": "decomp", "Module": "decomp",
    "StateMachine": "behavior", "State": "behavior", "Transition": "behavior", "BusinessRule": "behavior", "Algorithm": "behavior", "Flow": "behavior",
    "DecisionTable": "behavior", "Condition": "behavior",
    "Operation": "contracts", "Interface": "contracts", "Event": "contracts", "ContractCompatibility": "contracts", "ExternalSystem": "contracts",
    "Entity": "data", "Attribute": "data", "Index": "data", "DataStore": "data", "RetentionPolicy": "data",
    "ErrorCode": "resilience", "FailurePolicy": "resilience", "RetryPolicy": "resilience", "TimeoutBudget": "resilience", "Saga": "resilience", "Transaction": "resilience",
    "IdempotencyPolicy": "resilience", "ConsistencyModel": "resilience", "ReconciliationPolicy": "resilience",
    "Migration": "migration", "MigrationStage": "migration",
    "QualityTarget": "quality", "LoadProfile": "quality", "CapacityModel": "quality", "HighAvailabilityTopology": "quality", "RecoveryObjective": "quality",
    "Measurement": "quality", "Quantity": "quality", "WorkItem": "quality",
    "PermissionDecision": "security", "Threat": "security", "Control": "security", "Credential": "security", "Role": "security", "AuditTrail": "security",
    "Decision": "decisions",
    "TestCase": "trace", "Scenario": "trace", "TestReceipt": "trace",
}

# first matching rule wins; applied to prose facts (id FACT-*) and constraints without a structural id
KEYWORD_PART: tuple[tuple[str, re.Pattern], ...] = tuple((k, re.compile(p, re.I)) for k, p in (
    ("security", r"bảo mật|xác thực|phân quyền|\btoken\b|mã hóa|mã hoá|bí mật|\bsecret|oidc|audit|kiểm toán|\brbac\b|\btls\b|nhạy cảm|approval|identity"),
    ("contracts", r"endpoint|\bapi\b|webhook|giao diện lập trình"),
    ("migration", r"migration|di trú|rollback|expand|backfill"),
    ("data", r"khoá|khóa|\bbảng\b|schema|jsonb|phân vùng|lưu trữ|chỉ mục|\bindex|on delete|foreign|unique|dữ liệu"),
    ("resilience", r"retry|thử lại|backoff|timeout|idempot|\bsaga\b|bù trừ|suy giảm có kiểm soát"),
    ("quality", r"triển khai|deploy|vận hành|binary|\broot\b|pinned|quan sát|metric|backup|sao lưu|cấu hình|ci/cd|\bci\b|kubernetes|helm|cảnh báo vận hành|quy mô|dung lượng|độ trễ|\bp9[59]\b|\btải\b|throughput|latency|\bsla\b|chịu tải"),
    ("decomp", r"thành phần|component|container|dịch vụ|\bservice\b|module"),
))

# NFR group (last heading of the source section) -> part; unknown headings stay in quality
NFR_HEADING_PART: tuple[tuple[str, re.Pattern], ...] = tuple((k, re.compile(p, re.I)) for k, p in (
    ("security", r"bảo mật|kiểm toán|an toàn|security|audit"),
    ("data", r"toàn vẹn|chất lượng dữ liệu|integrity"),
))
TRACE_HEADING = re.compile(r"truy vết|trace", re.I)
GOAL_TEXT = re.compile(r"tổng quan|mục tiêu|mục đích", re.I)
FAMILY_ID = re.compile(r"^[A-Za-z]+(?:-[A-Za-z]+)*$")  # an id without a number is a group of ids, not one element
HIDDEN_CODE = re.compile(r"^(FACT-|TR-|SM-|GAP-)|:")  # ids made by the knowledge graph, not found in the source documents
SECTION_NUMBER = re.compile(r"^(\d+(?:\.\d+)*)\.?\s+")

CHILD_PARENT = {"Attribute": "Entity", "State": "StateMachine", "Transition": "StateMachine"}
TECHNICAL_PROPS = {"logical_id", "revision", "source_section", "sourced_props", "definition", "evidence_span", "upstream_definition", "source_columns",
                   "tdds", "quantities", "target", "variants", "m", "ent", "brl", "fr", "m_fr"}

_DEF_NOISE = re.compile(r"\\([_\[\]*])")


def clean(value: Any) -> str:
    """Source text with markdown escapes from table extraction removed (``display\\_name`` -> ``display_name``)."""
    if value is None:
        return ""
    if isinstance(value, bool):
        return "có" if value else "không"
    if isinstance(value, (list, tuple)):
        return ", ".join(clean(v) for v in value if clean(v))
    if isinstance(value, dict):
        return "; ".join(f"{k}: {clean(v)}" for k, v in value.items())
    return _DEF_NOISE.sub(r"\1", str(value)).strip()


def natural_key(text: str) -> list:
    return [int(p) if p.isdigit() else p for p in re.split(r"(\d+)", text)]


def props(e: dict) -> dict:
    return e.get("props") or {}


def prop_text(e: dict) -> str:
    p = props(e)
    for k in ("statement", "chosen", "predicate", "n_i_dung", "meaning"):
        if clean(p.get(k)):
            return clean(p[k])
    return clean(e.get("name"))


def last_heading(e: dict) -> str:
    parts = [x.strip() for x in (props(e).get("source_section") or "").split(" > ") if x.strip()]
    return parts[-1] if parts else ""


def section_number(e: dict) -> str:
    """Number of the deepest numbered heading of the source section (``5.1``), or ''."""
    for part in reversed([x.strip() for x in (props(e).get("source_section") or "").split(" > ")]):
        m = SECTION_NUMBER.match(part)
        if m:
            return m.group(1)
    return ""


def keyword_part(text: str) -> str | None:
    for key, pat in KEYWORD_PART:
        if pat.search(text):
            return key
    return None


def is_hidden_code(eid: str) -> bool:
    return bool(HIDDEN_CODE.search(eid))


def id_family(eid: str) -> str:
    """``NFR-PE-04`` -> ``NFR-PE``; ``FR-AST-13`` -> ``FR-AST``; ``ENT-01`` -> ``ENT``; ``FACT-ab12`` -> ``FACT``."""
    parts = eid.split(":")[0].split("-")
    if len(parts) >= 3 and parts[0] in ("NFR", "FR", "API", "UC", "EIF"):
        return "-".join(parts[:2])
    return parts[0]


def is_family_row(e: dict) -> bool:
    """Row of a traceability table that maps a whole id family (``FR-ADM``) to design elements; it is not a requirement itself."""
    return e.get("class") in ("Requirement", "QualityTarget") and bool(FAMILY_ID.match(e["id"])) and bool(TRACE_HEADING.search(last_heading(e)))


def strip_numbering(heading: str) -> str:
    return re.sub(r"^\d+(?:\.\d+)*\.?\s+", "", heading).strip()


def _nfr_part(heading: str) -> str:
    for key, pat in NFR_HEADING_PART:
        if pat.search(heading):
            return key
    return "quality"


def place_elements(elements: list[dict]) -> dict[str, str | None]:
    """Part of the TDD for every element (None = appendix)."""
    nfr_family: dict[str, str] = {}
    for e in elements:
        if e.get("class") == "QualityTarget" and not TRACE_HEADING.search(last_heading(e)) and not is_family_row(e):
            nfr_family.setdefault(id_family(e["id"]), _nfr_part(last_heading(e)))
    out: dict[str, str | None] = {}
    for e in elements:
        cls, eid = e.get("class"), e["id"]
        text = f"{e.get('name') or ''} {prop_text(e)}"
        if is_family_row(e):
            out[eid] = "trace"
        elif cls == "Requirement" and eid.startswith("FACT-"):
            out[eid] = keyword_part(text) or "scope"
        elif cls == "QualityTarget":
            h = last_heading(e)
            part = nfr_family.get(id_family(eid), "quality") if (TRACE_HEADING.search(h) or not h) else _nfr_part(h)
            out[eid] = (keyword_part(text) or "quality") if eid.startswith("FACT-") else part
        elif cls == "Constraint":
            out[eid] = "decomp" if eid.startswith("CT-") else "data" if eid.startswith("INT-") else (keyword_part(text) or "scope")
        elif cls == "BusinessRule" and eid.startswith("FACT-"):
            out[eid] = "behavior"
        else:
            out[eid] = CLASS_PART.get(cls)
    return out


# ---- source pointers: headings of the sources that look related to a missing topic (headings only, never content)
POINTER_TOPICS = {
    "context": r"bối cảnh|actor|hệ thống ngoài|giả định|động lực|stakeholder",
    "api": r"\bapi\b|giao diện|định tuyến|endpoint|openapi|webhook",
    "flows": r"tiến trình|luồng|pipeline|matching|thuật toán|job định kỳ|quy trình",
    "failure": r"sự cố|độ tin cậy|chịu lỗi|khôi phục|retry|timeout",
    "security": r"bảo mật|phân quyền|xác thực|mối đe dọa|token|zero-trust|kiểm toán",
    "deploy": r"triển khai|hạ tầng|môi trường|scale|mạng|phân phối",
    "observability": r"quan sát|giám sát|logging|metrics|observab",
    "data": r"dữ liệu|schema|chỉ mục|phân vùng|di trú|sao lưu|migration",
    "migration": r"di trú|migration|sao lưu|khôi phục",
    "test": r"kiểm thử|\btest",
    "risks": r"rủi ro|vấn đề mở|nợ kỹ thuật|open issue",
    "components": r"sơ đồ|building block|container|thành phần|phụ thuộc",
}
NON_GOAL_LINE = re.compile(r"^\s*(?:[-*]\s*)?(?:\*\*)?\s*ngoài phạm vi|^\s*(?:[-*]\s*)?(?:\*\*)?\s*non-?goals?\b|^\s*(?:[-*]\s*)?(?:\*\*)?\s*out of scope", re.I)
HEADING_LINE = re.compile(r"^(#{1,6})\s+(.*\S)\s*$")


def source_headings(text: str) -> list[tuple[int, int, str]]:
    return [(i, len(m.group(1)), m.group(2)) for i, line in enumerate(text.split("\n"), 1) if (m := HEADING_LINE.match(line))]


def pointers_for(topic: str, texts: dict[tuple[str, str], str], limit: int = 8) -> list[tuple[tuple[str, str], int, str]]:
    pat = re.compile(POINTER_TOPICS[topic], re.I)
    out = []
    for key in sorted(texts):
        for line, level, title in source_headings(texts[key]):
            if 2 <= level <= 3 and pat.search(title):
                out.append((key, line, title))
    return out[:limit]


def non_goal_passages(texts: dict[tuple[str, str], str], bullets: int = 12) -> list[tuple[tuple[str, str], int, list[str]]]:
    """Lines of the sources that open an 'out of scope' passage, with the list lines that follow (verbatim)."""
    out = []
    for key in sorted(texts):
        lines = texts[key].split("\n")
        for i, line in enumerate(lines):
            if NON_GOAL_LINE.match(line):
                body = [line.strip()]
                for nxt in lines[i + 1:i + 1 + bullets]:
                    if re.match(r"^\s*([-*]|\d+[.)])\s+\S", nxt):
                        body.append(nxt.strip())
                    elif nxt.strip():
                        break
                out.append((key, i + 1, body))
    return out


def heading_section_number(texts: dict[tuple[str, str], str], key: tuple[str, str], line: int) -> str:
    """Section number of the heading at ``line`` (``9`` for '## 9. Triển khai'), or ''."""
    lines = texts.get(key, "").split("\n")
    if 0 < line <= len(lines):
        m = HEADING_LINE.match(lines[line - 1])
        if m:
            n = SECTION_NUMBER.match(m.group(2))
            return n.group(1) if n else ""
    return ""
