#!/usr/bin/env python3
"""Chạy toàn bộ bộ kiểm tra tích hợp (integrated) theo features và services.

    python run_all.py                      # tất cả suite
    python run_all.py 01-auth auth-service # chỉ một số suite
    python run_all.py --list               # liệt kê các suite kiểm tra
    python run_all.py --verify-catalog     # đối chiếu route catalog với Go source
    python run_all.py --no-coverage        # bỏ qua báo cáo route coverage
"""
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
    # Skip directories not using python test runner
    ignored_subdirs = {'client', 'server', 'node_modules', '.venv', '__pycache__'}
    for root, dirs, files in os.walk(BASE_DIR):
        rel_parts = Path(root).relative_to(BASE_DIR).parts
        if any(part in ignored_subdirs for part in rel_parts):
            continue
        if root == str(BASE_DIR):
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
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("suites", nargs="*", help="tên suite (thư mục feature hoặc service)")
    ap.add_argument("--list", action="store_true", help="liệt kê tất cả các suite khả dụng")
    ap.add_argument("--verify-catalog", action="store_true", help="đối chiếu catalog route với Go")
    ap.add_argument("--no-coverage", action="store_true", help="không in báo cáo route coverage")
    args = ap.parse_args()

    test_files = get_test_files()
    available_suites = list(dict.fromkeys(suite_name for suite_name, _ in test_files))

    if args.list:
        for suite_name, path in test_files:
            rel = path.relative_to(BASE_DIR)
            print(f"{suite_name:32s} {rel}")
        return 0

    if args.verify_catalog:
        return verify_catalog()

    wanted = set(args.suites)
    unknown = wanted - set(available_suites)
    if unknown:
        print(f"Suite không tồn tại: {sorted(unknown)} (dùng --list để xem danh sách)")
        return 2

    ctx = build_context()
    if not ctx.cfg.has_admin_credentials:
        print("[warn] thiếu ORCA_ADMIN_EMAIL/ORCA_ADMIN_PASSWORD — các suite cần đăng nhập sẽ bị bỏ qua")

    for suite_name, path in test_files:
        if wanted and suite_name not in wanted:
            continue
        try:
            mod = load_module_from_file(path.stem, path)
            print(f"\n=== {suite_name} ({path.name}) ===")
            run_suite(ctx, getattr(mod, 'SUITE', suite_name), mod.run)
        except Exception as e:
            print(f"Lỗi khi chạy {path.name}: {e}")

    code = summarize(ctx)
    if not args.no_coverage and not wanted:
        print_coverage()
    return code

if __name__ == "__main__":
    sys.exit(main())
