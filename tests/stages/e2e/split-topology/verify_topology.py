"""TOP-01..07: Orca and the dev servers really are separate hosts (TDD-WF-AGENT-001 §11.1).

Usage: python3 verify_topology.py [--multi]      (stack from split-up.sh; ORCA_E2E_GATEWAY_URL if not the default)
"""
from __future__ import annotations

import sys
import time

from split_harness import (Blocked, create_template, docker, execute, in_devserver, open_context, project_on, settle, step_rows,
                           write_report)

A, B = "e2e-devserver-a", "e2e-devserver-b"
FIXTURE_REPO = "/srv/projects/fixture-repo"
INTERNAL = ("infra-fleet-service", "workflow-service", "postgres", "nats")


def tcp_probe(container: str, host: str, port: int) -> bool:
    """True when container can open host:port (bash /dev/tcp, 3 s)."""
    out = docker("exec", container, "bash", "-c", f"timeout 3 bash -c '</dev/tcp/{host}/{port}' 2>/dev/null && echo open || echo closed")
    return out.stdout.strip() == "open"


def container_ip(name: str, network_suffix: str) -> str:
    out = docker("inspect", "-f", "{{range $k, $v := .NetworkSettings.Networks}}{{$k}}={{$v.IPAddress}} {{end}}", name)
    for pair in out.stdout.split():
        net, _, ip = pair.partition("=")
        if net.endswith(network_suffix):
            return ip
    raise Blocked(f"no {network_suffix} address for {name}: {out.stdout.strip() or out.stderr.strip()}")


def main(multi: bool) -> int:
    ctx = open_context(f"top-{int(time.time())}")
    rec, ws = ctx.rec, ctx.ws
    rec.current_suite = "topology"

    def registered():
        for name in (A, B) if multi else (A,):
            s = next((x for x in ws.call("devServer.list", {}) if x.get("name") == name), None)
            assert s, f"{name} not registered"
            assert s.get("approvalStatus") == "approved" and s.get("status") == "connected", f"{name}: {s.get('approvalStatus')}/{s.get('status')}"
    rec.run("TOP-01", "Dev servers are registered, approved and connected", registered)

    def no_internal_names():
        leaked = [h for h in INTERNAL if in_devserver(A, f"getent hosts {h}").returncode == 0]
        assert not leaked, f"the dev server resolves control-plane names: {leaked}"
    rec.run("TOP-02", "A dev server cannot resolve control-plane service names", no_internal_names)

    def no_internal_tcp():
        ip = container_ip("orca-go-infra-fleet", "orca-go-net")
        assert not tcp_probe("orca-e2e-devserver-a", ip, 9090), f"the dev server reached infra-fleet-service gRPC at {ip}:9090"
        pg = container_ip("orca-go-postgres", "orca-go-net")
        assert not tcp_probe("orca-e2e-devserver-a", pg, 5432), f"the dev server reached Postgres at {pg}:5432"
    rec.run("TOP-03", "A dev server cannot open TCP to control-plane addresses", no_internal_tcp)

    def control_plane_cannot_reach_devserver():
        ip = container_ip("orca-e2e-devserver-a", "orca-e2e-wan")
        net = next(n for n in docker("network", "ls", "--format", "{{.Name}}").stdout.split() if n.endswith("orca-go-net") and n.startswith("orca-e2e"))
        out = docker("run", "--rm", "--network", net, "busybox:1.36", "sh", "-c",
                     f"nslookup devserver-a >/dev/null 2>&1 && echo resolves; nc -z -w 3 {ip} 22 && echo open; ping -c1 -W2 {ip} >/dev/null 2>&1 && echo ping; true", timeout=60)
        assert not out.stdout.strip(), f"a control-plane container reached the dev server: {out.stdout.strip()}"
    rec.run("TOP-04", "The control plane cannot reach a dev server except through its agent connection", control_plane_cannot_reach_devserver)

    def gateway_over_wan():
        assert in_devserver(A, "getent hosts orca-gateway").returncode == 0, "orca-gateway does not resolve on the WAN"
        assert tcp_probe("orca-e2e-devserver-a", "orca-gateway", 8081), "orca-gateway:8081 not reachable from the dev server"
    rec.run("TOP-05", "A dev server reaches the public gateway over the WAN", gateway_over_wan)

    def runs_on(server: str, test_label: str) -> None:
        pid = project_on(ctx, server, FIXTURE_REPO, test_label)
        tid = create_template(ctx, test_label, [{"id": "where", "type": "shell", "config": {"target": f"project:{pid}", "script": "hostname; cat /etc/hostname"}}])
        eid = execute(ctx, tid, pid)
        status = settle(ctx, eid, timeout=90)
        row = step_rows(eid).get("where", {})
        stdout = (row.get("output") or {}).get("stdout", "")
        assert status == "completed", f"execution {status}: {row.get('error') or row.get('output')}"
        expected = server.removeprefix("e2e-")
        assert expected in stdout and "orca-go" not in stdout, f"the shell step ran on {stdout.strip()!r}, expected {expected}"

    rec.run("TOP-06", "A workflow step runs on the project's dev server, not on the Orca host", lambda: runs_on(A, "top06"))

    def other_server():
        if not multi:
            raise Blocked("start with split-up.sh --multi")
        runs_on(B, "top07")
    rec.run("TOP-07", "Steps are routed to the dev server of their own project", other_server)

    problems = ctx.run_cleanups()
    ctx.session.close()
    return write_report("split-topology", ctx, {"cleanup_problems": problems})


if __name__ == "__main__":
    sys.exit(main("--multi" in sys.argv[1:]))
