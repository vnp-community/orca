"""T7.3: MCP contract (TST-17): input/output schema of each tool, auth, Origin, size, sql_select only on v_* views, timeout,
start_run in the background, review tools only when enabled, stdio."""
import io
import json
import threading
import time
import urllib.error
import urllib.request

import pytest

from testspec_agent.conftest import make_cfg
from tsagent.gateway import tool_catalog
from tsagent.gateway.server import MAX_TEXT, make_server, serve_stdio
from tsagent.store.db import Database

pytestmark = pytest.mark.contract
TOKEN = "test-token-123"
OLD = {"list_specs", "get_spec", "get_testcase", "get_traceability", "get_coverage", "get_validation", "get_version_history",
       "diff_versions", "search", "get_neighbors", "start_run", "get_run_status", "list_pending_review"}
NEW = {"get_analysis", "list_gaps", "sql_select", "get_tcir"}
REVIEW = {"approve_nodes", "reject_nodes", "baseline_spec"}


class Client:
    def __init__(self, url, token=TOKEN):
        self.url, self.token, self.id = url, token, 0

    def raw(self, body, headers=None, token=True):
        h = {"Content-Type": "application/json", **(headers or {})}
        if token and self.token:
            h["Authorization"] = f"Bearer {self.token}"
        req = urllib.request.Request(self.url, body if isinstance(body, bytes) else json.dumps(body).encode(), h, method="POST")
        try:
            with urllib.request.urlopen(req, timeout=30) as r:
                data = r.read()
                return r.status, json.loads(data) if data else None
        except urllib.error.HTTPError as e:
            return e.code, None

    def rpc(self, method, params=None):
        self.id += 1
        st, data = self.raw({"jsonrpc": "2.0", "id": self.id, "method": method, "params": params or {}})
        assert st == 200, st
        return data

    def call(self, name, args=None):
        res = self.rpc("tools/call", {"name": name, "arguments": args or {}})["result"]
        text = res["content"][0]["text"]
        return res["isError"], (json.loads(text) if not res["isError"] and text[:1] in "{[" else text)

    def ok(self, name, args=None):
        err, out = self.call(name, args)
        assert not err, out
        return out


def wait(c, run_id, timeout=60):
    end = time.time() + timeout
    while time.time() < end:
        st = c.ok("get_run_status", {"runId": run_id})
        if st["state"] in ("done", "failed"):
            return st
        time.sleep(0.1)
    raise AssertionError("run chưa xong")


@pytest.fixture(scope="module")
def server(tmp_path_factory, pay_docs_m):
    tmp = tmp_path_factory.mktemp("mcp")
    cfg = make_cfg(tmp, token=TOKEN, mcp_review=True)
    Database(cfg.db).init_schema()
    srv = make_server(cfg, "127.0.0.1", 0)
    th = threading.Thread(target=srv.serve_forever, daemon=True)
    th.start()
    c = Client(f"http://127.0.0.1:{srv.server_address[1]}/mcp")
    docs = [{"name": p.rsplit("/", 1)[-1], "content": open(p, encoding="utf-8").read()} for p in pay_docs_m]
    r = c.ok("start_run", {"feature": "PAY", "documents": docs})
    assert wait(c, r["runId"])["state"] == "done"
    yield srv, c, cfg, docs
    srv.shutdown()


@pytest.fixture(scope="module")
def pay_docs_m():
    import os

    from testspec_agent.conftest import PAY
    return [os.path.join(PAY, f) for f in ("urd.md", "srs.md", "tdd.md")]


def test_initialize_and_tools(server):
    _, c, cfg, _ = server
    init = c.rpc("initialize", {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "t", "version": "1"}})
    assert init["result"]["serverInfo"]["name"] == "tsagent"
    names = {t["name"] for t in c.rpc("tools/list")["result"]["tools"]}
    assert OLD | NEW | REVIEW <= names
    for t in c.rpc("tools/list")["result"]["tools"]:
        assert t["inputSchema"]["type"] == "object" and t["description"]
    assert c.rpc("nope")["error"]["code"] == -32601


def test_catalog_file_up_to_date():
    from tsagent.gateway import SCHEMA_FILE
    assert json.load(open(SCHEMA_FILE, encoding="utf-8")) == json.loads(json.dumps(tool_catalog(), sort_keys=True))


def test_auth_origin_size_health(server):
    srv, c, cfg, _ = server
    body = {"jsonrpc": "2.0", "id": 1, "method": "ping"}
    assert c.raw(body, token=False)[0] == 401
    assert Client(c.url, "sai").raw(body)[0] == 401
    assert c.raw(body, {"Origin": "https://evil.example"})[0] == 403
    assert c.raw(body, {"Origin": "http://localhost:3000"})[0] == 200
    assert c.raw(b"x" * (cfg.max_body_kb * 1024 + 10))[0] == 413
    with urllib.request.urlopen(c.url.replace("/mcp", "/health")) as r:
        assert json.loads(r.read())["status"] == "ok"


def test_external_bind_requires_token(tmp_path):
    from tsagent.errors import ConfigError
    with pytest.raises(ConfigError):
        make_server(make_cfg(tmp_path, token=""), "0.0.0.0", 0)


def test_read_tools_shapes(server):
    _, c, _, _ = server
    assert c.ok("list_specs")["items"][0]["feature"] == "PAY"
    hist = c.ok("get_version_history", {"feature": "PAY"})
    assert hist[-1]["version"] == 1 and "changes" in hist[-1]
    cov = c.ok("get_coverage", {"feature": "PAY"})
    assert {"testRequirements", "coverageItems", "testCases"} <= set(cov["counts"]) and cov["counts"]["testCases"] > 0
    val = c.ok("get_validation", {"feature": "PAY"})
    assert {"blocking", "baselineBlockers", "warnings", "rules"} <= set(val)
    assert {r["level"] for r in val["rules"]} <= {"block", "block_baseline", "warn"}
    tr = c.ok("get_traceability", {"feature": "PAY", "compact": True})
    assert {"FR-001", "NFR-001"} <= {x["source"] for x in tr}
    pend = c.ok("list_pending_review", {"feature": "PAY"})
    assert {"total", "byClass", "pendingConfirmation", "inferred", "inferredCount", "pendingConfirmationCount"} <= set(pend)
    an = c.ok("get_analysis", {"feature": "PAY", "subject": "INT-01"})
    assert an["obligations"] and an["obligations"][0]["conditions"][0]["oracle"]
    gaps = c.ok("list_gaps", {"feature": "PAY"})
    assert gaps["total"] == len(gaps["gaps"]) > 0 and gaps["gaps"][0]["owner"]
    page = c.ok("list_gaps", {"feature": "PAY", "limit": 1})
    assert len(page["gaps"]) == 1 and page["total"] == gaps["total"] and (page["next"] == 1 or gaps["total"] == 1)
    assert all(isinstance(x["testCases"], int) for x in tr)  # compact: one row per subject, with counts
    full = c.ok("get_traceability", {"feature": "PAY"})
    code = next(x for x in full if x["testCases"])["testCases"][0]
    tc = c.ok("get_testcase", {"feature": "PAY", "code": code})
    assert tc["steps"] and tc["assertions"][0]["oracle"]
    one = c.ok("get_tcir", {"feature": "PAY", "code": code})
    assert one["schemaVersion"] == "tcir/v1" and one["id"] == code and one["data"]["policy"] == "generate"
    assert [e["id"] for e in one["expected"]] == [f"{code}.E{a['ordinal']}" for a in tc["assertions"]]
    assert all(e["oracle"]["code"] == a["rels"]["oracle"][0] for e, a in zip(one["expected"], tc["assertions"], strict=True))
    page = c.ok("get_tcir", {"feature": "PAY", "limit": 1})
    assert len(page["testCases"]) == 1 and page["total"] >= 1 and page["specVersion"] >= 1
    s = c.ok("search", {"feature": "PAY", "query": "INT-01"})
    assert s["total"] > 0
    nb = c.ok("get_neighbors", {"feature": "PAY", "code": "INT-01"})
    assert nb["nodes"]
    err, msg = c.call("get_spec", {"feature": "../etc", "docKind": "TestCases"})
    assert err and "invalid arguments" in msg


def test_get_spec_paging(server):
    _, c, _, _ = server
    parts, off = [], 0
    while off is not None:
        r = c.ok("get_spec", {"feature": "PAY", "docKind": "TestCases", "offset": off, "max_chars": 5000})
        parts.append(r["text"])
        off = r["next"]
    assert len("".join(parts)) == r["total"] and "".join(parts).startswith("---")


def test_truncation_marker(server):
    _, c, _, _ = server
    err, text = c.call("get_traceability", {"feature": "PAY"})
    if len(text) >= MAX_TEXT - 100:
        assert "cut at" in text[-200:]


def test_sql_select_guard(server):
    _, c, _, _ = server
    out = c.ok("sql_select", {"query": "SELECT code, condition_kind FROM v_test_condition ORDER BY code", "limit": 5})
    assert out["columns"] and len(out["rows"]) == 5 and out["truncated"]
    for q in ("SELECT * FROM src_node", "SELECT * FROM tso_node", "DELETE FROM v_test_condition", "PRAGMA table_info(run)",
              "ATTACH DATABASE '/tmp/x.db' AS x", "SELECT * FROM sqlite_master"):
        err, msg = c.call("sql_select", {"query": q})
        assert err and ("refused" in msg or "not authorized" in msg.lower()), (q, msg)
    err, msg = c.call("sql_select", {"query": "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c) SELECT count(*) FROM c"})
    assert err and "exceeded" in msg


def test_start_run_paths_outside_roots_rejected(server):
    _, c, _, _ = server
    err, msg = c.call("start_run", {"feature": "X", "paths": ["/etc/hostname"]})
    assert err and "INGEST-ROOT" in msg


def test_start_run_dedup(server):
    _, c, _, docs = server
    r = c.ok("start_run", {"feature": "PAY", "documents": docs})
    assert r["deduplicated"]


def test_review_tools_and_baseline(server):
    _, c, _, _ = server
    assert c.ok("approve_nodes", {"feature": "PAY", "by": "qa"})["updated"] > 0
    res = c.ok("baseline_spec", {"feature": "PAY", "by": "qa"})
    assert res["version"] == 1 and c.ok("get_version_history", {"feature": "PAY"})[-1]["status"] == "Baselined"


def test_review_tools_hidden_without_flag(tmp_path):
    cfg = make_cfg(tmp_path, mcp_review=False)
    Database(cfg.db).init_schema()
    out = io.StringIO()
    lines = [json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/list"}),
             json.dumps({"jsonrpc": "2.0", "method": "notifications/initialized"}),
             json.dumps({"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": "approve_nodes", "arguments": {}}})]
    serve_stdio(cfg, io.StringIO("\n".join(lines) + "\n"), out)
    res = [json.loads(x) for x in out.getvalue().splitlines()]
    assert len(res) == 2  # a notification gets no response
    assert not REVIEW & {t["name"] for t in res[0]["result"]["tools"]}
    assert res[1]["result"]["isError"] and "TSAGENT_MCP_REVIEW" in res[1]["result"]["content"][0]["text"]
