"""Unit tests for CSV export rendering (RFC 4180 + formula-injection guard).

Stdlib only (``unittest``) - ``csv_export.py`` has no FastAPI or driver
import, so the injection guard is testable without the web stack.
Run: ``py -m unittest discover -s tests -v`` from the service directory.
"""

import csv
import io
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from analytics.csv_export import csv_cell, csv_chunks


class TestCsvCell(unittest.TestCase):
    def test_plain_values_pass_through(self):
        self.assertEqual(csv_cell("acme"), "acme")
        self.assertEqual(csv_cell(""), "")
        self.assertEqual(csv_cell(None), "")

    def test_numbers_stay_numeric(self):
        """A negative number must NOT be escaped, or SUM() breaks in Excel."""
        self.assertEqual(csv_cell(-42), "-42")
        self.assertEqual(csv_cell(-1.5), "-1.5")

    def test_formula_triggers_are_neutralised(self):
        for payload in ("=1+1", "+1+1", "-1+1", "@SUM(A1)", "=cmd|'/c calc'!A1"):
            self.assertEqual(csv_cell(payload), f"'{payload}", msg=payload)

    def test_negative_numeric_string_is_not_escaped(self):
        """ClickHouse hands Decimal columns over as strings.

        Escaping "-3.00" would make it text in the spreadsheet and break
        SUM/AVERAGE for negative commission reversals and chargebacks.
        """
        self.assertEqual(csv_cell("-3.00"), "-3.00")
        self.assertEqual(csv_cell("-0.01"), "-0.01")
        self.assertEqual(csv_cell("  -12.50 "), "  -12.50 ")

    def test_minus_prefixed_formula_is_still_escaped(self):
        """-2+3+cmd is a formula, not a number, despite the leading '-'."""
        payload = "-2+3+cmd|'/c calc'!A0"
        self.assertEqual(csv_cell(payload), f"'{payload}")

    def test_whitespace_prefixed_formula_is_neutralised(self):
        """Excel trims leading whitespace before evaluating the cell."""
        self.assertEqual(csv_cell("  =1+1"), "'  =1+1")
        self.assertEqual(csv_cell("\t=1+1"), "'\t=1+1")

    def test_inner_trigger_characters_are_not_escaped(self):
        """Only a leading trigger matters; mid-string '=' is harmless."""
        self.assertEqual(csv_cell("a=b"), "a=b")
        self.assertEqual(csv_cell("2+2"), "2+2")


class TestCsvChunks(unittest.TestCase):
    def test_empty_rows_yields_nothing(self):
        self.assertEqual(list(csv_chunks([])), [])

    def test_header_and_rows_round_trip(self):
        rows = [
            {"affiliate_id": "1", "amount": "10.50"},
            {"affiliate_id": "2", "amount": "-3.00"},
        ]
        chunks = list(csv_chunks(rows))
        self.assertEqual(len(chunks), 1)
        parsed = list(csv.reader(io.StringIO(chunks[0])))
        self.assertEqual(parsed[0], ["affiliate_id", "amount"])
        self.assertEqual(parsed[1], ["1", "10.50"])
        # Negative numeric survives as a number, not as an escaped string.
        self.assertEqual(parsed[2], ["2", "-3.00"])

    def test_embedded_delimiter_and_quote_are_quoted(self):
        rows = [{"name": 'a,b"c', "note": "line1"}]
        text = list(csv_chunks(rows))[0]
        parsed = list(csv.reader(io.StringIO(text)))
        self.assertEqual(parsed[1], ['a,b"c', "line1"])

    def test_injection_survives_the_round_trip_escaped(self):
        rows = [{"name": '=HYPERLINK("http://evil","click")'}]
        text = list(csv_chunks(rows))[0]
        parsed = list(csv.reader(io.StringIO(text)))
        self.assertTrue(parsed[1][0].startswith("'="), parsed[1][0])

    def test_none_becomes_empty_cell(self):
        rows = [{"a": None, "b": "x"}]
        parsed = list(csv.reader(io.StringIO(list(csv_chunks(rows))[0])))
        self.assertEqual(parsed[1], ["", "x"])


if __name__ == "__main__":
    unittest.main()
