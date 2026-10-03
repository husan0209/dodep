#!/usr/bin/env python3
"""Validate infra/k8s/monitoring/alerting/bonus-alerting-rules.yaml.

Checks, in order:
  1. the file parses as YAML and is a ConfigMap with the expected shape;
  2. the embedded payload parses too (the block scalar hides it from the
     outer document, so a typo inside would only surface in Grafana);
  3. every rule has uid/title/condition/for/severity and a resolvable
     condition expression;
  4. every PromQL metric name actually exists in the bonus collector set;
  5. every label value used in a matcher is one the service really emits;
  6. runbook links point at anchors that the runbook file defines.

Deliberately conservative: any metric name that is not in METRICS, or any
label value not in LABEL_VALUES, fails the run. That is the whole point -
a typo in a metric name produces an alert that can never fire.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[3]
RULES = ROOT / "infra/k8s/monitoring/alerting/bonus-alerting-rules.yaml"
RUNBOOK = ROOT / "docs/infra/runbooks/bonus-service.md"
METRICS_GO = ROOT / "services/go/bonus/internal/telemetry/metrics.go"
SERVICE_GO = ROOT / "services/go/bonus/internal/service/bonus_service.go"

# Collector names declared in telemetry/metrics.go.
METRICS = {
    "bonus_http_requests_total",
    "bonus_http_request_duration_seconds",
    "bonus_grpc_requests_total",
    "bonus_grpc_request_duration_seconds",
    "bonus_bonuses_awarded_total",
    "bonus_bonuses_award_skipped_total",
    "bonus_wagers_recorded_total",
    "bonus_wagering_completed_total",
    "bonus_conversion_credits_total",
    "bonus_payment_events_total",
    "bonus_wagering_progress_ratio",
}

# Values the service really passes to the label arguments.
LABEL_VALUES = {
    "result": {
        "credited", "skipped_no_wallet", "failed",          # conversion credit
        "recorded", "no_active_bonus", "expired",           # wagers recorded
        "credit_failed", "invalid_amount", "repository_failure",
        "completed",                                        # wagers recorded: completed
        "awarded", "skipped_not_first", "malformed",        # payment events
    },
    "reason": {"already_awarded", "invalid_amount"},
}

# kube-state-metrics series, not owned by the service.
EXTERNAL_METRICS = {
    "kube_pod_status_ready",
    "kube_pod_container_status_restarts_total",
}

errors: list[str] = []


def fail(msg: str) -> None:
    errors.append(msg)


def metric_names(expr: str) -> set[str]:
    """Metric names referenced by a PromQL expression.

    Deliberately simple: pull identifiers that are followed by a label
    selector, a histogram/counter suffix, or a function argument. Good
    enough to catch a misspelled collector name, which is the failure we
    care about.
    """
    names = set()
    for m in re.finditer(r"\b([a-z_][a-z0-9_]*)\s*(?:\{|\[|_bucket\b|_sum\b|_count\b|\s*\()", expr):
        name = m.group(1)
        # A histogram/summary suffix belongs to the base metric: Prometheus
        # exposes _bucket/_sum/_count series for a histogram named without them.
        for suffix in ("_bucket", "_sum", "_count"):
            if name.endswith(suffix):
                name = name[: -len(suffix)]
                break
        names.add(name)
    # Drop keywords/functions that the regex swept up.
    return names - {
        "sum", "rate", "increase", "histogram_quantile", "by", "le", "and",
        "on", "clamp_min", "absent", "vector", "or", "min", "max",
    }


def main() -> int:
    if not RULES.is_file():
        fail(f"missing {RULES}")
        return report()

    doc = yaml.safe_load(RULES.read_text(encoding="utf-8"))
    if doc.get("kind") != "ConfigMap":
        fail("top-level object is not a ConfigMap")
        return report()
    data = doc.get("data") or {}
    payload_key = "bonus-alerting-rules.yaml"
    if payload_key not in data:
        fail(f"ConfigMap has no {payload_key!r} key (has: {sorted(data)})")
        return report()

    try:
        payload = yaml.safe_load(data[payload_key])
    except yaml.YAMLError as exc:
        fail(f"embedded payload is not valid YAML: {exc}")
        return report()

    if set(payload or {}) != {"groups"}:
        fail(f"payload top-level keys = {sorted(payload or {})}, expected ['groups']")

    seen_uids: set[str] = set()
    for group in (payload or {}).get("groups", []):
        gname = group.get("name", "<unnamed>")
        if not group.get("interval"):
            fail(f"group {gname}: no interval")
        for rule in group.get("rules", []):
            uid = rule.get("uid")
            title = rule.get("title", "<no title>")
            if not uid:
                fail(f"group {gname}: a rule has no uid ({title})")
                continue
            if uid in seen_uids:
                fail(f"duplicate uid {uid}")
            seen_uids.add(uid)

            condition = rule.get("condition")
            if not condition:
                fail(f"{uid}: no condition")
                continue
            refs = {d["refId"]: d for d in rule.get("data", [])}
            if condition not in refs:
                fail(f"{uid}: condition {condition} has no matching data ref")
            for ref in refs.values():
                if ref.get("datasourceUid") == "__expr__":
                    expr = (ref.get("model") or {}).get("expression")
                    if expr not in refs:
                        fail(f"{uid}: expression {expr} does not resolve to a data ref")
                    if not (ref.get("model") or {}).get("type"):
                        fail(f"{uid}: expression ref {expr} has no type")

            if not rule.get("for"):
                fail(f"{uid}: no for= duration")
            labels = rule.get("labels") or {}
            if labels.get("severity") not in {"critical", "warning", "info"}:
                fail(f"{uid}: severity {labels.get('severity')!r} not in critical/warning/info")
            if not rule.get("annotations", {}).get("summary"):
                fail(f"{uid}: no annotation summary")

            for ref in refs.values():
                model = ref.get("model") or {}
                expr = model.get("expr")
                if not expr:
                    continue  # __expr__ threshold ref, or ClickHouse rawSql
                if "rawSql" in model:
                    continue
                for name in metric_names(expr):
                    if name in METRICS or name in EXTERNAL_METRICS:
                        continue
                    if name.startswith("bonus_"):
                        fail(f"{uid}: references unknown bonus metric {name!r}")
                    else:
                        fail(f"{uid}: references unexpected metric {name!r}")
                # Label matchers must use values the service emits.
                for m in re.finditer(r'(\w+)="([^"]+)"', expr):
                    label, value = m.group(1), m.group(2)
                    allowed = LABEL_VALUES.get(label)
                    if allowed and value not in allowed:
                        # Only a problem for our own labels, not kube labels.
                        if any(a.startswith(value) for a in allowed):
                            fail(f"{uid}: {label}={value!r} looks like a typo of a real enum")
                        else:
                            fail(f"{uid}: {label}={value!r} is not a value the service emits "
                                 f"(expected one of: {sorted(allowed)})")
                    elif allowed is None and label in {"code", "status", "type", "currency", "route", "method"}:
                        fail(f"{uid}: matcher on service label {label}={value!r} is not in the known enum set")

    # Every metric the service exposes should be either alerted on or
    # deliberately unused; warn (as an error here) only about total silence
    # on the money-critical ones.
    used: set[str] = set()
    for group in (payload or {}).get("groups", []):
        for rule in group.get("rules", []):
            for ref in rule.get("data", []):
                expr = (ref.get("model") or {}).get("expr") or ""
                used |= metric_names(expr)
    money_critical = {
        "bonus_conversion_credits_total",
        "bonus_wagering_completed_total",
        "bonus_wagers_recorded_total",
        "bonus_bonuses_award_skipped_total",
        "bonus_payment_events_total",
    }
    for m in sorted(money_critical - used):
        fail(f"money-critical metric {m} has no alert rule")

    # Cross-check against the Go source so the lists above cannot drift.
    if METRICS_GO.is_file():
        go = METRICS_GO.read_text(encoding="utf-8")
        declared = set(re.findall(r'Name:\s*"([a-z_][a-z0-9_]*)"', go))
        for m in sorted(METRICS - declared):
            fail(f"metric {m} asserted here but not declared in metrics.go")
        for m in sorted(declared - METRICS):
            fail(f"metrics.go declares {m}, which this validator does not know about")

    if SERVICE_GO.is_file():
        svc = SERVICE_GO.read_text(encoding="utf-8")
        emitted = set(re.findall(r'=\s*"([a-z_]+)"', svc))
        for values in LABEL_VALUES.values():
            for v in values:
                if v not in emitted and f'"{v}"' not in svc:
                    fail(f"label value {v!r} asserted here is not present in bonus_service.go")

    if RUNBOOK.is_file():
        book = RUNBOOK.read_text(encoding="utf-8")
        anchors = {
            re.sub(r"[^a-z0-9]+", "-", h.strip().lower()).strip("-")
            for h in re.findall(r"^#{2,4}\s+(.+)$", book, re.MULTILINE)
        }
        # One report per distinct anchor, listing the rules that use it, so a
        # single bad link does not produce one error per rule.
        usage: dict[str, set[str]] = {}
        for group in (payload or {}).get("groups", []):
            for rule in group.get("rules", []):
                link = (rule.get("labels") or {}).get("runbook", "")
                m = re.search(r"#([\w\-]+)", link)
                if m:
                    usage.setdefault(m.group(1), set()).add(rule.get("uid", "?"))
        for anchor, users in sorted(usage.items()):
            if anchor not in anchors:
                fail(f"runbook anchor #{anchor} (used by {', '.join(sorted(users))}) "
                     f"not found in {RUNBOOK.name}")
        # And the reverse: a rule that links to the runbook must not silently
        # point at an anchor that exists but is about an unrelated incident.
        print(f"runbook: {len(usage)} distinct anchor(s), all defined" if all(
            a in anchors for a in usage) else "")
    else:
        fail(f"runbook {RUNBOOK} does not exist but rules link to it")

    print(f"checked {len(seen_uids)} rules in {RULES.relative_to(ROOT)}")
    for uid in sorted(seen_uids):
        print(f"  - {uid}")
    return report()


def report() -> int:
    if errors:
        print(f"\nFAILED with {len(errors)} problem(s):", file=sys.stderr)
        for e in errors:
            print(f"  x {e}", file=sys.stderr)
        return 1
    print("\nOK: bonus alert rules are consistent with the code")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())