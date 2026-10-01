"""Tests for settings parsing from environment variables.

Regression guard: pydantic-settings JSON-decodes list-typed fields coming
from the environment. With `redpanda_brokers: list[str]` and the CI value
`REDPANDA_BROKERS=localhost:9092` (plain CSV) every module importing the
settings raised SettingsError at import time, which broke pytest collection
in the fraud-ml CI job.
"""

import importlib

import pytest

from internal.config import Settings as DetectorSettings
from internal.config import parse_brokers
from src.config import Settings as PipelineSettings


class TestParseBrokers:
    """Broker parsing accepts CSV and JSON, rejects neither."""

    @pytest.mark.parametrize(
        "raw,expected",
        [
            ("localhost:9092", ["localhost:9092"]),
            ("a:9092,b:9092", ["a:9092", "b:9092"]),
            (" a:9092 , b:9092 ", ["a:9092", "b:9092"]),
            ('["a:9092","b:9092"]', ["a:9092", "b:9092"]),
            ("[not-json]", ["[not-json]"]),
            ("", []),
        ],
    )
    def test_formats(self, raw, expected):
        assert parse_brokers(raw) == expected

    def test_list_input_passthrough(self):
        assert parse_brokers(["a:9092"]) == ["a:9092"]


class TestSettingsFromEnv:
    """Settings instantiate with the exact env used by CI."""

    def test_detector_settings_accept_csv_env(self, monkeypatch):
        monkeypatch.setenv("REDPANDA_BROKERS", "localhost:9092")
        monkeypatch.setenv("CLICKHOUSE_HOST", "localhost")
        monkeypatch.setenv("CLICKHOUSE_PORT", "9000")

        settings = DetectorSettings()

        assert settings.redpanda_brokers == "localhost:9092"
        assert settings.redpanda_broker_list == ["localhost:9092"]
        assert settings.clickhouse_host == "localhost"
        assert settings.clickhouse_port == 9000

    def test_detector_settings_accept_multiple_csv_brokers(self, monkeypatch):
        monkeypatch.setenv("REDPANDA_BROKERS", "a:9092,b:9092")

        assert DetectorSettings().redpanda_broker_list == ["a:9092", "b:9092"]

    def test_detector_settings_accept_json_env(self, monkeypatch):
        monkeypatch.setenv("REDPANDA_BROKERS", '["a:9092","b:9092"]')

        assert DetectorSettings().redpanda_broker_list == ["a:9092", "b:9092"]

    def test_pipeline_settings_accept_csv_env(self, monkeypatch):
        monkeypatch.setenv("REDPANDA_BROKERS", "localhost:9092")

        settings = PipelineSettings()

        assert settings.redpanda_broker_list == ["localhost:9092"]

    def test_defaults_without_env(self, monkeypatch):
        monkeypatch.delenv("REDPANDA_BROKERS", raising=False)

        assert DetectorSettings().redpanda_broker_list == ["localhost:9092"]
        assert PipelineSettings().redpanda_broker_list == ["localhost:9092"]

    def test_quality_thresholds_are_tunable(self, monkeypatch):
        monkeypatch.setenv("MAX_FPR", "0.02")
        monkeypatch.setenv("MIN_RECALL_AT_MAX_FPR", "0.8")
        monkeypatch.setenv("MODEL_QUALITY_THRESHOLD", "0.95")

        settings = PipelineSettings()

        assert settings.max_fpr == pytest.approx(0.02)
        assert settings.min_recall_at_max_fpr == pytest.approx(0.8)
        assert settings.model_quality_threshold == pytest.approx(0.95)


class TestModuleImportsWithCiEnv:
    """Importing the app under CI env must not raise."""

    def test_detector_module_imports(self, monkeypatch):
        monkeypatch.setenv("REDPANDA_BROKERS", "localhost:9092")
        module = importlib.import_module("internal.models.fraud_detector")

        assert module.FraudDetector is not None

    def test_main_module_imports(self, monkeypatch):
        monkeypatch.setenv("REDPANDA_BROKERS", "localhost:9092")
        module = importlib.import_module("main")

        assert module.app is not None
