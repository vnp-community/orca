"""One-line structured progress on stderr (stdout carries the result)."""
import sys

QUIET = False


def log(event: str, **kw) -> None:
    if QUIET:
        return
    print("tsa: " + event + "".join(f" {k}={v}" for k, v in kw.items()), file=sys.stderr, flush=True)
