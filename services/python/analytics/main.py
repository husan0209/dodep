"""Container entrypoint shim for the generic Dockerfile.python.

The image runs ``uvicorn main:app`` from the service directory, so this
module re-exports the FastAPI application from the ``src`` layout.
Local development should use ``src/analytics/main.py`` directly.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "src"))

from analytics.main import app

__all__ = ["app"]
