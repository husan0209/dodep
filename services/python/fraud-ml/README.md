# Fraud ML Service

Real-time and batch fraud detection for Opus Casino (10M+ users).

Python trains and evaluates models weekly; the exported **ONNX** artifact is
served by the Rust side (< 5 ms inference), so the training pipeline and the
serving contract must agree exactly on features, ordering and threshold.

---

## Architecture

```
ClickHouse user_events ──► FeatureExtractor ──► FraudModel (XGBoost)
       fraud_signals             (src/features)      (src/models)
                                     │                    │
                                     ▼                    ▼
                             FeatureTransformer      quality gates
                             (feature registry)   (src/evaluation)
                                                          │
                                                          ▼
                                          ONNX export + parity gate
                                                (src/export)
                                                          │
                                    ┌─────────────────────┴──────────┐
                                    ▼                                ▼
                          Rust serving (< 5ms)          fraud-ml FastAPI
                          (reads ONNX metadata)          (batch scoring, models)
```

```
main.py                 FastAPI app, lifespan, probes, /metrics
internal/api/routes.py  detection + model-management endpoints
internal/consumers/     Redpanda consumer (streaming events → detectors)
internal/models/        rule-based detectors (IsolationForest / XGBoost)
internal/repository/    ClickHouse access for detector features
src/config.py           settings (quality thresholds, FPR budget, paths)
src/data/               ClickHouse + feature store
src/features/           feature extraction, derived features, registry
src/models/             FraudModel: split, train, persist, export
src/evaluation/         operating-point selection and quality gates
src/export/             ONNX export and parity validation
src/pipeline/           weekly training orchestration
tests/                  unit + contract tests (no live ClickHouse required)
```

## Operating point: why not 0.5?

Fraud base rate is ~1%, so a fixed 0.5 threshold misses almost everything,
while an unconstrained "90 % recall" threshold floods the manual review
queue. The review queue is a **finite resource**, so the service optimizes
the actual business constraint:

> maximize **recall** subject to **FPR ≤ max_fpr** (default 5 %)

This is the Neyman–Pearson formulation, standard in production fraud
systems. `select_threshold_constrained()` returns the threshold achieving
that trade-off; `validate_quality_gates()` blocks promotion when the model
cannot reach a minimum recall *within* the FPR budget.

Thresholds are **selected on the validation split only** and reported on the
held-out test split, so the published numbers are not optimistic.

## Leakage controls

| Risk | Control |
|---|---|
| Future behaviour in training | temporal split by `event_time` (never random) |
| Labels maturing after the split | 1-day `embargo` gap around each boundary |
| Threshold fitted on test | threshold picked on validation, applied to test |
| Class imbalance | `scale_pos_weight` computed from the data, not hard-coded |
| Unstable runs | fixed `random_state`, `tree_method=hist` |

## Artifacts and the serving contract

`TrainPipeline.run()` writes, per run:

| File | Purpose |
|---|---|
| `fraud_model.json` | XGBoost booster |
| `fraud_model.onnx` | serving graph (Rust) |
| `model_card.json` | threshold, feature order, metrics, gates, parity result |

Rule-based detectors in `internal/models/` are a separate, heuristic layer
(IsolationForest / RandomForest / small XGBoost). Their probabilities are
reported on the same 0-100 risk scale, with decision thresholds coming from
`internal/config` settings rather than a hard-coded 0.5.

The ONNX graph embeds its own metadata, so the Rust side reads one artifact:

```json
{
  "model_type": "xgboost_binary_logistic",
  "feature_columns": ["bets_7d", "bets_24h", "..."],
  "threshold": 0.5247,
  "max_fpr": 0.05,
  "threshold_selected_on": "validation",
  "positive_class_index": 1
}
```

Graph signature: input `features [None, 20] float32`, outputs `label [None]`
and `probabilities [None, 2]` (column 1 is `P(fraud)`).

### Parity gate

Before an artifact is uploaded, its ONNX probabilities are compared against
`sklearn`'s `predict_proba` on real held-out feature rows. Promotion fails
if `max |Δp| > 1e-4` or label agreement `< 99.9 %`. This catches the class
of bug where the Rust service silently scores with a different model or a
different threshold.

## Quality gates (tasks/ТЗ.md, Этап 11)

| Gate | Default | Rationale |
|---|---|---|
| `auc_roc` | ≥ 0.90 | discrimination |
| `fpr_at_max_fpr` | ≤ 0.05 | review-queue budget |
| `recall_at_max_fpr` | ≥ 0.70 | detection rate inside the budget |
| `precision_at_90_recall` | ≥ 0.50 | reporting quality |

Tune via environment variables (`MAX_FPR`, `MIN_RECALL_AT_MAX_FPR`,
`MODEL_QUALITY_THRESHOLD`, `PRECISION_THRESHOLD`).

> Accuracy is deliberately **not** a gate: at a 1 % base rate a model can
> reach 99 % accuracy while detecting nothing.

## Running locally

```bash
pip install -r requirements.txt

# API + streaming consumer + batch scoring
uvicorn main:app --port 8000

# Weekly training job
python -m src.pipeline.train_pipeline   # or invoke TrainPipeline().run()
```

Probes: `GET /health` (liveness), `GET /ready` (503 until the detector is
initialized), `GET /metrics` (Prometheus).

Endpoints (prefix `/api/v1`): `POST /detect/bet-anomaly`,
`/detect/bonus-abuse`, `/detect/payment-fraud`, `/detect/account-takeover`,
`/detect/batch`, `GET /models/status`, `POST /models/reload`,
`GET /statistics`.

## Development

```bash
pytest -q        # 100+ tests, no live ClickHouse/Kafka needed
ruff check .
black --check .
mypy .
```

CI (`.github/workflows/ci-python.yml`) runs the same four commands.

### Adding a feature

1. Add it to `FEATURE_REGISTRY` in `src/features/transformation.py` — **append
   at the end**, order is the serving contract.
2. Compute it in `FeatureExtractor.compute_derived_features()` (or the SQL).
3. `MODEL_FEATURES` derives from the registry automatically.
4. Retrain; the new model card and ONNX metadata record the new order, and
   `FraudModel.load()` rejects a card whose order disagrees with the code.

## Graceful degradation

The API is usable without its backing services, because detection runs on
in-memory detectors:

| Dependency | If unavailable |
|---|---|
| ClickHouse | logged as error; `/health` and detection endpoints still work; the weekly training job fails loudly on its own connection |
| Redpanda | consumer start is skipped, `/health` stays green, batch scoring still works |

`/ready` depends only on the fraud detector, so it reflects "can this
instance serve traffic", not "is every downstream healthy".

## Known limitations

- `extract_training_data()` keeps one row per user (`QUALIFY row_number() = 1`),
  so the current model scores **accounts**, not individual events. Per-event
  scoring needs the `QUALIFY` clause dropped and a re-sized split.
- No drift monitoring (PSI / feature distribution checks) is wired yet; add
  it around the weekly run before relying on long-lived models.
- The rule-based detectors are heuristic and are not part of the trained
  artifact; they exist for the streaming path where the batch risk model is
  too coarse.
