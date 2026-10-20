"""T4.1: HTTP providers with a fake transport: error mapping, retry, Retry-After, breaker, keys never in logs."""
import logging

import httpx
import pytest

from tsagent.errors import CircuitOpen, LLMPermanent, LLMTransient, LLMUnknownOutcome
from tsagent.llm.policy import CircuitBreaker, RetryPolicy
from tsagent.llm.providers.anthropic import AnthropicProvider
from tsagent.llm.providers.openai import OpenAIProvider

pytestmark = pytest.mark.unit
KEY = "sk-ant-test-SECRET-123456789"


def ok_anthropic(text='{"a":1}'):
    return httpx.Response(200, json={"content": [{"type": "text", "text": text}]})


def provider(handler, cls=AnthropicProvider, sleeps=None, **kw):
    sleeps = sleeps if sleeps is not None else []
    return cls("m-1", api_key=KEY, transport=httpx.MockTransport(handler), sleep=sleeps.append,
               policy=kw.pop("policy", RetryPolicy(max_attempts=3, base_s=0.01)), **kw)


def seq(*responses):
    calls = []

    def handler(req):
        calls.append(req)
        r = responses[min(len(calls) - 1, len(responses) - 1)]
        if isinstance(r, Exception):
            raise r
        return r
    handler.calls = calls
    return handler


def test_ok_and_request_shape():
    h = seq(ok_anthropic())
    assert provider(h).send("sys", "hi") == '{"a":1}'
    req = h.calls[0]
    assert req.headers["x-api-key"] == KEY and req.url.path == "/v1/messages"
    assert b'"system": "sys"' in req.content or b'"system":"sys"' in req.content


def test_openai_shape():
    h = seq(httpx.Response(200, json={"choices": [{"message": {"content": "{}"}}]}))
    assert provider(h, OpenAIProvider).send("s", "u") == "{}"
    assert h.calls[0].headers["authorization"] == f"Bearer {KEY}"


def test_429_respects_retry_after_then_succeeds():
    sleeps = []
    h = seq(httpx.Response(429, headers={"retry-after": "2"}), ok_anthropic())
    assert provider(h, sleeps=sleeps).send("s", "u")
    assert sleeps == [2.0] and len(h.calls) == 2


def test_5xx_and_network_errors_are_transient_and_retried():
    h = seq(httpx.Response(503), httpx.ConnectError("x"), ok_anthropic())
    assert provider(h).send("s", "u") and len(h.calls) == 3


def test_retries_exhausted():
    h = seq(httpx.Response(500))
    with pytest.raises(LLMTransient):
        provider(h).send("s", "u")
    assert len(h.calls) == 3


def test_read_timeout_is_unknown_outcome():
    h = seq(httpx.ReadTimeout("t"), httpx.ReadTimeout("t"), httpx.ReadTimeout("t"))
    with pytest.raises(LLMUnknownOutcome):
        provider(h).send("s", "u")


@pytest.mark.parametrize("status,fatal", [(400, False), (404, False), (401, True), (403, True)])
def test_4xx_permanent_not_retried(status, fatal):
    h = seq(httpx.Response(status, text="bad"))
    with pytest.raises(LLMPermanent) as e:
        provider(h).send("s", "u")
    assert len(h.calls) == 1 and e.value.fatal is fatal


def test_breaker_opens_then_half_open():
    t = [0.0]
    br = CircuitBreaker(min_calls=3, failure_ratio=0.5, open_s=10, clock=lambda: t[0])
    h = seq(httpx.Response(400))
    p = provider(h, breaker=br)
    for _ in range(3):
        with pytest.raises(LLMPermanent):
            p.send("s", "u")
    with pytest.raises(CircuitOpen):
        p.send("s", "u")
    assert len(h.calls) == 3
    t[0] = 11
    p2 = provider(seq(ok_anthropic()), breaker=br)
    assert p2.send("s", "u") and br.state == "closed"


def test_missing_key_is_fatal(monkeypatch):
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    with pytest.raises(LLMPermanent) as e:
        AnthropicProvider("m")
    assert e.value.fatal


def test_key_never_logged(caplog):
    caplog.set_level(logging.DEBUG)
    h = seq(httpx.Response(500), ok_anthropic())
    p = provider(h)
    p.send("s", "u")
    assert KEY not in caplog.text and KEY not in repr(p)
