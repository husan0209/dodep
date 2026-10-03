"""
Local mirror of the Architecture Guards (.github/workflows/architecture-guards.yml).

Lets developers run the guards without pushing, and documents each rule's
scope so the CI and the local output never drift.

Usage:
    py tools/guards/architecture_guards.py
    py tools/guards/architecture_guards.py --rule G4

Exit code is non-zero when a guard fails — same contract as CI.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def _configure_stdio() -> None:
    """Make output safe for a cp1251/cp437 console (Windows CI shells).

    Violation lines contain arbitrary repo text — including a BOM when a
    file was written by a PowerShell editor. Without this the guard would
    crash *while reporting* a violation, turning a clean failure into a
    confusing UnicodeEncodeError.
    """
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8", errors="replace")
        except (AttributeError, ValueError):
            pass

# ── Rule definitions ────────────────────────────────────────────────────────
# Each rule: (id, title, file globs, regex, is_blocking, explanation)


def _clean(line: str) -> str:
    """Normalise a source line for printing (BOM, control chars, length)."""
    text = line.strip().lstrip("﻿")
    text = "".join(ch for ch in text if ch == "\t" or ch >= " ")
    return text[:200]


def _git_files() -> list[str]:
    """Repo-relative paths of tracked + untracked (not ignored) files.

    Uses `git ls-files` so the guard sees exactly what CI would ship and
    skips node_modules / target / .next without walking them.
    Falls back to a manual walk if git is unavailable.
    """
    try:
        result = subprocess.run(
            ["git", "ls-files", "--cached", "--others", "--exclude-standard"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=True,
            timeout=60,
        )
    except (OSError, subprocess.SubprocessError):
        skip = {
            "node_modules", "target", ".next", ".turbo", "test-results",
            "playwright-report", "dist", "build", "__pycache__",
            ".mypy_cache", ".pytest_cache", ".ruff_cache", "coverage",
            ".venv", "venv",
        }
        found: list[str] = []
        for path in ROOT.rglob("*"):
            if not path.is_file():
                continue
            rel = path.relative_to(ROOT)
            if any(part in skip for part in rel.parts):
                continue
            found.append(rel.as_posix())
        return found
    return [line for line in result.stdout.splitlines() if line]


def _go_files(include_cmd: bool) -> list[Path]:
    files = sorted((ROOT / "services" / "go").rglob("*.go"))
    out = []
    for path in files:
        if path.name.endswith("_test.go"):
            continue
        rel = path.relative_to(ROOT).as_posix()
        if not include_cmd and "/cmd/" in f"/{rel}":
            continue
        out.append(path)
    return out


def g1_float_money() -> tuple[bool, list[str]]:
    """G1: no f32/f64 or float32/float64 for balances in money services."""
    hits: list[str] = []
    patterns = (
        (ROOT / "services/rust/wallet-core/src", re.compile(r"balance.*(?:f32|f64)")),
        (ROOT / "services/go/payment", re.compile(r"balance.*(?:float32|float64)")),
    )
    for base, rx in patterns:
        if not base.exists():
            continue
        for path in sorted(base.rglob("*")):
            if not path.is_file():
                continue
            for num, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
                if rx.search(line):
                    hits.append(f"{path.relative_to(ROOT).as_posix()}:{num}: {_clean(line)}")
    return (not hits), hits


def g2_raw_prints() -> tuple[bool, list[str]]:
    """G2: no fmt.Println in service code (tests and cmd/ are exempt)."""
    rx = re.compile(r"fmt\.Println\(")
    hits: list[str] = []
    for path in _go_files(include_cmd=False):
        for num, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            if rx.search(line):
                hits.append(f"{path.relative_to(ROOT).as_posix()}:{num}: {_clean(line)}")
    return (not hits), hits


def g3_hardcoded_keys() -> tuple[bool, list[str]]:
    """G3: no private key material committed to the repo.

    Scope notes — the check looks for PEM headers, which also appear in
    documentation and in the guard's own definition, so those are excluded:
      - this file and the CI workflow (they *define* the marker strings)
      - dot-directories (`.claude/`, `.qwen/`, …): bundled agent-skill
        documentation, not project code
    """
    markers = ("BEGIN RSA PRIVATE KEY", "BEGIN OPENSSH PRIVATE KEY")
    self_rel = "tools/guards/architecture_guards.py"
    workflow_rel = ".github/workflows/architecture-guards.yml"
    hits: list[str] = []

    # Enumerate tracked+untracked project files through git instead of
    # rglob(): it is ~100x faster on a tree with node_modules/target/.next
    # and it matches what CI would actually ship.
    paths = _git_files()
    for rel_str in paths:
        if rel_str in (self_rel, workflow_rel):
            continue
        rel = Path(rel_str)
        if rel.parts and rel.parts[0].startswith("."):
            continue
        try:
            text = (ROOT / rel).read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        for num, line in enumerate(text.splitlines(), 1):
            if any(marker in line for marker in markers):
                hits.append(f"{rel_str}:{num}")
    return (not hits), hits


def g4_ts_ignore() -> tuple[bool, list[str]]:
    """G4: no @ts-ignore / eslint-disable in the frontend.

    Both suppressions defeat the type checker and the linter; the repo treats
    them as violations, so an unused directive is a violation too.
    """
    rx = re.compile(r"@ts-ignore|@ts-nocheck|eslint-disable")
    hits: list[str] = []
    for path in sorted((ROOT / "apps").rglob("*")):
        if not path.is_file() or "node_modules" in path.parts or ".next" in path.parts:
            continue
        if path.suffix not in {".ts", ".tsx", ".js", ".jsx", ".mjs", ".vue"}:
            continue
        for num, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            if rx.search(line):
                hits.append(f"{path.relative_to(ROOT).as_posix()}:{num}: {_clean(line)}")
    return (not hits), hits


RULES = {
    "G1": ("No float types for money in wallet/payment", g1_float_money),
    "G2": ("No fmt.Println in Go service code", g2_raw_prints),
    "G3": ("No hardcoded private keys", g3_hardcoded_keys),
    "G4": ("No @ts-ignore / eslint-disable in apps/", g4_ts_ignore),
}


def main() -> int:
    _configure_stdio()
    parser = argparse.ArgumentParser(description="Run the Architecture Guards locally.")
    parser.add_argument("--rule", choices=sorted(RULES), help="run a single guard")
    args = parser.parse_args()

    selected = [args.rule] if args.rule else sorted(RULES)
    failed: list[str] = []

    for rule_id in selected:
        title, fn = RULES[rule_id]
        print(f"{rule_id}: {title}")
        ok, hits = fn()
        if ok:
            print(f"  OK — no violations")
        else:
            print(f"  FAIL — {len(hits)} violation(s):")
            for hit in hits[:20]:
                print(f"    {hit}")
            if len(hits) > 20:
                print(f"    … and {len(hits) - 20} more")
            failed.append(rule_id)

    if failed:
        print(f"\nFailed guards: {', '.join(failed)}")
        return 1
    print(f"\nAll guards passed ({', '.join(selected)}).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
