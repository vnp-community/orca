#!/usr/bin/env python3
"""Lấy (tạo) Personal Access Token MCP bằng tài khoản admin của môi trường triển khai.

    python get_mcp_token.py                       # tạo PAT read+write+exec, ghi ORCA_MCP_TOKEN vào tests/mcp/.env
    python get_mcp_token.py --scopes read --days 1
    python get_mcp_token.py --print               # in nguyên token ra stdout (để: export ORCA_MCP_TOKEN=$(...))
    python get_mcp_token.py --list                # liệt kê PAT hiện có
    python get_mcp_token.py --revoke <id>         # thu hồi một PAT

Cấu hình đọc như các script kiểm tra: tests/mcp/.env, rồi deploy/dev/.env (BOOTSTRAP_ADMIN_*,
PUBLIC_BASE_URL). Token chỉ hiện MỘT lần ở phản hồi tạo; mặc định script ghi vào .env (đã nằm trong
.gitignore, đặt quyền 600) và chỉ in dạng che. Chỉ tạo scope orca:read/write/exec, không tạo orca:admin.
"""
from __future__ import annotations

import argparse
import os
import stat
import sys
from pathlib import Path

from mcp_check_framework import RestSession
from mcp_env_config import HERE, load_config

SCOPES = {"read": "orca:read", "write": "orca:write", "exec": "orca:exec"}


def _mask(secret: str) -> str:
    return secret[:6] + "…" + secret[-4:] if len(secret) > 12 else "***"


def _api_error(resp) -> str:
    try:
        body = resp.json()
    except ValueError:
        return resp.text[:200]
    return str(body.get("error") or body.get("message") or body)[:200]


def write_env_value(path: Path, key: str, value: str) -> None:
    """Đặt key=value trong file .env, giữ nguyên các dòng khác; tạo file nếu chưa có."""
    lines = path.read_text(encoding="utf-8").splitlines() if path.is_file() else []
    out, done = [], False
    for line in lines:
        if line.strip().startswith(f"{key}=") or line.strip().startswith(f"export {key}="):
            if not done:
                out.append(f"{key}={value}")
                done = True
            continue  # bỏ dòng trùng lặp
        out.append(line)
    if not done:
        out.append(f"{key}={value}")
    path.write_text("\n".join(out) + "\n", encoding="utf-8")
    path.chmod(stat.S_IRUSR | stat.S_IWUSR)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--scopes", default="read,write,exec", help="danh sách trong read,write,exec (mặc định: cả ba)")
    ap.add_argument("--days", type=int, default=7, help="thời hạn (ngày); vượt mức admin cho phép sẽ bị từ chối")
    ap.add_argument("--name", default="", help="tên token (mặc định: orca-mcp-selftest-<host>)")
    ap.add_argument("--env-file", default=str(HERE / ".env"), help="file .env sẽ ghi ORCA_MCP_TOKEN")
    ap.add_argument("--no-write", action="store_true", help="không ghi vào file .env")
    ap.add_argument("--print", dest="print_token", action="store_true", help="in nguyên token ra stdout")
    ap.add_argument("--list", action="store_true", help="liệt kê PAT rồi thoát")
    ap.add_argument("--revoke", metavar="ID", help="thu hồi PAT theo id rồi thoát")
    args = ap.parse_args()

    cfg = load_config()
    if not cfg.has_admin_credentials:
        print("Thiếu tài khoản: đặt ORCA_ADMIN_EMAIL/PASSWORD trong tests/mcp/.env "
              "hoặc BOOTSTRAP_ADMIN_EMAIL/PASSWORD trong deploy/dev/.env", file=sys.stderr)
        return 2
    session = RestSession(cfg)
    resp = session.login(cfg.admin_email, cfg.admin_password)
    if resp.status_code != 200 or not session.authenticated:
        print(f"Đăng nhập {cfg.base_url} thất bại ({cfg.admin_email}): HTTP {resp.status_code} {_api_error(resp)}",
              file=sys.stderr)
        return 1

    if args.list:
        r = session.request("GET", "/v1/auth/mcp-tokens")
        if r.status_code != 200:
            print(f"HTTP {r.status_code}: {_api_error(r)}", file=sys.stderr)
            return 1
        for t in r.json().get("tokens") or []:
            print(f"{t.get('id')}  {str(t.get('status') or '?'):8s}  hết hạn {t.get('expiresAt') or '?'}  {','.join(t.get('scopes') or [])}  {t.get('name') or ''}")
        return 0
    if args.revoke is not None:
        if not args.revoke.strip():
            print("--revoke cần một id (xem --list)", file=sys.stderr)
            return 2
        r = session.request("DELETE", f"/v1/auth/mcp-tokens/{args.revoke}")
        print("đã thu hồi" if r.status_code == 204 else f"HTTP {r.status_code}: {_api_error(r)}")
        return 0 if r.status_code == 204 else 1

    wanted = [s.strip() for s in args.scopes.split(",") if s.strip()]
    bad = [s for s in wanted if s not in SCOPES]
    if bad or not wanted:
        print(f"scope không hợp lệ: {bad or args.scopes!r}; chọn trong {','.join(SCOPES)}", file=sys.stderr)
        return 2
    name = args.name or f"orca-mcp-selftest-{os.uname().nodename}"
    resp = session.request("POST", "/v1/auth/mcp-tokens", json_body={
        "name": name, "scopes": [SCOPES[s] for s in wanted], "expires_in_days": args.days})
    if resp.status_code != 201:
        err = _api_error(resp)
        hint = ""
        if "MCP_DISABLED" in err or resp.status_code == 404:
            hint = "\nMCP đang tắt trên gateway: cần MCP_ENABLED=true (kèm MCP_PUBLIC_BASE_URL) trong deploy/dev/.env rồi deploy lại."
        elif "TOKEN_TOO_LONG" in err:
            hint = "\nGiảm --days (admin đặt thời hạn tối đa ở Settings → MCP)."
        elif "TOKEN_LIMIT" in err:
            hint = "\nĐã đủ số PAT đang hoạt động; thu hồi bớt bằng --list / --revoke."
        print(f"Tạo token thất bại: HTTP {resp.status_code} {err}{hint}", file=sys.stderr)
        return 1
    body = resp.json()
    secret, token = body.get("secret", ""), body.get("token") or {}
    if not secret:
        print("Phản hồi không có secret", file=sys.stderr)
        return 1

    print(f"Đã tạo PAT {token.get('id')}  scope={','.join(token.get('scopes') or [])}  hết hạn {token.get('expiresAt')}")
    if not args.no_write:
        env_path = Path(args.env_file)
        write_env_value(env_path, "ORCA_MCP_TOKEN", secret)
        print(f"Đã ghi ORCA_MCP_TOKEN vào {env_path} (quyền 600)")
    if args.print_token:
        print(secret)
    else:
        print(f"Token (che): {_mask(secret)}   — dùng --print để in nguyên giá trị")
    print(f"Thu hồi khi không dùng nữa: python get_mcp_token.py --revoke {token.get('id')}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
