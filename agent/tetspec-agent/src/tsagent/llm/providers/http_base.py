"""Common base of HTTP providers: connection pool, split timeouts, error mapping, retry, breaker."""
import os

import httpx

from ...errors import LLMPermanent, LLMTransient, LLMUnknownOutcome
from ..policy import CircuitBreaker, RetryPolicy, call_with_retry


class Provider:
    """Interface: `send(system, user) -> text`; `tag` identifies it for the cache (provider + model)."""
    tag = "base"

    def send(self, system, user):  # pragma: no cover - interface
        raise NotImplementedError


def _retry_after(resp):
    raw = resp.headers.get("retry-after")
    try:
        return float(raw) if raw else None
    except ValueError:
        return None


def map_status(resp):
    """HTTP status -> error following the ErrorMapping. None for 2xx."""
    s = resp.status_code
    if 200 <= s < 300:
        return None
    body = resp.text[:300]
    if s in (401, 403):
        return LLMPermanent(f"HTTP {s}: wrong API key or not authorised", fatal=True)
    if s == 429 or s >= 500 or s == 408:
        e = LLMTransient(f"HTTP {s}: {body}")
        e.retry_after = _retry_after(resp)
        return e
    return LLMPermanent(f"HTTP {s}: {body}")


class HttpProvider(Provider):
    key_env = ""
    default_base = ""

    def __init__(self, model, base_url=None, api_key=None, *, connect_s=10.0, read_s=120.0, pool_s=10.0,
                 max_conn=8, policy=None, breaker=None, transport=None, sleep=None):
        self.model = model
        self.base_url = (base_url or self.default_base).rstrip("/")
        self._key = api_key if api_key is not None else os.environ.get(self.key_env, "")
        if not self._key:
            raise LLMPermanent(f"missing environment variable {self.key_env}", fatal=True)
        if not model:
            raise LLMPermanent(f"missing model name for {self.name}", fatal=True)
        self.policy = policy or RetryPolicy()
        self.breaker = breaker or CircuitBreaker()
        self._sleep = sleep
        self.client = httpx.Client(
            timeout=httpx.Timeout(connect=connect_s, read=read_s, write=read_s, pool=pool_s),
            limits=httpx.Limits(max_connections=max_conn, max_keepalive_connections=max_conn),
            transport=transport)

    name = "http"

    @property
    def tag(self):
        return f"{self.name}:{self.model}"

    def __repr__(self):  # never prints the key
        return f"{type(self).__name__}(model={self.model!r}, base_url={self.base_url!r})"

    def headers(self):  # pragma: no cover - subclasses implement
        raise NotImplementedError

    def body(self, system, user):  # pragma: no cover
        raise NotImplementedError

    def parse(self, data):  # pragma: no cover
        raise NotImplementedError

    path = ""

    def _once(self, system, user):
        try:
            resp = self.client.post(self.base_url + self.path, headers=self.headers(), json=self.body(system, user))
        except (httpx.ReadTimeout, httpx.WriteTimeout) as e:
            raise LLMUnknownOutcome(f"timed out waiting for the response: {type(e).__name__}") from None
        except (httpx.ConnectError, httpx.ConnectTimeout, httpx.PoolTimeout, httpx.RemoteProtocolError,
                httpx.ReadError, httpx.WriteError) as e:
            raise LLMTransient(f"network error: {type(e).__name__}") from None
        err = map_status(resp)
        if err:
            raise err
        try:
            return self.parse(resp.json())
        except (ValueError, KeyError, IndexError, TypeError) as e:
            raise LLMPermanent(f"malformed response: {e}") from None

    def send(self, system, user):
        kw = {"sleep": self._sleep} if self._sleep else {}
        return call_with_retry(lambda: self._once(system, user), self.policy, self.breaker, **kw)

    def close(self):
        self.client.close()
