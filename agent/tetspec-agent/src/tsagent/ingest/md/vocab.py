"""Vocabulary of the v1 md template: code prefix -> class, column name -> attribute or relation, value coercion.

Vietnamese literals here match the column names of Vietnamese source documents."""
import re
import unicodedata

from ..tdd_ontology import ALL_RELATIONS

PREFIX_CLASS = {
    "FR": "FunctionalRequirement", "NFR": "NonFunctionalRequirement", "UR": "UserRequirement", "AC": "AcceptanceCriterion",
    "MOD": "Module", "INT": "Integration", "SCN": "Scenario", "SM": "StateMachine", "TRN": "Transition", "ST": "State",
    "EP": "Endpoint", "FLD": "Field", "RSP": "Response", "ERR": "ErrorMapping", "TO": "TimeoutPolicy", "RT": "RetryPolicy",
    "CB": "CircuitBreaker", "POOL": "ConnectionPool", "SEC": "SecurityScheme", "LIM": "PartnerLimit", "PARTY": "ExternalParty",
    "CH": "Channel", "MSG": "Message", "SCH": "Schema", "HDR": "Header", "FB": "FallbackStrategy", "MON": "MonitoringSpec",
    "RCJ": "ReconciliationJob", "DE": "DataEntity", "MECH": "Mechanism", "ADR": "Decision", "ALT": "Alternative",
    "EV": "Evidence", "TECH": "Technology", "RISK": "Risk", "ASM": "Assumption", "OQ": "OpenQuestion", "CON": "Constraint",
    "ENV": "Environment", "DI": "DesignItem", "TST": "TestCase", "CR": "ChangeRequest", "ACT": "Actor",
}
CODE_RE = re.compile(r"^([A-Z]{1,6})-([A-Za-z0-9][A-Za-z0-9._\-]*)$")
CODE_ANY = re.compile(r"(?<![A-Za-z0-9_\-])([A-Z]{1,6}-[A-Za-z0-9][A-Za-z0-9._\-]*[A-Za-z0-9])(?![A-Za-z0-9_])|(?<![A-Za-z0-9_\-])([A-Z]{1,6}-[A-Za-z0-9])(?![A-Za-z0-9_\-])")
HEADING_RE = re.compile(r"^(?:(?P<num>\d+(?:\.\d+)*)\.?\s+)?(?P<code>[A-Z]{1,6}-[A-Za-z0-9][A-Za-z0-9._\-]*)\s*[:：]\s*(?P<title>.*)$")

REL_CANON = {r.lower(): r for r in ALL_RELATIONS}
REL_VN = {"phụ thuộc": "dependsOn", "công nghệ": "uses", "dữ liệu sở hữu": "ownsData", "thực thể dữ liệu": "ownsData",
          "máy trạng thái": "inMachine", "truy vết tới": "tracesTo", "ảnh hưởng": "affects", "bỏ": "removes"}
# reverse columns: the cell holds the code of the source node; the relation goes from the node in the cell to the row's node (ONTOLOGY-TDD section 1.5 v1)
REVERSE = {"của endpoint": "hasField", "phản hồi của": "hasResponse", "gắn vào": "hasRisk", "yêu cầu": "hasAcceptance",
           "module tích hợp": "integratesVia", "đáp ứng": "satisfiedBy", "chặn": "blockedBy", "chi tiết hóa": "refinedBy"}
ATTR_VN = {"nội dung": "text", "mô tả": "description", "tên": "name", "trách nhiệm": "responsibility", "phương án": "name",
           "tóm tắt": "summary", "câu hỏi": "statement", "giả định": "statement", "quyết định": "statement", "ghi chú": "note",
           "ghi": "note", "chủ sở hữu": "owner", "điều kiện": "condition", "từ": "from", "đến": "to", "tiêu chí": "statement",
           "kiểu": "type", "ví dụ": "example", "xác thực": "auth", "căn cứ": "basis", "loại": "kind", "vị trí": "location"}
SKIP_COLS = {"lớp", "class", "#"}
KV_COLS = {"thuộc tính", "attrs", "props"}
INLINE_AC = {"tiêu chí chấp nhận", "acceptance criteria"}
# class -> relation linked from the Integration when the entity sits under an INT-xx heading
CONTRACT_REL = {"ExternalParty": "withParty", "Endpoint": "hasEndpoint", "Channel": "usesChannel", "ErrorMapping": "hasErrorMapping",
                "TimeoutPolicy": "hasTimeoutPolicy", "RetryPolicy": "hasRetryPolicy", "CircuitBreaker": "protectedBy",
                "ConnectionPool": "usesPool", "SecurityScheme": "securedBy", "PartnerLimit": "boundedBy",
                "FallbackStrategy": "hasFallback", "ReconciliationJob": "reconciledBy", "MonitoringSpec": "monitoredBy"}

NUM_ATTRS = {"targetValue", "connectMs", "readMs", "totalMs", "poolAcquireMs", "idleMs", "maxAttempts", "failureRatePercent",
             "minimumCalls", "openDurationSec", "maxTotal", "maxPerRoute", "minIdle", "maxLifetimeSec", "idleTimeoutSec",
             "requestsPerSecond", "maxPayloadKB", "maxConcurrentConnections", "slaAvailabilityPercent", "p99LatencyMs",
             "responseBudgetMs", "partitionCount", "retentionDays", "maxMessageKB", "rotationDays", "min", "max", "minLength",
             "maxLength", "statusCode", "expectedRps"}
BOOL_ATTRS = {"retryable", "idempotent", "jitter", "idempotentOnly", "required", "validationEnabled", "sandboxAvailable",
              "contractConfirmed", "sensitive", "consumerIdempotent", "signatureCheck", "terminal", "primary"}
LIST_ATTRS = {"enum", "steps", "metrics"}
_NUM = re.compile(r"^-?\d+(?:\.\d+)?$")
TRUE = ("true", "có", "yes", "y", "1")
FALSE = ("false", "không", "no", "n", "0")


def fold(s):
    s = unicodedata.normalize("NFD", s)
    s = "".join(c for c in s if unicodedata.category(c) != "Mn").replace("đ", "d").replace("Đ", "D")
    return re.sub(r"[^A-Za-z0-9]+", "_", s).strip("_")


def clean(cell):
    s = re.sub(r"<br\s*/?>", " ", cell or "")
    s = s.replace("**", "").replace("`", "").replace("\\|", "|")
    return re.sub(r"\s+", " ", s).strip()


def classify(header):
    """Return (kind, value): kind in code | rel | rev | attr | kv | ac | skip."""
    raw = header or ""
    tok = re.search(r"`([A-Za-z_]\w*)`", raw)
    plain = clean(re.sub(r"\([^)]*\)", "", raw)).strip()
    key = tok.group(1) if tok else plain
    low = key.lower()
    if low in ("mã", "code", "id"):
        return "code", None
    if low in SKIP_COLS:
        return "skip", None
    if low in KV_COLS:
        return "kv", None
    if low in INLINE_AC:
        return "ac", None
    if low in REL_CANON:
        return "rel", REL_CANON[low]
    if low in REL_VN:
        return "rel", REL_VN[low]
    if low in REVERSE:
        return "rev", REVERSE[low]
    if low in ATTR_VN:
        return "attr", ATTR_VN[low]
    if tok:
        return "attr", key
    if re.fullmatch(r"[A-Za-z][A-Za-z0-9_]*", key):
        # unquoted column titles (`Method`, `Path`) are normalised to the ontology's lowerCamel attribute names
        return "attr", key[0].lower() + key[1:]
    return "attr", fold(key) or "col"


def coerce(key, text):
    t = clean(text)
    if t in ("", "—", "-", "–", "(không)", "n/a", "N/A"):
        return None
    low = t.lower()
    if key in BOOL_ATTRS:
        first = re.split(r"[\s(,;]", low, maxsplit=1)[0]
        if first in TRUE:
            return True
        if first in FALSE:
            return False
    if key in NUM_ATTRS and _NUM.match(t.replace("_", "")):
        t2 = t.replace("_", "")
        return float(t2) if "." in t2 else int(t2)
    if key in LIST_ATTRS:
        sep = "->" if key == "steps" and "->" in t else ","
        return [x.strip() for x in t.split(sep) if x.strip()]
    return t


def codes_in(text, known=()):
    out = []
    for m in CODE_ANY.finditer(clean(text)):
        c = m.group(1) or m.group(2)
        if (c.split("-", 1)[0] in PREFIX_CLASS or c in known) and c not in out:
            out.append(c)
    return out


def class_of(code):
    m = CODE_RE.match(code or "")
    return PREFIX_CLASS.get(m.group(1)) if m else None


def parse_kv(text):
    out = []
    for part in re.split(r";\s*", clean(text).rstrip(".")):
        m = re.match(r"^\s*([A-Za-z_]\w*)\s*=\s*(.+?)\s*$", part, re.S)
        if m:
            out.append((m.group(1), m.group(2)))
    return out
