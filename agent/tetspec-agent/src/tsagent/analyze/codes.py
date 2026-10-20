"""Test KG node codes (TDD v2 section 5.6): derived from source codes, never sequential, stable across versions.

Classes missing from table 5.6 use their own prefixes (DEVIATIONS D11): TB (TestBasisItem), TFM (FaultModel), TPRT (Partition),
TPATH (ScenarioPath), TRISK (RiskRating), TGAP (TestGap), TPC (PassCriterion).
"""
import hashlib
import re

MAX_LEN = 96
_BAD = re.compile(r"[^A-Za-z0-9_.]+")


def norm(part):
    """One code segment: only A-Z a-z 0-9 _ . and '-', no double '-' and none at either end."""
    s = _BAD.sub("-", str(part)).strip("-")
    return re.sub(r"-{2,}", "-", s) or "x"


def short_hash(*parts, n=6):
    return hashlib.sha256("\x00".join(str(p) for p in parts).encode("utf-8")).hexdigest()[:n]


def make(prefix, *parts):
    code = "-".join([prefix] + [norm(p) for p in parts if p not in (None, "")])
    if len(code) <= MAX_LEN:
        return code
    tail = short_hash(code, n=8)
    return code[:MAX_LEN - 9].rstrip("-") + "-" + tail


def basis(feature, tdd_code):
    return make("TB", feature, tdd_code)


def obligation(feature, subject, group):
    return make("TOB", feature, subject, group)


def condition(feature, subject, kind, key):
    return make("TCND", feature, subject, kind, key)


def oracle(feature, source, key=None):
    return make("TORC", feature, source, key)


def fault_model(feature, subject, key):
    return make("TFM", feature, subject, key)


def partition(feature, subject, key):
    return make("TPRT", feature, subject, key)


def path(feature, subject, key):
    return make("TPATH", feature, subject, key)


def risk(feature, subject):
    return make("TRISK", feature, subject)


def gap(feature, subject, key):
    return make("TGAP", feature, subject, key)


def requirement(feature, level, subject, group):
    return make("TR", feature, level, subject, group)


def pass_criterion(tr_code, ordinal):
    return make("TPC", tr_code.removeprefix("TR-"), ordinal)


def design(feature, level, subject, technique):
    return make("TD", feature, level, subject, technique)


def coverage(feature, level, cond_code, key):
    return make("TCOV", feature, level, _cond_tail(cond_code, feature), key)


def testcase(level, feature, cond_code, key):
    return make("TC", level, feature, _cond_tail(cond_code, feature), key)


def _cond_tail(cond_code, feature):
    """Drop the repeated `TCND-<feature>-` when a condition code is embedded in a later-layer code."""
    head = f"TCND-{norm(feature)}-"
    return cond_code[len(head):] if cond_code.startswith(head) else cond_code


# ------------------------------------------------------------------ layer C (DEVIATIONS D11)
def _tail(code, prefix):
    return code.split("-", 1)[1] if code.startswith(prefix + "-") else code


def step(tc_code, ordinal):
    return make("TSTEP", _tail(tc_code, "TC"), ordinal)


def assertion(tc_code, ordinal):
    return make("TAS", _tail(tc_code, "TC"), ordinal)


def test_data(tc_code):
    return make("TDATA", _tail(tc_code, "TC"))


def data_field(tc_code, name):
    return make("TDF", _tail(tc_code, "TC"), name)


def fixture(feature, subject, key):
    return make("TFX", feature, subject, key)


def stub(feature, target, behavior, status=None):
    return make("TSTUB", feature, target, behavior, status)


def fault_injection(fm_code):
    return make("TFI", _tail(fm_code, "TFM"))


def test_set(feature, level, subject):
    return make("TSET", feature, level, subject)


def environment(feature, level):
    return make("TENV", feature, level)


def primary_subject(g, obligation):
    """Code of the main TDD subject node of an obligation (a Scenario obligation also has the participating Modules)."""
    subs = [g.get(s).attrs["tddCode"] for s in obligation.rel("subject") if g.get(s) is not None]
    head = f"TOB-{norm(g.feature)}-"
    for tdd in subs:
        if obligation.code.startswith(head + norm(tdd) + "-"):
            return tdd
    return subs[0] if subs else None
