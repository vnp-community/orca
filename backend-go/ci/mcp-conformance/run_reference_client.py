#!/usr/bin/env python3
"""MCP reference-client conformance (tier 1b): the official `mcp` Python SDK as an
independent client of Orca's /mcp endpoint.

  MCP_CONF_BASE_URL=http://localhost:8081 MCP_CONF_TOKEN=<dev PAT> python run_reference_client.py

Exit code 0 only if every check passed. Skipped steps are reported, never silent.
"""
from __future__ import annotations

import asyncio
import sys

import check_initialize_tools
import check_oauth_discovery
import check_resumability
from conformance_support import Config, Result


async def main() -> int:
    cfg = Config.from_env()
    results: list[Result] = []
    for mod in (check_initialize_tools, check_resumability, check_oauth_discovery):
        try:
            results += await mod.run(cfg)
        except Exception as e:  # a crash is a failure of that tier step, with the reason
            results.append(Result(f"{mod.__name__} crashed", False, f"{type(e).__name__}: {e}"))
    for r in results:
        print(r.line())
    failed = [r for r in results if not r.ok]
    print(f"\n{len(results) - len(failed)} passed, {len(failed)} failed against {cfg.mcp_url}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
