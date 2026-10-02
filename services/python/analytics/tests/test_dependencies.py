"""Guard: requirements.txt and pyproject.toml must declare the same pins.

CI installs this service through two paths:
  - `pip install -r requirements.txt`  (.github/workflows/ci-python.yml)
  - `pip install -e ".[dev]"`          (.github/workflows/ci.yaml)
and the container build uses requirements.txt as well
(infra/docker/Dockerfile.python). If the files drift, one path resolves a
different stack than the others.
"""

import re
import tomllib
from pathlib import Path

import pytest

SERVICE_DIR = Path(__file__).resolve().parents[1]

_PIN_RE = re.compile(r"^([A-Za-z0-9._-]+)(?:\[[A-Za-z0-9,._-]+\])?\s*==\s*([A-Za-z0-9._+!-]+)$")

# Lint/test tooling that CI installs on its own (unpinned).
_CI_TOOL_RE = re.compile(r"^(pytest|pytest-asyncio|pytest-cov|ruff|black|mypy)([<>=!~0-9.]*)$")
CI_TOOLS = {"pytest", "pytest-asyncio", "pytest-cov", "ruff", "black", "mypy"}


def _normalize(name: str) -> str:
    return name.lower().replace("_", "-")


def parse_requirements() -> dict[str, str]:
    pins: dict[str, str] = {}
    for raw_line in (SERVICE_DIR / "requirements.txt").read_text(encoding="utf-8").splitlines():
        line = raw_line.split("#", 1)[0].strip()
        if not line or line.startswith("-"):
            continue
        match = _PIN_RE.match(line)
        if match is None:
            pytest.fail(f"Unpinned requirement (drift risk): {raw_line!r}")
        pins[_normalize(match.group(1))] = match.group(2)
    return pins


def parse_pyproject(include_dev: bool = False) -> dict[str, str]:
    data = tomllib.loads((SERVICE_DIR / "pyproject.toml").read_text(encoding="utf-8"))
    specs = list(data["project"]["dependencies"])
    if include_dev:
        for extra in data["project"].get("optional-dependencies", {}).values():
            specs.extend(extra)

    pins: dict[str, str] = {}
    for spec in specs:
        match = _PIN_RE.match(spec)
        if match is None:
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

    def test_requirements_fully_pinned(self):
        pins = parse_requirements()
        for package in ("fastapi", "pydantic", "clickhouse-connect", "structlog"):
            assert package in pins, f"{package} missing from requirements.txt"

    def test_pyproject_fully_pinned(self):
        assert parse_pyproject(include_dev=True)

    def test_no_heavy_unused_dependency(self):
        """apache-airflow was removed: unused and it broke CI installs."""
        declared = " ".join(parse_pyproject().keys())
        assert "airflow" not in declared

    def test_sets_match(self):
        requirements = parse_requirements()
        pyproject = parse_pyproject(include_dev=True)

        missing = sorted(set(requirements) - set(pyproject))
        assert not missing, f"absent from pyproject.toml: {missing}"

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


class TestContainerContract:
    """infra/docker/Dockerfile.python builds this service from requirements.txt."""

    def test_requirements_file_exists_for_docker_build(self):
        assert (SERVICE_DIR / "requirements.txt").is_file()

    def test_root_shim_present_for_uvicorn_main_app(self):
        """The image runs `uvicorn main:app` from the service directory."""
        shim = SERVICE_DIR / "main.py"
        assert shim.is_file()
        assert "app" in shim.read_text(encoding="utf-8")
