import os
import sys
import shutil
from pathlib import Path

BASE_DIR = Path('/Users/binhnt/Work/blockchain/vnp-blc/orca/tests/stages/isolated/services')
BACKEND_DIR = BASE_DIR / 'backend'

SERVICES = [
    'auth-service', 'tenant-service', 'project-service', 'infra-fleet-service',
    'ai-provider-service', 'workflow-service', 'task-service', 'orchestration-service',
    'automation-service', 'annotation-service', 'notification-service', 'usage-service',
    'mcp-service', 'request-service', 'credential-broker-service', 'git-gateway-service',
    'scm-integration-service', 'issue-tracking-service', 'issue-status-sync', 'api-gateway'
]

FILE_MAPPING = {
    'api_auth.py': 'auth-service',
    'api_auth_enforcement.py': 'auth-service',
    'api_admin.py': 'auth-service',
    'api_tenants.py': 'tenant-service',
    'api_projects.py': 'project-service',
    'api_tasks.py': 'task-service',
    'api_automation_workflow.py': 'automation-service',
    'api_infra_orchestration.py': 'infra-fleet-service',
    'api_scm_issues.py': 'scm-integration-service',
    'api_notifications_usage_ai.py': 'notification-service',
    'api_annotations_git.py': 'annotation-service',
    'api_websocket_channels.py': 'api-gateway',
}

def ensure_dirs():
    for svc in SERVICES:
        (BASE_DIR / svc).mkdir(parents=True, exist_ok=True)

def migrate_framework():
    framework_files = ['check_framework.py', 'env_config.py', 'orca_api_session.py', 'orca_route_catalog.py', '.env', '.env.example', 'requirements.txt']
    for f in framework_files:
        src = BACKEND_DIR / f
        if src.exists():
            shutil.copy(src, BASE_DIR / f)

def fix_imports(content):
    if "import sys; sys.path.insert(0" not in content:
        lines = content.split('\n')
        # find where to insert (after imports like from __future__ import annotations)
        insert_idx = 0
        for i, line in enumerate(lines):
            if line.startswith('from __future__'):
                insert_idx = i + 1
                break
        
        insert_code = """
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))
"""
        lines.insert(insert_idx, insert_code.strip())
        return '\n'.join(lines)
    return content

def migrate_tests():
    for fname, svc in FILE_MAPPING.items():
        src = BACKEND_DIR / fname
        if src.exists():
            dest = BASE_DIR / svc / fname
            content = src.read_text()
            content = fix_imports(content)
            dest.write_text(content)
            print(f"Moved {fname} to {svc}")

def generate_missing_tests():
    used_services = set(FILE_MAPPING.values())
    missing = set(SERVICES) - used_services
    
    for svc in missing:
        safe_name = svc.replace('-', '_')
        content = f'''"""Test cho {svc}."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from check_framework import Context, run_single

SUITE = "{svc}"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for {svc}
    ctx.skip("{svc}", "Chưa có test cho {svc}")

if __name__ == "__main__":
    run_single(SUITE, run)
'''
        dest = BASE_DIR / svc / f"test_{safe_name}.py"
        dest.write_text(content)
        print(f"Generated missing test for {svc}")

def generate_run_all():
    content = '''#!/usr/bin/env python3
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
    print(f"\\nĐộ phủ route HTTP: {total - len(missing)}/{total}")
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
            print(f"\\n=== {svc} ({path.name}) ===")
            run_suite(ctx, getattr(mod, 'SUITE', svc), mod.run)
        except Exception as e:
            print(f"Lỗi khi chạy {path.name}: {e}")

    code = summarize(ctx)
    if not args.no_coverage and not wanted:
        print_coverage()
    return code

if __name__ == "__main__":
    sys.exit(main())
'''
    (BASE_DIR / 'run_all.py').write_text(content)
    # Also set executable permission
    os.chmod(BASE_DIR / 'run_all.py', 0o755)

def main():
    ensure_dirs()
    migrate_framework()
    migrate_tests()
    generate_missing_tests()
    generate_run_all()
    print("Done refactoring tests.")

if __name__ == '__main__':
    main()
