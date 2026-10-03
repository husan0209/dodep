"""Guard: requirements.txt and pyproject.toml must declare the same pins.

CI installs dependencies through two different paths:
  - `pip install -r requirements.txt`  (.github/workflows/ci-python-fraud-ml.yml)
  - `pip install -e ".[dev]"`          (.github/workflows/ci.yaml)

When those files drift, one path resolves a different stack than the other
and tests fail only in CI. This test pins them together.
"""

import re
import tomllib
from pathlib import Path

import pytest

SERVICE_DIR = Path(__file__).resolve().parents[1]


# "name[extra]==version" — extras (uvicorn[standard]) are part of the spec
_PIN_RE = re.compile(r"^([A-Za-z0-9._-]+)(?:\[[A-Za-z0-9,._-]+\])?\s*==\s*([A-Za-z0-9._+!-]+)$")

# Lint/test tooling that CI installs on its own (latest version, no pin).
_CI_TOOL_RE = re.compile(
    r"^(pytest|pytest-asyncio|pytest-cov|ruff|black|mypy)([<>=!~0-9.]*)$",
)
CI_TOOLS = {"pytest", "pytest-asyncio", "pytest-cov", "ruff", "black", "mypy"}


def _normalize(name: str) -> str:
    return name.lower().replace("_", "-")


def parse_requirements() -> dict[str, str]:
    """Map package name -> pinned version from requirements.txt."""
    pins: dict[str, str] = {}
    text = (SERVICE_DIR / "requirements.txt").read_text(encoding="utf-8")
    for raw_line in text.splitlines():
        line = raw_line.split("#", 1)[0].strip()
        if not line or line.startswith("-"):
            continue
        match = _PIN_RE.match(line)
        if match:
            pins[_normalize(match.group(1))] = match.group(2)
        else:
            pytest.fail(f"Unpinned requirement (drift risk): {raw_line!r}")
    return pins


def parse_pyproject(include_dev: bool = False) -> dict[str, str]:
    """
    Map package name -> pinned version from pyproject.toml.

    include_dev also reads [project.optional-dependencies].dev, which is
    what `pip install -e ".[dev]"` pulls in.
    """
    data = tomllib.loads((SERVICE_DIR / "pyproject.toml").read_text(encoding="utf-8"))
    specs = list(data["project"]["dependencies"])
    if include_dev:
        for extra in data["project"].get("optional-dependencies", {}).values():
            specs.extend(extra)

    pins: dict[str, str] = {}
    for spec in specs:
        match = _PIN_RE.match(spec)
        if match is None:
            # dev tooling is intentionally unpinned (CI installs latest)
            if include_dev and _CI_TOOL_RE.match(spec):
                name = spec.split("[")[0].split(">=")[0].split("==")[0]
                pins.setdefault(_normalize(name), "dev")
                continue
            pytest.fail(f"Unpinned dependency in pyproject.toml: {spec!r}")
            continue
        pins[_normalize(match.group(1))] = match.group(2)
    return pins


class TestDependencyFilesAgree:
    """Both install paths must resolve to the same versions."""

    def test_requirements_are_fully_pinned(self):
        pins = parse_requirements()
        assert pins, "no pinned requirements parsed"
        # The runtime stack CI needs
        for package in ("fastapi", "pydantic", "polars", "xgboost", "onnxmltools"):
            assert package in pins, f"{package} missing from requirements.txt"

    def test_pyproject_is_fully_pinned(self):
        pins = parse_pyproject(include_dev=True)
        assert pins, "no pinned dependencies parsed"

    def test_sets_match(self):
        requirements = parse_requirements()
        pyproject = parse_pyproject(include_dev=True)

        # Everything requirements.txt installs must also be reachable through
        # `pip install -e ".[dev]"`.
        missing_in_pyproject = sorted(set(requirements) - set(pyproject))
        assert not missing_in_pyproject, f"absent from pyproject.toml: {missing_in_pyproject}"

        # pyproject may add tooling that CI installs separately.
        unexpected = sorted(set(pyproject) - set(requirements) - CI_TOOLS)
        assert not unexpected, f"in pyproject.toml only: {unexpected}"

    def test_versions_match(self):
        requirements = parse_requirements()
        pyproject = parse_pyproject(include_dev=True)

        mismatched = {
            name: (requirements[name], pyproject[name])
            for name in requirements
            if name in pyproject and requirements[name] != pyproject[name]
        }
        assert not mismatched, f"version mismatch (requirements, pyproject): {mismatched}"

    def test_testclient_dependency_present(self):
        """fastapi.testclient needs httpx for tests/test_api.py."""
        dev = parse_pyproject(include_dev=True)
        assert "httpx" in dev
        assert "httpx" in parse_requirements()
