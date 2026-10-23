"""Deterministic spec-to-script check for pytest files (no LLM): one assertion per expected result, tagged `# @exp <TC>.E<n>`.

The checker is what keeps a script anchored to the spec: it blocks missing, extra, re-valued and vacuous assertions, and the diff guard
blocks a fix round from rewriting an assertion that already matched the spec.
"""
from __future__ import annotations

import ast
import io
import json
import re
import tokenize
from dataclasses import asdict, dataclass, field

EXP_RE = re.compile(r"@exp\s+(\S+)")
# `# @gap <TC>.E<n> <reason>` stands for an expected result the spec does not define well enough to assert (an open TestGap).
GAP_RE = re.compile(r"@gap\s+(\S+)\s*(.*)")
# Words with which a spec value admits it is incomplete; such a value may be marked @gap even without a TestGap on the case.
MISSING_RE = re.compile(r"(?i)thiếu|cần bổ sung|chưa xác định|chưa rõ|chưa có|không nêu|missing|unknown|not specified|\bTBD\b")
TOKEN_RE = re.compile(r"[\w./{}:-]+")
NEUTRAL = (0, 1, "")
NUMBER_RE = re.compile(r"[-+]?\d+(?:[.,]\d+)?")
ATOMIC_RE = re.compile(r"^[\w.:/@+-]+$")
OPS = {ast.Eq: "==", ast.NotEq: "!=", ast.Lt: "<", ast.LtE: "<=", ast.Gt: ">", ast.GtE: ">=", ast.In: "in", ast.NotIn: "not in",
       ast.Is: "is", ast.IsNot: "is not"}
SKIP_MARKS = ("skip", "skipif", "xfail")

ERROR, WARNING, INFO = "error", "warning", "info"


@dataclass
class Finding:
    rule: str
    severity: str
    message: str
    file: str = ""
    line: int | None = None
    case: str | None = None
    exp: str | None = None


@dataclass
class Assertion:
    file: str
    line: int
    function: str
    cases: list[str]
    exp: list[str]
    op: str | None
    literal: object
    has_literal: bool
    source: str
    in_try: bool
    constant: bool
    # Every constant written inside the assertion (compared values, dict keys, subscripts): each must come from the spec.
    literals: list = field(default_factory=list)


@dataclass
class Gap:
    file: str
    line: int
    function: str
    exp: str
    reason: str


@dataclass
class Scan:
    file: str
    functions: dict[str, list[str]] = field(default_factory=dict)      # test function -> case ids from @pytest.mark.tc
    assertions: list[Assertion] = field(default_factory=list)
    gaps: list[Gap] = field(default_factory=list)
    findings: list[Finding] = field(default_factory=list)


def _comments(source: str) -> dict[int, str]:
    out = {}
    try:
        for tok in tokenize.generate_tokens(io.StringIO(source).readline):
            if tok.type == tokenize.COMMENT:
                out[tok.start[0]] = tok.string
    except (tokenize.TokenError, IndentationError):
        pass
    return out


def _mark(dec: ast.expr) -> tuple[str, ast.Call | None] | None:
    call = dec if isinstance(dec, ast.Call) else None
    name = ast.unparse(dec.func if call else dec)
    for prefix in ("pytest.mark.", "mark."):
        if name.startswith(prefix):
            return name[len(prefix):], call
    return None


def _literal(node: ast.expr) -> tuple[bool, object]:
    try:
        return True, ast.literal_eval(node)
    except (ValueError, TypeError, SyntaxError, MemoryError, RecursionError):
        return False, None


def _op_and_value(test: ast.expr) -> tuple[str | None, bool, object]:
    if isinstance(test, ast.Compare) and len(test.ops) == 1:
        op = OPS.get(type(test.ops[0]))
        ok, val = _literal(test.comparators[0])
        if not ok:
            ok, val = _literal(test.left)
        return op, ok, val
    if isinstance(test, ast.UnaryOp) and isinstance(test.op, ast.Not):
        return "falsy", False, None
    return "truthy", False, None


class _Visitor(ast.NodeVisitor):
    def __init__(self, scan: Scan, comments: dict[int, str]):
        self.scan, self.comments = scan, comments
        self.func: str | None = None
        self.try_depth = 0
        self.starts: list[tuple[int, str]] = []

    def visit_FunctionDef(self, node):   # noqa: N802
        if not node.name.startswith("test"):
            return self.generic_visit(node)
        cases = []
        for d in node.decorator_list:
            m = _mark(d)
            if m is None:
                continue
            name, call = m
            if name == "tc" and call and call.args and isinstance(call.args[0], ast.Constant) and isinstance(call.args[0].value, str):
                cases.append(call.args[0].value)
            if name in SKIP_MARKS:
                self.scan.findings.append(Finding("skip-marker", ERROR, f"@pytest.mark.{name} on a generated test", self.scan.file,
                                                  node.lineno))
        self.scan.functions[node.name] = cases
        self.starts.append((node.lineno, node.name))
        prev, self.func = self.func, node.name
        self.generic_visit(node)
        self.func = prev

    visit_AsyncFunctionDef = visit_FunctionDef

    def visit_Try(self, node):   # noqa: N802
        guarded = bool(node.handlers)
        self.try_depth += guarded
        for n in node.body:
            self.visit(n)
        self.try_depth -= guarded
        for n in node.handlers + node.orelse + node.finalbody:
            self.visit(n)

    visit_TryStar = visit_Try

    def visit_Call(self, node):   # noqa: N802
        if ast.unparse(node.func) in ("pytest.skip", "pytest.xfail", "skip"):
            self.scan.findings.append(Finding("skip-call", ERROR, f"{ast.unparse(node.func)}() in a generated test", self.scan.file,
                                              node.lineno))
        self.generic_visit(node)

    def visit_Assert(self, node):   # noqa: N802
        lines = range(node.lineno, (node.end_lineno or node.lineno) + 1)
        exp = [m for ln in lines for m in EXP_RE.findall(self.comments.get(ln, ""))]
        op, has, val = _op_and_value(node.test)
        literals = [n.value for n in ast.walk(node.test) if isinstance(n, ast.Constant)
                    and isinstance(n.value, (str, int, float)) and not isinstance(n.value, bool)]
        self.scan.assertions.append(Assertion(
            self.scan.file, node.lineno, self.func or "", list(self.scan.functions.get(self.func or "", [])), exp, op, val, has,
            ast.unparse(node.test), self.try_depth > 0, isinstance(node.test, ast.Constant), literals))


def scan(source: str, file: str) -> Scan:
    s = Scan(file)
    try:
        tree = ast.parse(source)
    except SyntaxError as exc:
        s.findings.append(Finding("syntax", ERROR, f"syntax error: {exc.msg}", file, exc.lineno))
        return s
    comments = _comments(source)
    v = _Visitor(s, comments)
    v.visit(tree)
    # A comment after the last statement lies outside the function's AST span: a @gap belongs to the nearest test above it.
    for ln, text in sorted(comments.items()):
        m = GAP_RE.search(text)
        owner = max((st for st in v.starts if st[0] <= ln), default=None)
        if m and owner:
            s.gaps.append(Gap(file, ln, owner[1], m.group(1), m.group(2).strip()))
    return s


def _num(text) -> float | None:
    if isinstance(text, bool):
        return None
    if isinstance(text, (int, float)):
        return float(text)
    m = NUMBER_RE.search(str(text))
    return float(m.group().replace(",", ".")) if m else None


def _same(expected: str, literal: object) -> bool:
    if str(literal) == expected:
        return True
    a, b = _num(expected), _num(literal)
    return a is not None and b is not None and a == b and NUMBER_RE.fullmatch(expected.strip()) is not None


class _Corpus:
    """What the spec says for one test case: its objective, steps, expected results, data, gaps, and of its contract the endpoints,
    entity fields and the statuses of related requirements. An assertion may only use constants that occur here."""

    def __init__(self, case: dict):
        contract = case.get("contract") or {}
        parts = [case.get("objective"), case.get("steps"), case.get("expected"), case.get("data"), case.get("gaps"), case.get("preconditions"),
                 [(e.get("method"), e.get("path"), e.get("requestFields"), e.get("responses")) for e in contract.get("endpoints") or []],
                 [e.get("fields") for e in contract.get("entities") or []],
                 [s for s in contract.get("statusCodes") or [] if s.get("related")]]
        self.text = json.dumps(parts, ensure_ascii=False).lower()
        self.tokens = set(TOKEN_RE.findall(self.text))
        self.numbers = {float(n) for n in NUMBER_RE.findall(self.text.replace(",", "."))}

    def has(self, literal) -> bool:
        if literal in NEUTRAL:
            return True
        if isinstance(literal, (int, float)):
            # The same quantity in another unit (2 s written as 2000 ms, 1 min as 60 s) is still the spec's value.
            v = float(literal)
            return any(c in self.numbers for c in (v, v / 1000, v * 1000, v / 60, v * 60))
        lit = str(literal).lower()
        if any(ch.isspace() for ch in lit) or not TOKEN_RE.fullmatch(lit):
            return lit in self.text
        return lit in self.tokens


def _may_be_gap(case: dict, e: dict) -> bool:
    return bool(case.get("gaps") or e.get("gaps")) or bool(MISSING_RE.search(str(e.get("value") or "")))


def check(cases: list[dict], scans: list[Scan]) -> tuple[list[Finding], list[dict]]:
    """Findings and the assertion manifest (one row per expected result) for `cases` over the scanned files."""
    findings = [f for s in scans for f in s.findings]
    expected = {e["id"]: (c["id"], e) for c in cases for e in c.get("expected") or []}
    by_case = {c["id"]: c for c in cases}
    corpora = {c["id"]: _Corpus(c) for c in cases}
    case_ids = {c["id"] for c in cases}
    gaps: dict[str, list[Gap]] = {}
    for s in scans:
        for g in s.gaps:
            if g.exp not in expected:
                findings.append(Finding("unknown-exp", ERROR, f"@gap {g.exp} is not an expected result of these test cases", g.file, g.line,
                                        exp=g.exp))
            else:
                gaps.setdefault(g.exp, []).append(g)
    marked = {cid for s in scans for cs in s.functions.values() for cid in cs}
    for cid in sorted(case_ids - marked):
        findings.append(Finding("missing-test", ERROR, f"no test function marked @pytest.mark.tc({cid!r})", case=cid))
    by_exp: dict[str, list[Assertion]] = {}
    for s in scans:
        for a in s.assertions:
            if a.constant:
                findings.append(Finding("vacuous", ERROR, f"constant assertion `assert {a.source}`", a.file, a.line))
            if a.in_try:
                findings.append(Finding("in-try", ERROR, "assertion inside try/except can never fail", a.file, a.line))
            if not a.exp:
                findings.append(Finding("extra", ERROR, f"assertion without @exp: `assert {a.source}` (only spec expectations may be asserted)",
                                        a.file, a.line))
            for x in a.exp:
                if x not in expected:
                    findings.append(Finding("unknown-exp", ERROR, f"@exp {x} is not an expected result of these test cases", a.file, a.line,
                                            exp=x))
                else:
                    by_exp.setdefault(x, []).append(a)
    manifest = []
    for x, (cid, e) in sorted(expected.items()):
        found = by_exp.get(x, [])
        row = {"exp": x, "case": cid, "kind": e.get("kind"), "op": e.get("op"), "value": e.get("value"), "status": "ok"}
        marked_gap = gaps.get(x, [])
        if marked_gap and found:
            findings.append(Finding("gap-and-assert", ERROR, f"{x} is both asserted and marked @gap; keep one", marked_gap[0].file,
                                    marked_gap[0].line, cid, x))
        elif marked_gap:
            g = marked_gap[0]
            if not _may_be_gap(by_case[cid], e):
                findings.append(Finding("gap-not-allowed", ERROR, f"{x} is marked @gap but the spec defines it ({e.get('op')} "
                                        f"{str(e.get('value'))[:80]!r}) and no open TestGap is behind it: assert it", g.file, g.line, cid, x))
                manifest.append({**row, "status": "mismatch", "file": g.file, "line": g.line, "function": g.function})
            else:
                findings.append(Finding("spec-gap", INFO, f"{x} not asserted: {g.reason or 'the spec leaves it open'}", g.file, g.line, cid, x))
                manifest.append({**row, "status": "gap", "file": g.file, "line": g.line, "function": g.function, "reason": g.reason})
            continue
        if not found:
            findings.append(Finding("missing", ERROR, f"no assertion for expected result {x} ({e.get('op')} {e.get('value')!r})", case=cid,
                                    exp=x))
            manifest.append({**row, "status": "missing"})
            continue
        if len(found) > 1:
            findings.append(Finding("duplicate", ERROR, f"{len(found)} assertions carry @exp {x}", found[1].file, found[1].line, cid, x))
        a = found[0]
        row.update(file=a.file, line=a.line, function=a.function, assertion=a.source)
        errors_before = len(findings)
        if cid not in a.cases:
            findings.append(Finding("wrong-test", ERROR, f"@exp {x} sits in {a.function}, which is not marked tc({cid!r})", a.file, a.line,
                                    cid, x))
        invented = [lit for lit in a.literals if not corpora[cid].has(lit)]
        if invented:
            findings.append(Finding("value-not-in-spec", ERROR, f"the assertion uses {', '.join(repr(v) for v in invented[:5])}, which the spec "
                                    "and the contract of this case never state: assert only what the spec says, or mark the expected @gap "
                                    "when it is left open", a.file, a.line, cid, x))
        want_op = str(e.get("op") or "")
        if want_op in OPS.values() and a.op != want_op:
            findings.append(Finding("op-mismatch", ERROR, f"spec operator is {want_op!r}, the assertion uses {a.op!r}", a.file, a.line, cid,
                                    x))
        value = str(e.get("value") if e.get("value") is not None else "")
        if ATOMIC_RE.fullmatch(value):
            if not a.has_literal:
                findings.append(Finding("value-not-literal", ERROR, f"compare with the spec value {value!r} written as a literal", a.file,
                                        a.line, cid, x))
            elif not _same(value, a.literal):
                findings.append(Finding("value-mismatch", ERROR, f"spec value is {value!r}, the assertion uses {a.literal!r}", a.file, a.line,
                                        cid, x))
        elif _num(value) is not None and a.has_literal and _num(a.literal) is not None:
            if _num(a.literal) != _num(value):
                findings.append(Finding("value-unit", WARNING, f"spec value {value!r} vs literal {a.literal!r}: check the unit", a.file,
                                        a.line, cid, x))
        elif value:
            findings.append(Finding("value-unchecked", INFO, f"spec value is prose ({value[:80]!r}); only the operator was checked", a.file,
                                    a.line, cid, x))
        if any(f.severity == ERROR for f in findings[errors_before:]):
            row["status"] = "mismatch"
        manifest.append(row)
    return findings, manifest


def guard(baseline: dict[str, str], scans: list[Scan]) -> list[Finding]:
    """Diff guard for fix rounds: an assertion that already matched the spec must stay byte-for-byte (after unparse) the same."""
    now = {x: a.source for s in scans for a in s.assertions for x in a.exp}
    out = []
    for x, src in sorted(baseline.items()):
        if now.get(x) != src:
            out.append(Finding("diff-guard", ERROR, f"fix round changed the verified assertion {x}: `{src}` -> `{now.get(x)}`", exp=x))
    return out


def verified(findings: list[Finding], manifest: list[dict]) -> dict[str, str]:
    """Assertions with no error against them: the baseline the diff guard protects."""
    bad = {f.exp for f in findings if f.severity == ERROR and f.exp}
    return {r["exp"]: r["assertion"] for r in manifest if r["status"] == "ok" and r["exp"] not in bad}


def as_dicts(findings: list[Finding]) -> list[dict]:
    return [{k: v for k, v in asdict(f).items() if v not in (None, "")} for f in findings]
