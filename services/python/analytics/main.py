"""Container entrypoint shim for the generic Dockerfile.python.

The image copies `services/python/` to `/app/` and runs
`python -m uvicorn main:app` from `/app/analytics`, so the package lives in
`src/analytics` and is not importable without help. This module puts `src`
on sys.path and re-exports the ASGI app.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "src"))

from analytics.main import app  # noqa: E402  (path setup must run first)

__all__ = ["app"]


if __name__ == "__main__":
    from analytics.main import run

    run()
