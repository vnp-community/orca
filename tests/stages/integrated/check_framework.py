"""Khung chạy kiểm tra tối giản: ghi nhận PASS/FAIL/SKIP, dọn dẹp, báo cáo."""
from __future__ import annotations

import sys
import traceback
from dataclasses import dataclass, field
from typing import Any, Callable

import requests

from env_config import Config, load_config
from orca_api_session import ApiSession, find_value, json_body


@dataclass
class Outcome:
    suite: str
    name: str
    status: str  # PASS | FAIL | SKIP
    detail: str = ""


def _matches(code: int, spec: Any) -> bool:
    if isinstance(spec, int):
        return code == spec
    if spec == "2xx":
        return 200 <= code < 300
    if spec == "4xx":
        return 400 <= code < 500
    # Gateway đã xử lý request (không crash/không 501), không phải lỗi xác thực.
    if spec == "handled":
        return code < 500 and code not in (401, 403, 405)
    if spec == "no5xx":
        return code < 500
    raise ValueError(f"spec không hợp lệ: {spec!r}")


@dataclass
class Context:
    cfg: Config
    anon: ApiSession
    admin: ApiSession | None = None
    user: ApiSession | None = None
    state: dict[str, Any] = field(default_factory=dict)
    results: list[Outcome] = field(default_factory=list)
    cleanups: list[tuple[str, Callable[[], None]]] = field(default_factory=list)
    suite: str = ""

    # ---- ghi nhận ------------------------------------------------------
    def _record(self, name: str, status: str, detail: str = "") -> bool:
        self.results.append(Outcome(self.suite, name, status, detail))
        mark = {"PASS": "  ok ", "FAIL": " FAIL", "SKIP": " skip"}[status]
        print(f"[{mark}] {self.suite}: {name}" + (f"  -> {detail}" if detail and status != "PASS" else ""))
        return status == "PASS"

    def expect(self, name: str, resp: requests.Response, *accepted: Any) -> bool:
        """PASS nếu status khớp một trong `accepted` (int | '2xx' | '4xx' | 'handled' | 'no5xx')."""
        accepted = accepted or ("2xx",)
        if any(_matches(resp.status_code, a) for a in accepted):
            return self._record(name, "PASS")
        snippet = (resp.text or "")[:200].replace("\n", " ")
        return self._record(name, "FAIL",
                            f"{resp.request.method if resp.request else ''} {resp.url} -> "
                            f"{resp.status_code} (mong đợi {list(accepted)}): {snippet}")

    def assert_true(self, name: str, cond: bool, detail: str = "") -> bool:
        return self._record(name, "PASS" if cond else "FAIL", "" if cond else detail)

    def skip(self, name: str, reason: str) -> None:
        self._record(name, "SKIP", reason)

    def cleanup(self, label: str, fn: Callable[[], None]) -> None:
        self.cleanups.append((label, fn))

    def need_admin(self) -> ApiSession | None:
        if self.admin is None or not self.admin.authenticated:
            self.skip("yêu cầu phiên đăng nhập", "không đăng nhập được (kiểm tra ORCA_ADMIN_*)")
            return None
        return self.admin

    def need_writes(self, name: str) -> bool:
        if not self.cfg.allow_writes:
            self.skip(name, "ORCA_ALLOW_WRITES=false")
            return False
        return True

    def run_cleanups(self) -> None:
        for label, fn in reversed(self.cleanups):
            try:
                fn()
            except Exception as exc:  # dọn dẹp không được làm hỏng báo cáo
                print(f"[warn] cleanup {label}: {exc}")
        self.cleanups.clear()

    # ---- tiện ích dữ liệu ---------------------------------------------
    def name(self, tag: str) -> str:
        import uuid
        return f"{self.cfg.prefix}-{tag}-{uuid.uuid4().hex[:8]}"


def build_context(login: bool = True) -> Context:
    cfg = load_config()
    ctx = Context(cfg=cfg, anon=ApiSession(cfg, "anon"))
    if login and cfg.has_admin_credentials:
        s = ApiSession(cfg, "admin")
        resp = s.login(cfg.admin_email, cfg.admin_password)
        if resp.status_code == 200 and s.authenticated:
            ctx.admin = s
        else:
            print(f"[warn] đăng nhập admin thất bại: {resp.status_code} {resp.text[:120]}")
    if login and cfg.user_email and cfg.user_password:
        u = ApiSession(cfg, "user")
        resp = u.login(cfg.user_email, cfg.user_password)
        if resp.status_code == 200 and u.authenticated:
            ctx.user = u
    return ctx


def run_suite(ctx: Context, name: str, fn: Callable[[Context], None]) -> None:
    ctx.suite = name
    try:
        fn(ctx)
    except Exception:  # lỗi lập trình/mạng trong suite -> FAIL có traceback
        ctx._record("suite crashed", "FAIL", traceback.format_exc(limit=3))
    finally:
        ctx.run_cleanups()


def summarize(ctx: Context) -> int:
    counts = {"PASS": 0, "FAIL": 0, "SKIP": 0}
    for r in ctx.results:
        counts[r.status] += 1
    print("\n" + "=" * 70)
    print(f"PASS={counts['PASS']}  FAIL={counts['FAIL']}  SKIP={counts['SKIP']}  (base={ctx.cfg.base_url})")
    for r in ctx.results:
        if r.status == "FAIL":
            print(f"  FAIL {r.suite}: {r.name}\n       {r.detail}")
    return 1 if counts["FAIL"] else 0


def run_single(suite_name: str, fn: Callable[[Context], None]) -> None:
    """Điểm vào `python api_xxx.py` — chạy một suite độc lập."""
    ctx = build_context()
    run_suite(ctx, suite_name, fn)
    sys.exit(summarize(ctx))


__all__ = ["Context", "build_context", "run_suite", "summarize", "run_single",
           "find_value", "json_body"]
