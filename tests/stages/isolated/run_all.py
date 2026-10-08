#!/usr/bin/env python3
"""Chạy toàn bộ bộ kiểm tra API backend-go được phân chia theo thư mục."""
from __future__ import annotations

import argparse
import importlib.util
import os
import sys
from pathlib import Path

from check_framework import build_context, run_suite, summarize
from orca_api_session import EXERCISED
from orca_route_catalog import all_routes, normalize, verify_against_go_source

BASE_DIR = Path(__file__).resolve().parent

def get_test_files():
    test_files = []
    for root, dirs, files in os.walk(BASE_DIR):
        if 'backend' in root or root == str(BASE_DIR):
            continue
        for file in files:
            if file.endswith('.py') and (file.startswith('api_') or file.startswith('test_')):
                test_files.append((Path(root).name, Path(root) / file))
    return sorted(test_files)

def load_module_from_file(module_name, file_path):
    spec = importlib.util.spec_from_file_location(module_name, file_path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[module_name] = module
    spec.loader.exec_module(module)
    return module

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
    missing = [r for r in all_routes() if normalize(r) not in hit]
    total = len(all_routes())
    print(f"\nĐộ phủ route HTTP: {total - len(missing)}/{total}")
    for m, p in missing:
        print(f"  chưa gọi: {m} {p}")

def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("suites", nargs="*", help="tên suite (thư mục service)")
    ap.add_argument("--list", action="store_true")
    ap.add_argument("--verify-catalog", action="store_true")
    ap.add_argument("--no-coverage", action="store_true")
    args = ap.parse_args()

    test_files = get_test_files()
    available_suites = list(dict.fromkeys(svc for svc, _ in test_files))

    if args.list:
        for svc, path in test_files:
            print(f"{svc:24s} {path.name}")
        return 0

    if args.verify_catalog:
        return verify_catalog()

    wanted = set(args.suites)
    unknown = wanted - set(available_suites)
    if unknown:
        print(f"Suite không tồn tại: {sorted(unknown)} (dùng --list)")
        return 2

    ctx = build_context()
    if not ctx.cfg.has_admin_credentials:
        print("[warn] thiếu ORCA_ADMIN_EMAIL/ORCA_ADMIN_PASSWORD — các suite cần đăng nhập sẽ bị bỏ qua")
    
    for svc, path in test_files:
        if wanted and svc not in wanted:
            continue
        try:
            mod = load_module_from_file(path.stem, path)
            print(f"\n=== {svc} ({path.name}) ===")
            run_suite(ctx, getattr(mod, 'SUITE', svc), mod.run)
        except Exception as e:
            print(f"Lỗi khi chạy {path.name}: {e}")

    code = summarize(ctx)
    if not args.no_coverage and not wanted:
        print_coverage()
    return code

if __name__ == "__main__":
    sys.exit(main())
