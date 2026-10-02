"""Tests for the parquet-backed feature cache."""

from datetime import datetime, timedelta

import polars as pl
import pytest

from src.data.feature_store import FeatureStore


@pytest.fixture
def store(tmp_path) -> FeatureStore:
    return FeatureStore(cache_dir=str(tmp_path / "features"))


@pytest.fixture
def frame() -> pl.DataFrame:
    return pl.DataFrame({"user_id": [1, 2], "value": [0.5, 0.7]})


class TestCacheRoundTrip:
    """Cached frames survive a write/read cycle."""

    def test_cache_then_read(self, store, frame):
        users = [1, 2]
        as_of = datetime(2026, 1, 1)

        store.cache(frame, user_ids=users, as_of=as_of)
        cached = store.get_cached(users, as_of)

        assert cached is not None
        assert cached.height == frame.height
        assert cached["user_id"].to_list() == [1, 2]

    def test_cache_key_is_order_independent(self, store, frame):
        as_of = datetime(2026, 1, 1)
        store.cache(frame, user_ids=[2, 1], as_of=as_of)

        # Same users in a different order must hit the same cache entry
        assert store.get_cached([1, 2], as_of) is not None

    def test_different_timestamp_is_a_cache_miss(self, store, frame):
        store.cache(frame, user_ids=[1, 2], as_of=datetime(2026, 1, 1))
        assert store.get_cached([1, 2], datetime(2026, 1, 2)) is None

    def test_different_users_is_a_cache_miss(self, store, frame):
        store.cache(frame, user_ids=[1, 2], as_of=datetime(2026, 1, 1))
        assert store.get_cached([3, 4], datetime(2026, 1, 1)) is None


class TestExpiry:
    """TTL logic deletes stale entries instead of serving them."""

    def test_expired_entry_is_dropped(self, store, frame):
        users = [1, 2]
        as_of = datetime(2026, 1, 1)
        store.cache(frame, user_ids=users, as_of=as_of)

        cache_files = list(store.cache_dir.glob("*.parquet"))
        assert len(cache_files) == 1

        # Backdate the file beyond the TTL
        stale = datetime.now() - timedelta(hours=48)
        import os

        os.utime(cache_files[0], (stale.timestamp(), stale.timestamp()))

        assert store.get_cached(users, as_of, ttl_hours=24) is None
        # Stale file was removed, not left behind
        assert list(store.cache_dir.glob("*.parquet")) == []

    def test_fresh_entry_is_kept(self, store, frame):
        users = [1, 2]
        as_of = datetime(2026, 1, 1)
        store.cache(frame, user_ids=users, as_of=as_of)

        assert store.get_cached(users, as_of, ttl_hours=24) is not None
        assert len(list(store.cache_dir.glob("*.parquet"))) == 1

    def test_clear_expired_removes_only_stale_files(self, store, frame):
        as_of = datetime(2026, 1, 1)
        store.cache(frame, user_ids=[1, 2], as_of=as_of)
        store.cache(frame, user_ids=[3, 4], as_of=datetime(2026, 1, 2))

        files = list(store.cache_dir.glob("*.parquet"))
        assert len(files) == 2

        import os

        stale = datetime.now() - timedelta(hours=72)
        os.utime(files[0], (stale.timestamp(), stale.timestamp()))

        store.clear_expired(ttl_hours=24)

        remaining = list(store.cache_dir.glob("*.parquet"))
        assert len(remaining) == 1
        assert remaining[0] == files[1]

    def test_clear_expired_on_empty_cache_is_noop(self, store):
        store.clear_expired(ttl_hours=24)
        assert list(store.cache_dir.glob("*.parquet")) == []
