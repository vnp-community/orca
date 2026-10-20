"""Configuration from environment variables (TDD-TSAGENT-v2 section 10)."""
import os
import re
from dataclasses import dataclass, field

from .errors import ConfigError, InvalidFeature

LLM_PROVIDERS = ("none", "anthropic", "openai", "claude-cli", "fake")
LLM_STEPS = ("analysis", "design", "specification")
FEATURE_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")


def check_feature(feature):
    """The feature code becomes a folder name and part of node codes: only A-Z a-z 0-9 . _ -, at most 64 characters, no `..`."""
    if not isinstance(feature, str) or not FEATURE_RE.match(feature) or ".." in feature:
        raise InvalidFeature(f"invalid feature code: {feature!r} (only A-Z a-z 0-9 . _ -, at most 64 characters)")
    return feature


def _env(name, default=""):
    return os.environ.get(name, default)


def _int(name, default, minimum=0):
    raw = os.environ.get(name)
    if raw is None or raw == "":
        return default
    try:
        v = int(raw)
    except ValueError:
        raise ConfigError(f"{name} must be an integer, got {raw!r}") from None
    if v < minimum:
        raise ConfigError(f"{name} must be >= {minimum}, got {v}")
    return v


def _bool(name, default=False):
    raw = os.environ.get(name)
    if raw is None or raw == "":
        return default
    if raw.lower() in ("1", "true", "yes", "on"):
        return True
    if raw.lower() in ("0", "false", "no", "off"):
        return False
    raise ConfigError(f"{name} must be 1/0/true/false, got {raw!r}")


def _steps():
    raw = _env("TSAGENT_LLM_STEPS", ",".join(LLM_STEPS))
    steps = tuple(s.strip() for s in raw.split(",") if s.strip())
    bad = [s for s in steps if s not in LLM_STEPS]
    if bad:
        raise ConfigError(f"TSAGENT_LLM_STEPS has unsupported steps: {', '.join(bad)} (only {', '.join(LLM_STEPS)})")
    return steps


def _provider():
    """LLM mode; "" = not set (there is no default). The unset error is raised by `llm.check_llm`, not when Config is built."""
    p = _env("TSAGENT_LLM", "")
    if p and p not in LLM_PROVIDERS:
        raise ConfigError(f"TSAGENT_LLM is not supported: {p!r} (only {', '.join(LLM_PROVIDERS)})")
    return p


def _roots():
    raw = _env("TSAGENT_SOURCE_ROOTS", os.getcwd())
    return tuple(os.path.realpath(p) for p in raw.split(os.pathsep) if p)


@dataclass
class AutoApprovePolicy:
    """Auto-approval policy for LLM-generated nodes (TSO-020). Empty means off."""
    classes: tuple = ()
    min_confidence: float = 1.01

    @property
    def enabled(self):
        return bool(self.classes) and self.min_confidence <= 1.0

    @classmethod
    def parse(cls, raw):
        """Form: `classes=TestStep,Assertion;min_confidence=0.9`."""
        if not raw:
            return cls()
        classes, conf = (), 1.01
        for part in raw.split(";"):
            if not part.strip():
                continue
            if "=" not in part:
                raise ConfigError(f"TSAGENT_AUTO_APPROVE has a syntax error at {part!r} (form classes=A,B;min_confidence=0.9)")
            k, v = (x.strip() for x in part.split("=", 1))
            if k == "classes":
                classes = tuple(c.strip() for c in v.split(",") if c.strip())
            elif k == "min_confidence":
                try:
                    conf = float(v)
                except ValueError:
                    raise ConfigError(f"min_confidence must be a number, got {v!r}") from None
                if not 0 <= conf <= 1:
                    raise ConfigError("min_confidence must be in [0, 1]")
            else:
                raise ConfigError(f"TSAGENT_AUTO_APPROVE has no key {k!r}")
        return cls(classes, conf)


@dataclass
class Config:
    db: str = field(default_factory=lambda: _env("TSAGENT_DB", ".tsagent/tsagent.db"))
    specs_root: str = field(default_factory=lambda: _env("TSAGENT_SPECS_ROOT", "specs"))
    cache_dir: str = field(default_factory=lambda: _env("TSAGENT_CACHE_DIR", ".tsagent/cache"))
    snapshot_dir: str = field(default_factory=lambda: _env("TSAGENT_SNAPSHOT_DIR", ".tsagent/snapshots"))
    inbox_dir: str = field(default_factory=lambda: _env("TSAGENT_INBOX_DIR", ""))
    llm_provider: str = field(default_factory=_provider)
    llm_steps: tuple = field(default_factory=_steps)
    llm_caps: dict = field(default_factory=lambda: {
        "analysis": _int("TSAGENT_LLM_CAP_ANALYSIS", 200),
        "design": _int("TSAGENT_LLM_CAP_DESIGN", 100),
        "specification": _int("TSAGENT_LLM_CAP_SPEC", 300),
    })
    auto_approve: AutoApprovePolicy = field(default_factory=lambda: AutoApprovePolicy.parse(_env("TSAGENT_AUTO_APPROVE")))
    token: str = field(default_factory=lambda: _env("TSAGENT_TOKEN"))
    mcp_review: bool = field(default_factory=lambda: _bool("TSAGENT_MCP_REVIEW"))
    source_roots: tuple = field(default_factory=_roots)
    max_body_kb: int = field(default_factory=lambda: _int("TSAGENT_MAX_BODY_KB", 4096, 1))
    busy_timeout_ms: int = field(default_factory=lambda: _int("TSAGENT_BUSY_TIMEOUT_MS", 5000))
    claude_model: str = field(default_factory=lambda: _env("TSAGENT_CLAUDE_MODEL"))
    claude_bin: str = field(default_factory=lambda: _env("CLAUDE_BIN"))
    anthropic_model: str = field(default_factory=lambda: _env("ANTHROPIC_MODEL"))
    anthropic_base_url: str = field(default_factory=lambda: _env("ANTHROPIC_BASE_URL"))
    openai_model: str = field(default_factory=lambda: _env("OPENAI_MODEL"))
    openai_base_url: str = field(default_factory=lambda: _env("OPENAI_BASE_URL"))
    llm_parallel: int = field(default_factory=lambda: _int("TSAGENT_LLM_PARALLEL", 1, 1))
    llm_group_retries: int = field(default_factory=lambda: _int("TSAGENT_LLM_GROUP_RETRIES", 2, 0))
    context_depth: int = field(default_factory=lambda: _int("TSAGENT_CONTEXT_DEPTH", 2, 1))
    context_max_chars: int = field(default_factory=lambda: _int("TSAGENT_CONTEXT_MAX_CHARS", 24000, 1000))

    def inbox(self):
        return self.inbox_dir or os.path.join(self.cache_dir, "inbox")

    def llm_enabled_for(self, step):
        return self.llm_provider not in ("", "none") and step in self.llm_steps
