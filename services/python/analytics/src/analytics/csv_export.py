"""CSV rendering for the analytics export.

Stdlib only — no FastAPI, no database driver — so the injection guard can
be unit-tested without pulling in the HTTP stack.

Why this lives apart from ``main.py``: a security control that can only be
exercised by importing the whole web app is a security control nobody runs.
"""

from __future__ import annotations

import csv
import io
from collections.abc import Iterator
from decimal import Decimal, InvalidOperation
from typing import Any

# Characters that make a spreadsheet treat a cell as a formula rather than
# text. Affiliate names and campaign labels are attacker-influenced, so an
# unescaped "=cmd|..." would execute when an analyst opens the export.
_FORMULA_TRIGGERS = frozenset("=+-@")


def csv_cell(value: Any) -> str:
    """Render one CSV field, neutralising spreadsheet formula injection.

    ClickHouse returns Decimal columns as strings, so a negative amount
    arrives as ``"-3.00"``. Escaping that would turn a real figure into text
    and break SUM/AVERAGE in the analyst's spreadsheet, so a value that is
    entirely numeric is left alone even though it starts with ``-``. Only
    non-numeric text gets the ``'`` prefix that forces Excel and LibreOffice
    to treat it as a literal.

    The check looks past leading whitespace and control characters: both
    spreadsheets trim those before evaluating a cell.
    """
    if value is None:
        return ""
    if isinstance(value, str):
        stripped = value.lstrip(" \t\r\n")
        if stripped[:1] in _FORMULA_TRIGGERS:
            try:
                Decimal(stripped)
            except InvalidOperation:
                return f"'{value}"
    return str(value)


def csv_chunks(rows: list[dict[str, Any]]) -> Iterator[str]:
    """Yield RFC 4180 CSV text for a list of ClickHouse rows."""
    if not rows:
        return
    header = list(rows[0].keys())
    buffer = io.StringIO()
    writer = csv.writer(buffer, lineterminator="\n")
    writer.writerow(header)
    for row in rows:
        writer.writerow([csv_cell(row.get(column)) for column in header])
    yield buffer.getvalue()
