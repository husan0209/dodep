"""Tests for the ClickHouse data access layer (stubbed client).

These pin the query-result handling against the clickhouse-connect 0.7.x
API: QueryResult exposes `result_columns` (not `column_names`) and no Arrow
accessor, and Client exposes `query_df` / `query` / `command`.
"""

import pandas as pd
import pytest

from internal.repository.clickhouse_repository import ClickHouseRepository


class StubQueryResult:
    """Stand-in for clickhouse_connect.driver.query.QueryResult."""

    def __init__(self, rows, columns=None):
        self.result_rows = rows
        self.result_columns = columns or []


class StubClient:
    """Records executed queries and returns canned results."""

    def __init__(self, df=None, row=None):
        self._df = df if df is not None else pd.DataFrame()
        self._row = row
        self.queries = []
        self.commands = []
        self.inserted_payloads = []

    def query_df(self, query, parameters=None):
        self.queries.append((query, parameters))
        return self._df

    def query(self, query, parameters=None):
        self.queries.append((query, parameters))
        if self._row is None:
            return StubQueryResult([])
        return StubQueryResult([self._row])

    def command(self, query, parameters=None, data=None):
        self.commands.append((query, parameters))
        self.inserted_payloads.append(parameters)
        return 1


@pytest.fixture
def repo():
    return ClickHouseRepository(client=StubClient())


class TestListQueries:
    """List endpoints return DataFrames and bind parameters safely."""

    def test_get_user_bets(self, repo):
        repo.client._df = pd.DataFrame({"id": [1], "stake": [10.5]})
        df = repo.get_user_bets(user_id=7, hours=48, limit=10)

        assert isinstance(df, pd.DataFrame)
        assert list(df.columns) == ["id", "stake"]
        query, params = repo.client.queries[0]
        assert params == {"user_id": 7, "hours": 48, "limit": 10}
        # user_id is bound, never interpolated into SQL
        assert "7" not in query.replace("bet_events", "")

    def test_get_user_transactions(self, repo):
        repo.get_user_transactions(user_id=1)
        _, params = repo.client.queries[0]
        assert params == {"user_id": 1, "hours": 24, "limit": 1000}

    def test_get_user_login_history_defaults_to_week(self, repo):
        repo.get_user_login_history(user_id=2)
        _, params = repo.client.queries[0]
        assert params["hours"] == 168

    def test_get_user_bonus_history(self, repo):
        repo.get_user_bonus_history(user_id=3, days=7, limit=5)
        _, params = repo.client.queries[0]
        assert params == {"user_id": 3, "days": 7, "limit": 5}

    def test_empty_result_returns_empty_frame(self, repo):
        repo.client._df = pd.DataFrame()
        assert repo.get_user_bets(user_id=1).empty


class TestSingleRowQueries:
    """Row endpoints handle the empty case explicitly."""

    def test_get_user_profile(self, repo):
        repo.client._df = pd.DataFrame([{"user_id": 5, "country": "NL", "kyc_level": 3}])
        profile = repo.get_user_profile(user_id=5)

        assert profile is not None
        assert profile["user_id"] == 5
        assert profile["country"] == "NL"

    def test_get_user_profile_returns_none_when_empty(self, repo):
        repo.client._df = pd.DataFrame()
        assert repo.get_user_profile(user_id=5) is None

    def test_get_fraud_statistics(self, repo):
        repo.client._row = (120, 7, 42.5)
        stats = repo.get_fraud_statistics(hours=24)

        assert stats == {
            "total_events": 120,
            "fraud_count": 7,
            "avg_risk_score": 42.5,
        }

    def test_get_fraud_statistics_empty(self, repo):
        repo.client._row = None
        assert repo.get_fraud_statistics() == {}


class TestAggregatedFeatures:
    """Aggregations coerce nullable ClickHouse sums to floats."""

    def test_aggregated_features_with_data(self, repo):
        repo.client._row = (10, 250.5, 25.05, 100.0, 60.0, 4, 6)
        features = repo.get_aggregated_user_features(user_id=9)

        assert features["total_bets_30d"] == 10
        assert features["total_staked_30d"] == 250.5
        assert features["wins_30d"] == 4
        assert features["losses_30d"] == 6

    def test_aggregated_features_handle_null_sums(self, repo):
        repo.client._row = (0, None, None, None, None, 0, 0)
        features = repo.get_aggregated_user_features(user_id=9)

        assert features["total_staked_30d"] == 0
        assert features["avg_stake"] == 0

    def test_aggregated_features_without_rows(self, repo):
        repo.client._row = None
        assert repo.get_aggregated_user_features(user_id=9) == {}


class TestInsert:
    """Insert path serializes the JSON column and binds values."""

    def test_insert_fraud_signal_serializes_features(self, repo):
        repo.insert_fraud_signal(
            {
                "event_time": "2026-01-01 00:00:00",
                "user_id": 4,
                "fraud_type": "bonus_abuse",
                "risk_score": 88.0,
                "is_fraud": 1,
                "features": {"deposit_bonus_ratio": 12.5},
                "explanation": "multi-account",
            }
        )

        query, params = repo.client.commands[0]
        assert "INSERT INTO fraud_signals" in query
        # features is a JSON column: a dict would fail to bind
        assert params["features"] == '{"deposit_bonus_ratio": 12.5}'
        assert params["user_id"] == 4

    def test_insert_keeps_already_serialized_features(self, repo):
        repo.insert_fraud_signal(
            {"features": '{"a": 1}'},
        )
        _, params = repo.client.commands[0]
        assert params["features"] == '{"a": 1}'
