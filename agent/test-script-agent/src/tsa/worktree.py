"""The dedicated git worktree TSA writes into (branch tsa/<feature>-v<specVersion>). Only commands available since Git 2.25."""
from __future__ import annotations

import re
import subprocess
from pathlib import Path

from .errors import WorktreeError

BRANCH_SAFE = re.compile(r"[^A-Za-z0-9._-]+")


def branch_name(feature: str, spec_version: int) -> str:
    return f"tsa/{BRANCH_SAFE.sub('-', feature)}-v{spec_version}"


def default_path(repo: Path, branch: str) -> Path:
    # Outside the repo so the worktree never shows up as untracked files of the main checkout.
    return repo.resolve().parent / f"{repo.resolve().name}.tsa-worktrees" / branch.replace("/", "-")


def git(cwd: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess:
    proc = subprocess.run(["git", *args], cwd=cwd, capture_output=True, text=True)
    if check and proc.returncode != 0:
        raise WorktreeError(f"git {' '.join(args)} failed: {proc.stderr.strip() or proc.stdout.strip()}")
    return proc


def _branch_exists(repo: Path, branch: str) -> bool:
    return git(repo, "rev-parse", "--verify", "--quiet", f"refs/heads/{branch}", check=False).returncode == 0


def _worktree_branch(path: Path) -> str | None:
    proc = git(path, "rev-parse", "--abbrev-ref", "HEAD", check=False)
    return proc.stdout.strip() if proc.returncode == 0 else None


# Besides the test folders, only the files pytest reads its settings from; never the application source.
PYTEST_CONFIG_FILES = ("pyproject.toml", "pytest.ini", "setup.cfg", "tox.ini", "conftest.py")


def sparse_patterns(paths: list[str]) -> str:
    """Non-cone sparse-checkout patterns: the given folders and the root pytest config files, nothing else."""
    out = []
    for p in paths:
        p = p.strip().strip("/")
        if not p or ".." in p.split("/"):
            raise WorktreeError(f"invalid tests path {p!r}")
        out.append(f"/{p}/")
    return "\n".join(out + [f"/{f}" for f in PYTEST_CONFIG_FILES]) + "\n"


def _make_sparse(repo: Path, path: Path, paths: list[str]) -> None:
    # Per-worktree config (Git >= 2.20) so the main checkout of the repository is never made sparse.
    git(repo, "config", "extensions.worktreeConfig", "true")
    if git(repo, "config", "--get", "core.repositoryformatversion", check=False).stdout.strip() in ("", "0"):
        git(repo, "config", "core.repositoryformatversion", "1")
    git(path, "config", "--worktree", "core.sparseCheckout", "true")
    git(path, "config", "--worktree", "core.sparseCheckoutCone", "false")
    gitdir = Path(git(path, "rev-parse", "--git-dir").stdout.strip())
    gitdir = gitdir if gitdir.is_absolute() else path / gitdir
    (gitdir / "info").mkdir(parents=True, exist_ok=True)
    (gitdir / "info" / "sparse-checkout").write_text(sparse_patterns(paths), encoding="utf-8")
    git(path, "read-tree", "-mu", "HEAD")


def outside_paths(path: Path, paths: list[str]) -> list[str]:
    """Checked-out files that are neither under `paths` nor a root pytest config file: must be empty for an isolated worktree."""
    allowed = tuple(p.strip("/") + "/" for p in paths)
    out = []
    for f in git(path, "ls-files").stdout.splitlines():
        if f.startswith(allowed) or f in PYTEST_CONFIG_FILES:
            continue
        if (path / f).exists():
            out.append(f)
    return out


def create(repo: Path, base: str, branch: str, path: Path | None = None, reuse: bool = False,
           sparse_paths: list[str] | None = None) -> Path:
    """Add the worktree on a new branch from `base`. An existing worktree/branch is an error unless `reuse` (then it must match).

    With `sparse_paths` the worktree only checks out those folders (and the pytest config files): used when the test repository is
    also the application repository, so the generator cannot read the application source."""
    repo = Path(repo)
    if git(repo, "rev-parse", "--is-inside-work-tree", check=False).returncode != 0:
        raise WorktreeError(f"{repo} is not a git repository")
    git(repo, "rev-parse", "--verify", "--quiet", f"{base}^{{commit}}")
    path = Path(path) if path else default_path(repo, branch)
    if path.exists():
        if reuse and _worktree_branch(path) == branch:
            return path
        raise WorktreeError(f"{path} already exists" + ("" if reuse else "; pass --reuse to continue in it"))
    path.parent.mkdir(parents=True, exist_ok=True)
    no_checkout = ["--no-checkout"] if sparse_paths else []
    if _branch_exists(repo, branch):
        if not reuse:
            raise WorktreeError(f"branch {branch} already exists; pass --reuse to check it out in a new worktree")
        git(repo, "worktree", "add", *no_checkout, str(path), branch)
    else:
        git(repo, "worktree", "add", *no_checkout, "-b", branch, str(path), base)
    if sparse_paths:
        _make_sparse(repo, path, sparse_paths)
        leaked = outside_paths(path, sparse_paths)
        if leaked:
            raise WorktreeError(f"sparse worktree still has files outside {sparse_paths}: {leaked[:5]}")
    return path


def commit(path: Path, message: str, files: list[str]) -> str | None:
    """Commit `files` (relative to the worktree). Returns the new commit sha, or None when nothing changed."""
    if not files:
        return None
    git(path, "add", "--", *files)
    if git(path, "diff", "--cached", "--quiet", check=False).returncode == 0:
        return None
    git(path, "commit", "-q", "-m", message)
    return git(path, "rev-parse", "HEAD").stdout.strip()
