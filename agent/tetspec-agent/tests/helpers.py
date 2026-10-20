import json
import os
import stat
import sys
from pathlib import Path


def make_cli(folder: Path, name: str, scenario: list[dict]) -> Path:
    """Create an executable fake CLI.

    Each invocation consumes the next scenario entry:
    ``{"stdout": str, "stderr": str, "exit": int, "sleep": float, "output_file": str}``.
    Arguments and stdin of every call are appended to ``calls.jsonl``.
    """
    folder.mkdir(parents=True, exist_ok=True)
    (folder / f"{name}.scenario.json").write_text(json.dumps(scenario))
    script = folder / name
    script.write_text(f'''#!{sys.executable}
import json, sys, time, pathlib
here = pathlib.Path(__file__).resolve().parent
scn = json.loads((here / "{name}.scenario.json").read_text())
log = here / "calls.jsonl"
n = sum(1 for _ in log.open()) if log.exists() else 0
if "--version" in sys.argv:
    print("fake-cli 1.0"); sys.exit(0)
stdin = sys.stdin.read()
with log.open("a") as fh:
    fh.write(json.dumps({{"argv": sys.argv[1:], "stdin": stdin}}) + "\\n")
step = scn[min(n, len(scn) - 1)]
time.sleep(step.get("sleep", 0))
if "output_file" in step:
    argv = sys.argv
    pathlib.Path(argv[argv.index("-o") + 1]).write_text(step["output_file"])
sys.stdout.write(step.get("stdout", ""))
sys.stderr.write(step.get("stderr", ""))
sys.exit(step.get("exit", 0))
''')
    script.chmod(script.stat().st_mode | stat.S_IEXEC)
    return script


def calls(folder: Path) -> list[dict]:
    path = folder / "calls.jsonl"
    return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []


def claude_envelope(result="", structured=None, is_error=False, model="claude-sonnet-5-5", **extra) -> str:
    body = {"type": "result", "is_error": is_error, "result": result, "usage": {"input_tokens": 3, "output_tokens": 5},
            "total_cost_usd": 0.01, "modelUsage": {model: {}}}
    if structured is not None:
        body["structured_output"] = structured
    body.update(extra)
    return json.dumps(body)
