"""
Feature store for caching computed features.
"""

import hashlib
import os
import tempfile
from datetime import datetime
from pathlib import Path

import polars as pl
import structlog

logger = structlog.get_logger()


class FeatureStore:
    """Feature store with parquet-based caching."""

    def __init__(self, cache_dir: str | None = None):
        # Resolved from the platform temp dir rather than a hardcoded "/tmp",
        # which is not the temp location on every platform and is flagged by
        # bandit B108. Callers (and the FEATURE_CACHE_DIR env var) still win.
        base = (
            cache_dir
            or os.environ.get("FEATURE_CACHE_DIR")
            or str(Path(tempfile.gettempdir()) / "features")
        )
        self.cache_dir = Path(base)
        self.cache_dir.mkdir(parents=True, exist_ok=True)
        logger.info("feature_store.initialized", cache_dir=str(self.cache_dir))

    def _compute_cache_key(self, user_ids: list[int], as_of: datetime) -> str:
        """Compute cache key from user IDs and timestamp."""
        key_str = f"{sorted(user_ids)}_{as_of.isoformat()}"
        # Non-security use: this only needs a stable, collision-resistant
        # digest for a cache filename (bandit B324).
        return hashlib.sha256(key_str.encode()).hexdigest()[:32]

    def get_cached(
        self, user_ids: list[int], as_of: datetime, ttl_hours: int = 24
    ) -> pl.DataFrame | None:
        """Get cached features if available and not expired."""
        cache_key = self._compute_cache_key(user_ids, as_of)
        cache_path = self.cache_dir / f"{cache_key}.parquet"

        if cache_path.exists():
            modified = datetime.fromtimestamp(cache_path.stat().st_mtime)
            age_hours = (datetime.utcnow() - modified).total_seconds() / 3600

            if age_hours < ttl_hours:
                logger.debug(
                    "feature_store.cache_hit",
                    cache_key=cache_key,
                    age_hours=round(age_hours, 2),
                )
                return pl.read_parquet(cache_path)
            else:
                logger.debug(
                    "feature_store.cache_expired",
                    cache_key=cache_key,
                    age_hours=round(age_hours, 2),
                )
                cache_path.unlink()  # Remove expired cache

        logger.debug("feature_store.cache_miss", cache_key=cache_key)
        return None

    def cache(self, df: pl.DataFrame, user_ids: list[int], as_of: datetime):
        """Cache features to parquet file."""
        cache_key = self._compute_cache_key(user_ids, as_of)
        cache_path = self.cache_dir / f"{cache_key}.parquet"

        df.write_parquet(cache_path, compression="zstd")
        logger.info(
            "feature_store.cached",
            cache_key=cache_key,
            rows=len(df),
            path=str(cache_path),
        )

    def clear_expired(self, ttl_hours: int = 24):
        """Clear expired cache files."""
        cleared = 0
        for cache_file in self.cache_dir.glob("*.parquet"):
            modified = datetime.fromtimestamp(cache_file.stat().st_mtime)
            age_hours = (datetime.utcnow() - modified).total_seconds() / 3600

            if age_hours > ttl_hours:
                cache_file.unlink()
                cleared += 1

        if cleared > 0:
            logger.info("feature_store.cleared_expired", count=cleared)
