"""Retry with exponential backoff + jitter and a circuit breaker for LLM calls (INT-01 ErrorMapping, MECH-03)."""
import random
import threading
import time
from collections import deque
from dataclasses import dataclass, field

from ..errors import CircuitOpen, LLMError
from ..log import get_logger

log = get_logger("tsagent.llm")


@dataclass
class RetryPolicy:
    max_attempts: int = 4
    base_s: float = 0.5
    cap_s: float = 8.0
    total_s: float = 90.0  # maximum total wait for one call

    def delay(self, attempt, retry_after=None, rnd=random.random):
        if retry_after is not None:
            return min(float(retry_after), self.cap_s)
        return min(self.cap_s, self.base_s * (2 ** (attempt - 1))) * (0.5 + rnd() / 2)


@dataclass
class CircuitBreaker:
    """Opens when the failure ratio over the last `window` calls is >= `failure_ratio` (at least `min_calls`); half-opens after `open_s` seconds."""
    failure_ratio: float = 0.5
    min_calls: int = 6
    window: int = 20
    open_s: float = 30.0
    clock: object = time.monotonic
    _results: deque = field(default_factory=deque)
    _opened_at: float = None
    _half_open: bool = False
    _lock: object = field(default_factory=threading.Lock)

    @property
    def state(self):
        if self._opened_at is None:
            return "closed"
        return "half-open" if self.clock() - self._opened_at >= self.open_s else "open"

    def before(self):
        with self._lock:
            st = self.state
            if st == "open":
                raise CircuitOpen(f"circuit breaker is open ({self.open_s - (self.clock() - self._opened_at):.0f}s left)")
            if st == "half-open":
                if self._half_open:
                    raise CircuitOpen("circuit breaker is half-open: waiting for the trial call")
                self._half_open = True

    def record(self, ok):
        with self._lock:
            if self._half_open:
                self._half_open = False
                if ok:
                    self._opened_at = None
                    self._results.clear()
                else:
                    self._opened_at = self.clock()
                return
            self._results.append(ok)
            while len(self._results) > self.window:
                self._results.popleft()
            n = len(self._results)
            fails = sum(1 for r in self._results if not r)
            if n >= self.min_calls and fails / n >= self.failure_ratio:
                self._opened_at = self.clock()
                log.warning("circuit breaker opened: %d/%d calls failed", fails, n)


def call_with_retry(fn, policy, breaker=None, sleep=time.sleep, clock=time.monotonic):
    """Call `fn()`; `retryable` errors are retried; `fatal` or permanent errors are raised at once."""
    start = clock()
    attempt = 0
    while True:
        attempt += 1
        if breaker:
            breaker.before()
        try:
            out = fn()
        except LLMError as e:
            if breaker:
                breaker.record(False)
            if e.fatal or not e.retryable or attempt >= policy.max_attempts:
                raise
            wait = policy.delay(attempt, getattr(e, "retry_after", None))
            if clock() - start + wait > policy.total_s:
                raise
            log.info("LLM error %s, retrying (attempt %d) in %.1fs", e.category, attempt + 1, wait)
            sleep(wait)
            continue
        if breaker:
            breaker.record(True)
        return out
