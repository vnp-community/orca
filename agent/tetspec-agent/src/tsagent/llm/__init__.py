"""MOD-LLM: providers, retry/breaker, complete_json, cache, fake LLM."""
import os

from ..errors import ConfigError, LLMKeyMissing, LLMModelMissing, LLMNotConfigured
from .client import LLMClient, wrap_data

MODES = ("claude-cli", "anthropic", "openai", "none")


def check_llm(cfg):
    """Check the LLM configuration without network calls: a mode is set; API providers have a key and a model. Returns {"llm": mode}."""
    p = cfg.llm_provider
    if not p:
        raise LLMNotConfigured("LLM_NOT_CONFIGURED: no LLM mode chosen; set TSAGENT_LLM (or [testspec] llm) = "
                               + " | ".join(MODES))
    api = {"anthropic": ("ANTHROPIC_API_KEY", cfg.anthropic_model, "ANTHROPIC_MODEL"),
           "openai": ("OPENAI_API_KEY", cfg.openai_model, "OPENAI_MODEL")}.get(p)
    if api:
        key_env, model, model_env = api
        if not os.environ.get(key_env):
            raise LLMKeyMissing(f"LLM_KEY_MISSING: mode {p} needs the environment variable {key_env}")
        if not model:
            raise LLMModelMissing(f"LLM_MODEL_MISSING: mode {p} needs {model_env}")
    return {"llm": p}


def make_provider(cfg):
    check_llm(cfg)
    p = cfg.llm_provider
    if p == "none":
        return None
    if p == "fake":
        from .fake import FakeLLM
        if not (os.environ.get("PYTEST_CURRENT_TEST") or os.environ.get("TSAGENT_ALLOW_FAKE") == "1"):
            raise ConfigError("TSAGENT_LLM=fake is for tests only")
        return FakeLLM.current or FakeLLM()
    if p == "anthropic":
        from .providers.anthropic import AnthropicProvider
        return AnthropicProvider(cfg.anthropic_model, cfg.anthropic_base_url or None)
    if p == "openai":
        from .providers.openai import OpenAIProvider
        return OpenAIProvider(cfg.openai_model, cfg.openai_base_url or None)
    if p == "claude-cli":
        from .providers.claude_cli import ClaudeCliProvider
        return ClaudeCliProvider(cfg.claude_model, cfg.claude_bin)
    raise ConfigError(f"unsupported LLM provider: {p}")


def make_client(cfg, provider=None):
    return LLMClient(cfg, provider if provider is not None else make_provider(cfg))


__all__ = ["MODES", "LLMClient", "LLMKeyMissing", "LLMModelMissing", "LLMNotConfigured", "check_llm", "make_client", "make_provider",
           "wrap_data"]
