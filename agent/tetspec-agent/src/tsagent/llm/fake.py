"""Scripted fake LLM for tests (T4.4). Enabled only by TSAGENT_LLM=fake while pytest runs or with TSAGENT_ALLOW_FAKE=1."""
import json
import time

from .providers.http_base import Provider


class FakeLLM(Provider):
    """`script` is one of:
    - dict/list: the same JSON for every call;
    - callable(system, user) -> dict | str | Exception;
    - a list of the above (used in order, the last item repeats) when `sequence=True`.
    A returned (not raised) exception is raised; a str is returned verbatim (to test broken JSON)."""

    name = "fake"
    current = None  # set by the `fake_llm` fixture; make_provider uses it when TSAGENT_LLM=fake

    def __init__(self, script=None, sequence=False, delay_s=0.0, model="fake-1"):
        self.script = script if script is not None else {}
        self.sequence = sequence
        self.delay_s = delay_s
        self.model = model
        self.calls = []

    @property
    def tag(self):
        return f"fake:{self.model}"

    def _pick(self):
        if not self.sequence:
            return self.script
        i = min(len(self.calls) - 1, len(self.script) - 1)
        return self.script[i]

    def send(self, system, user):
        self.calls.append({"system": system, "user": user})
        if self.delay_s:
            time.sleep(self.delay_s)
        item = self._pick()
        if callable(item) and not isinstance(item, type):
            item = item(system, user)
        if isinstance(item, BaseException):
            raise item
        if isinstance(item, str):
            return item
        return json.dumps(item, ensure_ascii=False)
