"""Runtime configuration.

Precedence (highest first): explicit overrides (CLI flags) > environment
``TDD_AGENT__SECTION__KEY`` > ``tdd-agent.toml`` > defaults.  Invalid
configuration fails early with :class:`ConfigError`; nothing silently falls back
to another provider or store.
"""
from __future__ import annotations

import copy
import json
import os
import tomllib
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from .errors import ConfigError

ENV_PREFIX = "TDD_AGENT__"
CONFIG_FILENAME = "tdd-agent.toml"
SECRET_KEYS = ("token", "key", "secret", "password")

GATE_ROLES = ("BA", "SA/Dev", "SA", "BA/owner")
ACTOR_KINDS = ("human", "coding_agent")
LOG_PAYLOAD_MODES = ("none", "hash", "full")
LLM_ROLES = ("author", "critic", "extractor")
RENDER_STYLES = ("standard", "kg")
APPROVAL_MODES = ("auto", "manual")
APPROVABLE_GATES = ("G1", "G2", "G5", "G7")
TESTSPEC_CONTEXT_MODES = ("none", "plan", "solution")  # thêm ngữ cảnh cho test specs: không / Plan (OpenSpec change) / tài liệu Solution
TESTSPEC_LLM_MODES = ("", "claude-cli", "anthropic", "openai", "none")  # "" = chưa đặt: run sinh test specs dừng với LLM_NOT_CONFIGURED
FINGERPRINT_EXCLUDE = ("testspec",)  # đọc lúc chạy; phần ảnh hưởng kết quả đi vào băm của cổng GT, không làm mọi cổng stale

DEFAULTS: dict[str, Any] = {
    "llm": {
        "default": "claude_code",
        "require_distinct_critic": False,
        "roles": {"author": "", "critic": "", "extractor": ""},
        "budget": {"max_calls_per_run": 0, "max_wall_seconds": 0, "max_runs_per_day": 0},
        "claude_code": {
            "command": "claude",
            "model": "claude-sonnet-5-5",
            "effort": "medium",
            "timeout_seconds": 900,
            "max_attempts": 2,
            "system_prompt": "You are a structured-output function inside an automated pipeline. Reply only with what the caller's schema requests.",
        },
        "codex_cli": {
            "command": "codex",
            "model": "",
            "isolate": True,       # ignore the user's own Codex configuration and rules (their default model, hooks), so a run does not depend on one person's setup
            "timeout_seconds": 900,
            "max_attempts": 2,
        },
    },
    "store": {
        "knowledge": "sqlite:.tdd-agent/knowledge.db",
        "operational": "sqlite:.tdd-agent/operational.db",
    },
    "retrieval": {"default_budget": 5000, "candidates": False, "lexical": False},  # lexical: seed proposals come from the full-text index of the store; verification stays mandatory
    "obs": {
        "log_level": "INFO",
        "log_file": ".tdd-agent/logs/tdd-agent.jsonl",
        "log_stderr": False,
        "log_payloads": "none",
        "redact_patterns": [],
        "otlp_endpoint": "",
    },
    "mcp": {
        "transport": "stdio",
        "host": "127.0.0.1",
        "port": 8765,
        "token_env": "TDD_AGENT_MCP_TOKEN",
        "write": False,
        "approvals": False,
        "max_result_chars": 60000,
        "client_actor": "",
    },
    "approvals": {"mode": "", "required": [], "require_review_receipt": True, "actors": []},
    "paths": {"repo_root": ".", "work_dir": ".tdd-agent"},
    "openspec": {"max_revisions": 2, "language": ""},
    "change": {"propose_missing": ""},   # fill what a CR left out (acceptance, the element that realizes a requirement, where the code is) with model proposals marked as such; "" = yes in automatic approval mode
    "acceptance": {"repetitions": 3},
    "project": {"name": "", "owner": "", "tdd_id": "", "docs_dir": "docs/tdd", "prose": True, "prose_limit": 40, "sources": {"urd": "", "srs": "", "sad": ""}, "id_classes": {}},
    "render": {"style": "standard", "format": "md", "language": "vi", "source_detail": "section"},
    "testspec": {"enabled": True, "llm": "", "parallel": 4, "group_retries": 2, "specs_dir": "docs/testspec",
                 "kg_mcp": "", "kg_mcp_token_env": "TDD_AGENT_MCP_TOKEN", "context_mode": "none", "context_path": ""},
}


@dataclass(frozen=True)
class ActorConfig:
    id: str
    kind: str
    roles: tuple[str, ...]


@dataclass
class Config:
    data: dict[str, Any]
    base_dir: Path
    source: dict[str, str] = field(default_factory=dict)

    def section(self, name: str) -> dict[str, Any]:
        return self.data[name]

    def approval_mode(self) -> str:
        """``auto`` unless the project asked for approvals: an explicit ``approvals.mode`` wins, otherwise declaring approvers means manual."""
        cfg = self.data["approvals"]
        return cfg["mode"] or ("manual" if cfg["actors"] else "auto")

    def proposes_missing(self) -> bool:
        """Whether the change flow fills in what a CR left out (marked as proposed): by default exactly when nobody is there to ask, i.e. in automatic mode."""
        value = self.data["change"]["propose_missing"]
        return self.approval_mode() == "auto" if value == "" else bool(value)

    def get(self, dotted: str, default: Any = None) -> Any:
        node: Any = self.data
        for part in dotted.split("."):
            if not isinstance(node, dict) or part not in node:
                return default
            node = node[part]
        return node

    def path(self, dotted: str) -> Path:
        value = self.get(dotted)
        if value is None:
            raise ConfigError(f"missing path setting {dotted}")
        p = Path(str(value))
        return p if p.is_absolute() else (self.base_dir / p).resolve()

    @property
    def repo_root(self) -> Path:
        return self.path("paths.repo_root")

    @property
    def work_dir(self) -> Path:
        return self.path("paths.work_dir")

    def store_url(self, kind: str) -> str:
        url = str(self.get(f"store.{kind}"))
        if url.startswith("sqlite:") and url != "sqlite::memory:":
            p = Path(url[len("sqlite:"):])
            if not p.is_absolute():
                p = (self.base_dir / p).resolve()
            return "sqlite:" + str(p)
        return url

    def actors(self) -> list[ActorConfig]:
        return [ActorConfig(a["id"], a["kind"], tuple(a["roles"])) for a in self.data["approvals"]["actors"]]

    def redacted(self) -> dict[str, Any]:
        def walk(node: Any, key: str = "") -> Any:
            if isinstance(node, dict):
                return {k: walk(v, k) for k, v in node.items()}
            if isinstance(node, list):
                return [walk(v, key) for v in node]
            if isinstance(node, str) and node and any(s in key.lower() for s in SECRET_KEYS) and not key.lower().endswith("_env"):
                return "***"
            return node
        return walk(self.data)

    def fingerprint(self, sections: tuple[str, ...] | None = None) -> str:
        """Hash of the effective configuration, or only of the named ``sections`` (a gate depends on the sections it reads, not on who may approve)."""
        from hashlib import sha256
        data = {k: v for k, v in self.redacted().items() if k not in FINGERPRINT_EXCLUDE}
        if sections is not None:
            data = {k: data[k] for k in sections if k in data}
        return sha256(json.dumps(data, sort_keys=True).encode()).hexdigest()


def _merge(base: dict, extra: dict, where: str, source: dict[str, str], label: str) -> dict:
    for key, value in extra.items():
        if key not in base:
            raise ConfigError(f"unknown configuration key {where}{key}", code="CONFIG_UNKNOWN_KEY")
        if isinstance(base[key], dict):
            if not isinstance(value, dict):
                raise ConfigError(f"{where}{key} must be a table")
            _merge(base[key], value, f"{where}{key}.", source, label)
        else:
            base[key] = value
            source[f"{where}{key}"] = label
    return base


def _coerce_env(raw: str, current: Any) -> Any:
    if isinstance(current, bool):
        if raw.lower() in ("1", "true", "yes", "on"): return True
        if raw.lower() in ("0", "false", "no", "off"): return False
        raise ConfigError(f"expected boolean, got {raw!r}")
    if isinstance(current, int):
        try: return int(raw)
        except ValueError as exc: raise ConfigError(f"expected integer, got {raw!r}") from exc
    if isinstance(current, list):
        return json.loads(raw)
    return raw


def _validate(data: dict) -> None:
    llm = data["llm"]
    for role, name in llm["roles"].items():
        if role not in LLM_ROLES:
            raise ConfigError(f"unknown llm role {role}", code="CONFIG_UNKNOWN_ROLE")
    for key in ("max_calls_per_run", "max_wall_seconds", "max_runs_per_day"):
        if not isinstance(llm["budget"][key], int) or llm["budget"][key] < 0:
            raise ConfigError(f"llm.budget.{key} must be a non-negative integer (0 means unlimited)")
    for provider in ("claude_code", "codex_cli"):
        attempts = llm[provider]["max_attempts"]
        if not isinstance(attempts, int) or not 1 <= attempts <= 3:
            raise ConfigError(f"llm.{provider}.max_attempts must be 1..3")
        if not isinstance(llm[provider]["timeout_seconds"], int) or llm[provider]["timeout_seconds"] < 1:
            raise ConfigError(f"llm.{provider}.timeout_seconds must be positive")
    if data["obs"]["log_payloads"] not in LOG_PAYLOAD_MODES:
        raise ConfigError(f"obs.log_payloads must be one of {LOG_PAYLOAD_MODES}")
    if data["mcp"]["transport"] not in ("stdio", "http"):
        raise ConfigError("mcp.transport must be stdio or http")
    if data["render"]["style"] not in RENDER_STYLES:
        raise ConfigError(f"render.style must be one of {RENDER_STYLES}", code="CONFIG_RENDER_STYLE")
    if data["render"]["language"] not in ("vi", "en"):
        raise ConfigError("render.language must be vi or en", code="CONFIG_RENDER_LANGUAGE")
    if data["render"]["source_detail"] not in ("section", "line"):
        raise ConfigError("render.source_detail must be section or line", code="CONFIG_RENDER_SOURCE_DETAIL")
    if int(data["openspec"]["max_revisions"]) < 0:
        raise ConfigError("openspec.max_revisions must be >= 0", code="CONFIG_OPENSPEC")
    ts = data["testspec"]
    if ts["llm"] not in TESTSPEC_LLM_MODES:
        raise ConfigError(f"testspec.llm must be one of {TESTSPEC_LLM_MODES[1:]}", code="CONFIG_INVALID")
    if not isinstance(ts["parallel"], int) or isinstance(ts["parallel"], bool) or ts["parallel"] < 1:
        raise ConfigError("testspec.parallel must be an integer >= 1", code="CONFIG_INVALID")
    if not isinstance(ts["group_retries"], int) or isinstance(ts["group_retries"], bool) or ts["group_retries"] < 0:
        raise ConfigError("testspec.group_retries must be an integer >= 0", code="CONFIG_INVALID")
    if not isinstance(ts["specs_dir"], str) or not ts["specs_dir"].strip() or Path(ts["specs_dir"]).is_absolute() or ".." in Path(ts["specs_dir"]).parts:
        raise ConfigError("testspec.specs_dir must be a relative path inside the repository", code="CONFIG_INVALID")
    if ts["kg_mcp"] and not str(ts["kg_mcp"]).startswith(("http://", "https://")):
        raise ConfigError("testspec.kg_mcp must be empty (local stdio MCP server) or an http(s) URL of a tdd-agent MCP server", code="CONFIG_INVALID")
    if not str(ts["kg_mcp_token_env"]).strip():
        raise ConfigError("testspec.kg_mcp_token_env must name an environment variable", code="CONFIG_INVALID")
    if ts["context_mode"] not in TESTSPEC_CONTEXT_MODES:
        raise ConfigError(f"testspec.context_mode must be one of {TESTSPEC_CONTEXT_MODES}", code="CONFIG_INVALID")
    cp = ts["context_path"]
    if not isinstance(cp, str) or (cp.strip() and (Path(cp).is_absolute() or ".." in Path(cp).parts)):
        raise ConfigError("testspec.context_path must be empty or a relative path inside the repository", code="CONFIG_INVALID")
    if data["acceptance"]["repetitions"] < 1:
        raise ConfigError("acceptance.repetitions must be >= 1")
    if data["approvals"]["mode"] not in ("", *APPROVAL_MODES):
        raise ConfigError(f"approvals.mode must be one of {APPROVAL_MODES}", code="CONFIG_APPROVAL_MODE")
    bad = [g for g in data["approvals"]["required"] if g not in APPROVABLE_GATES]
    if bad:
        raise ConfigError(f"approvals.required names gates that have no approval step: {bad} (approvable gates: {APPROVABLE_GATES})", code="CONFIG_APPROVAL_REQUIRED")
    seen: set[str] = set()
    for actor in data["approvals"]["actors"]:
        if not isinstance(actor, dict) or set(actor) - {"id", "kind", "roles"} or not {"id", "kind", "roles"} <= set(actor):
            raise ConfigError("each approvals.actors entry needs exactly id, kind, roles")
        if actor["kind"] not in ACTOR_KINDS:
            raise ConfigError(f"approver kind must be one of {ACTOR_KINDS}")
        bad = [r for r in actor["roles"] if r not in GATE_ROLES]
        if bad:
            raise ConfigError(f"unknown approver roles {bad}")
        if actor["id"] in seen:
            raise ConfigError(f"duplicate approver {actor['id']}")
        seen.add(actor["id"])
    if data["mcp"]["approvals"] and not data["mcp"]["client_actor"] and data["mcp"]["transport"] == "stdio":
        raise ConfigError("mcp.approvals over stdio requires mcp.client_actor naming an allowlisted approver")
    if data["mcp"]["approvals"] and data["mcp"]["client_actor"] not in {a["id"] for a in data["approvals"]["actors"]}:
        raise ConfigError("mcp.client_actor must be present in approvals.actors")


def load_config(path: str | Path | None = None, overrides: dict[str, Any] | None = None, env: dict[str, str] | None = None, base_dir: Path | None = None) -> Config:
    env = os.environ if env is None else env
    data = copy.deepcopy(DEFAULTS)
    source: dict[str, str] = {}
    fixed_base = Path(base_dir).resolve() if base_dir else None
    base_dir = fixed_base or Path.cwd()
    candidate = Path(path) if path else Path.cwd() / CONFIG_FILENAME
    if path and not candidate.is_file():
        raise ConfigError(f"configuration file not found: {candidate}", code="CONFIG_FILE_MISSING")
    if candidate.is_file():
        try:
            file_data = tomllib.loads(candidate.read_text(encoding="utf-8"))
        except (tomllib.TOMLDecodeError, OSError) as exc:
            raise ConfigError(f"cannot read {candidate}: {exc}") from exc
        base_dir = fixed_base or candidate.parent.resolve()
        _merge(data, file_data, "", source, "file")
    for name, raw in sorted(env.items()):
        if not name.startswith(ENV_PREFIX):
            continue
        parts = [p.lower() for p in name[len(ENV_PREFIX):].split("__") if p]
        node = data
        for part in parts[:-1]:
            if part not in node or not isinstance(node[part], dict):
                raise ConfigError(f"unknown configuration key from environment: {name}", code="CONFIG_UNKNOWN_KEY")
            node = node[part]
        if not parts or parts[-1] not in node:
            raise ConfigError(f"unknown configuration key from environment: {name}", code="CONFIG_UNKNOWN_KEY")
        node[parts[-1]] = _coerce_env(raw, node[parts[-1]])
        source[".".join(parts)] = "env"
    if overrides:
        _merge(data, overrides, "", source, "flag")
    _validate(data)
    return Config(data=data, base_dir=base_dir, source=source)
