"""Anthropic Messages API."""
from .http_base import HttpProvider


class AnthropicProvider(HttpProvider):
    name = "anthropic"
    key_env = "ANTHROPIC_API_KEY"
    default_base = "https://api.anthropic.com"
    path = "/v1/messages"
    max_tokens = 16000

    def headers(self):
        return {"x-api-key": self._key, "anthropic-version": "2023-06-01", "content-type": "application/json"}

    def body(self, system, user):
        return {"model": self.model, "max_tokens": self.max_tokens, "system": system,
                "messages": [{"role": "user", "content": user}]}

    def parse(self, data):
        return "".join(b.get("text", "") for b in data["content"] if b.get("type") == "text")
