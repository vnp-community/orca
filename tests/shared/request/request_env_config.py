"""Cấu hình cho kiểm tra T2 của luồng Request: dùng lại khung tests/backend, thêm vài biến riêng."""
from __future__ import annotations

import os
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
BACKEND = HERE.parent / "backend"
if str(BACKEND) not in sys.path:
    sys.path.insert(0, str(BACKEND))

# Mười một loại Request, thứ tự như README v6 mục 3.4.
REQUEST_TYPES = [
    "change_request", "bug", "hotfix", "task", "spike", "question",
    "refactor", "security", "performance", "docs", "ops_request",
]


def project_id() -> str:
    """Project có dev server (hoặc stub agent) để phân loại; bắt buộc để chạy kịch bản tạo Request."""
    return os.environ.get("ORCA_REQUEST_PROJECT_ID", "")


def real_ai() -> bool:
    """T3: dùng AI thật thay cho stub; không đánh giá chất lượng quyết định của AI."""
    return os.environ.get("ORCA_REQUEST_E2E_REAL_AI", "false").strip().lower() in ("1", "true", "yes", "on")


def marker(typ: str, size: str) -> str:
    """Dấu hiệu stub agent đọc để phân loại (backend-go/services/request-service/e2e/stubs). AI thật bỏ qua nó."""
    urgency = " urgency=urgent" if typ == "hotfix" else ""
    return f"[e2e:type={typ} size={size}{urgency}]"
