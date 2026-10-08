#!/usr/bin/env python3
"""Chạy toàn bộ bộ kiểm tra API backend-go.

    python run_all.py                      # tất cả suite
    python run_all.py auth projects        # chỉ một số suite
    python run_all.py --list               # liệt kê suite
    python run_all.py --verify-catalog     # đối chiếu danh mục route với mã Go
    python run_all.py --no-coverage        # bỏ báo cáo độ phủ route

Cấu hình đọc từ tests/backend/.env (xem .env.example). Mã thoát khác 0 nếu có FAIL.
"""
from __future__ import annotations

import argparse
import importlib
import sys

from check_framework import build_context, run_suite, summarize
from orca_api_session import EXERCISED
from orca_route_catalog import all_routes, normalize, verify_against_go_source

# Thứ tự quan trọng: auth trước (rate-limit login), sweep xác thực sau, rồi các dịch vụ.
SUITES = [
    ("auth", "api_auth"),
    ("auth-enforcement", "api_auth_enforcement"),
    ("admin", "api_admin"),
    ("tenants", "api_tenants"),
    ("projects", "api_projects"),
    ("tasks", "api_tasks"),
    ("annotations-git", "api_annotations_git"),
    ("automation-workflow", "api_automation_workflow"),
    ("scm-issues", "api_scm_issues"),
    ("infra-orchestration", "api_infra_orchestration"),
    ("notifications-usage-ai", "api_notifications_usage_ai"),
    ("websocket", "api_websocket_channels"),
]


def verify_catalog() -> int:
    res = verify_against_go_source()
    if res is None:
        print("Không tìm thấy mã nguồn Go — bỏ qua --verify-catalog.")
        return 0
    missing, extra = res
    for m, p in sorted(missing):
        print(f"THIẾU trong danh mục (có trong Go): {m} {p}")
    for m, p in sorted(extra):
        print(f"THỪA trong danh mục (không có trong Go): {m} {p}")
    print("Danh mục route khớp mã Go." if not (missing or extra) else "Danh mục LỆCH mã Go.")
    return 1 if (missing or extra) else 0


def print_coverage() -> None:
    hit = {normalize(r) for r in EXERCISED}
    # {x} trong template của suite có thể khác tên tham số -> so sánh sau chuẩn hoá.
    missing = [r for r in all_routes() if normalize(r) not in hit]
    total = len(all_routes())
    print(f"\nĐộ phủ route HTTP: {total - len(missing)}/{total}")
    for m, p in missing:
        print(f"  chưa gọi: {m} {p}")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("suites", nargs="*", help="tên suite (mặc định: tất cả)")
    ap.add_argument("--list", action="store_true")
    ap.add_argument("--verify-catalog", action="store_true")
    ap.add_argument("--no-coverage", action="store_true")
    args = ap.parse_args()

    if args.list:
        for name, mod in SUITES:
            print(f"{name:24s} {mod}.py")
        return 0
    if args.verify_catalog:
        return verify_catalog()

    wanted = set(args.suites)
    unknown = wanted - {n for n, _ in SUITES}
    if unknown:
        print(f"Suite không tồn tại: {sorted(unknown)} (dùng --list)")
        return 2

    ctx = build_context()
    if not ctx.cfg.has_admin_credentials:
        print("[warn] thiếu ORCA_ADMIN_EMAIL/ORCA_ADMIN_PASSWORD — các suite cần đăng nhập sẽ bị bỏ qua")
    for name, mod_name in SUITES:
        if wanted and name not in wanted:
            continue
        mod = importlib.import_module(mod_name)
        print(f"\n=== {name} ===")
        run_suite(ctx, mod.SUITE, mod.run)
    code = summarize(ctx)
    if not args.no_coverage and not wanted:
        print_coverage()
    return code


if __name__ == "__main__":
    sys.exit(main())
