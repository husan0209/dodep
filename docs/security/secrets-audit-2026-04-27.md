# Secret Leak Audit — 2026-04-27

**Phase:** 0.2 Stabilization
**Auditor:** AI Agent (read-only sweep)
**Result:** ✅ **No real production secrets found in repository.**

---

## Scope

Audited entire `git ls-files` snapshot **and** historical commits for:

- `.env`, `.env.*` files committed accidentally
- `*.exe`, `*.log`, `*.pem`, `*.key`, `*.crt` binaries / private keys
- Helm `secrets.yaml` templates with plaintext defaults
- Kubernetes `Secret` manifests
- Hard-coded credentials in source code (Stripe `sk_live_*`, AWS `AKIA*`, GitHub `ghp_*`, NOWPayments `nowp_*`)
- Private RSA / EC / OpenSSH keys
- `password = "..."` patterns with high-entropy values

## Findings

### 1. `.gitignore` coverage — ✅ adequate

Existing rules block: `.env`, `.env.local`, `.env.*.local`, `*.env`, `*.log`, `*.exe`, `*.test`, `*.pem`, `*.key`, `*.crt`, `secrets/`, `vault-keys/`, `infra/k8s/secrets.yaml`.

### 2. Current `git ls-files` — ✅ clean

No `.env`, `.exe`, `.log` in tracked files. Only `*.env.example` templates (intended).

### 3. Helm secrets templates — ✅ templated, no plaintext

| File | Status |
|---|---|
| `infra/helm/charts/casino/templates/secrets.yaml` | Uses `{{ .Values.env \| dig ... \| default "" \| quote }}`. Defaults are dummy localhost / empty. |
| `infra/helm/charts/fraud-ml/templates/secrets.yaml` | Uses `{{ .Values.ml.clickhouse.X \| default "" \| quote }}`. Defaults empty. |
| `infra/helm/charts/notification/templates/secrets.yaml` | Uses `{{ .Values.env \| dig ... \| default "" \| quote }}`. Defaults empty. |

All real values are injected at deploy time via `--set` or `values-prod.yaml` (out of repo).

### 4. `infra/k8s/secrets.yaml` git history — ✅ only placeholders

```yaml
# git show 69fef03:infra/k8s/secrets.yaml
POSTGRES_PASSWORD: "CHANGE_ME_IN_PRODUCTION"
JWT_SECRET_KEY:    "CHANGE_ME_IN_PRODUCTION_USE_32_BYTES"
JWT_REFRESH_SECRET:"CHANGE_ME_IN_PRODUCTION_USE_32_BYTES"
SUMSUB_API_KEY:    "CHANGE_ME"
STRIPE_SECRET_KEY: "CHANGE_ME"
ENCRYPTION_KEY:    "CHANGE_ME_IN_PRODUCTION_USE_32_BYTES"
```

The file is now in `.gitignore` (since commit `0ad9f47`). All historical values are explicit `CHANGE_ME` placeholders. **No real secret was ever committed**, therefore `git filter-repo` is **not required**.

### 5. Hard-coded high-entropy credentials — ✅ none

`grep` for live API key formats (Stripe, AWS, GitHub, NOWPayments) returned **0 matches** in production code paths.

### 6. `password = "..."` patterns — ⚠️ test fixtures only

5 matches across 3 files, all in `tools/testing/k6/` load-test scripts:

| File | Value |
|---|---|
| `tools/testing/load-test.js:42` | `'testpassword'` |
| `tools/testing/k6/scenarios/10m-users.js:176,180,185` | `'TestPassword123!'` |
| `tools/testing/k6/k6-tests.yaml:115` | `'testpassword123'` |

These are **synthetic test passwords for k6 load fixtures**, not real credentials. Acceptable.

## Conclusion

**No mitigation required for Phase 0.2.** No `git filter-repo`, no key rotation.

## Preventive measures applied

| Layer | Tool | Status |
|---|---|---|
| CI | `gitleaks` in `.github/workflows/security-scan.yml` | ✅ already configured (push/PR/daily) |
| CI | Semgrep `p/secrets` ruleset | ✅ already configured |
| Pre-commit | Git hook in `.githooks/pre-commit` | ✅ added in this audit (opt-in) |
| Documentation | `docs/security/pre-commit-setup.md` | ✅ added in this audit |

## Recommendations (low priority, not Phase 0)

1. Add `gitleaks.toml` config tuned for project (allowlist test fixtures).
2. Adopt SOPS or sealed-secrets for `infra/k8s/` once real secrets management is needed pre-prod.
3. Audit Vault / external secret store (out of repo) before first prod deploy.

---

**Sign-off:** Phase 0.2 closed. No secret leak. Move on to Phase 0.4 (CORS hardening).
