"""OpenAI Chat Completions (or a compatible server via OPENAI_BASE_URL)."""
from .http_base import HttpProvider


class OpenAIProvider(HttpProvider):
    name = "openai"
    key_env = "OPENAI_API_KEY"
    default_base = "https://api.openai.com"
    path = "/v1/chat/completions"

    def headers(self):
        return {"authorization": f"Bearer {self._key}", "content-type": "application/json"}

    def body(self, system, user):
        return {"model": self.model, "messages": [{"role": "system", "content": system}, {"role": "user", "content": user}],
                "response_format": {"type": "json_object"}}

    def parse(self, data):
        return data["choices"][0]["message"]["content"]
