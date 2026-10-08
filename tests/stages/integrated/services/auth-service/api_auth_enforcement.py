"""Quét xác thực: mọi route yêu cầu session phải trả 401 khi không có cookie/bearer.

Danh sách route lấy từ orca_route_catalog (đối chiếu với mã Go bằng run_all.py --verify-catalog).
Spec: api-gateway.md §6 (authMiddleware), 07-security-architecture.md.
"""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

import re

from check_framework import Context, run_single
from orca_route_catalog import authenticated_routes

SUITE = "auth-enforcement"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    bad_cookie = "orca_session=invalid-session-token"
    for method, template in authenticated_routes():
        path = re.sub(r"\{[^}]*\}", ZERO_UUID, template)
        label = f"{method} {template}"
        ctx.expect(f"{label} không xác thực -> 401",
                   ctx.anon.request(method, path, template=template, auth=False), 401)
    # Cookie giả cũng phải bị từ chối, không được rơi vào placeholder bearer.
    for template in ("/v1/projects/", "/admin/api/stats", "/v1/tasks/" + ZERO_UUID):
        ctx.expect(f"cookie sai {template} -> 401",
                   ctx.anon.get(template, raw_cookie=bad_cookie, auth=False,
                                template=template.replace(ZERO_UUID, "{id}")), 401)


if __name__ == "__main__":
    run_single(SUITE, run)
