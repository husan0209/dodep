# Pre-commit Hook Setup

Opus Casino ships an **opt-in** pre-commit hook (`.githooks/pre-commit`) that runs [gitleaks](https://github.com/gitleaks/gitleaks) against staged changes. This catches accidental secret commits **before** they reach the remote.

## One-time setup

### 1. Install gitleaks

```bash
# macOS (Homebrew)
brew install gitleaks

# Windows (Scoop)
scoop install gitleaks

# Windows (winget)
winget install --id gitleaks.gitleaks

# Linux (manual)
curl -sSfL https://raw.githubusercontent.com/gitleaks/gitleaks/master/install.sh | sh -s -- -b $HOME/.local/bin
```

Verify:

```bash
gitleaks version  # expect 8.x or newer
```

### 2. Activate the hook for this repo

From repo root:

```bash
git config core.hooksPath .githooks
```

That tells Git to look for hooks in `.githooks/` instead of the default `.git/hooks/`. The hook is now active **only for this clone** — collaborators must run the same command on their machines.

### 3. Verify

Stage a file with a fake key and try to commit:

```bash
echo 'AWS_KEY="AKIAIOSFODNN7EXAMPLE"' > /tmp/test.env
git add /tmp/test.env
git commit -m "test"
# Expected: ❌ Potential secret detected
```

Then revert: `git reset HEAD /tmp/test.env && rm /tmp/test.env`.

## What the hook does

- Runs `gitleaks protect --staged --redact` against the diff being committed.
- If gitleaks is **not installed**, the hook prints a warning and exits 0 (does not block).
- If a secret is detected, the commit is **rejected** with redacted details.
- Bypass with `git commit --no-verify` (emergency only — leaves a paper trail in CI which still runs gitleaks on push).

## False positives

If gitleaks flags a legitimate file (e.g. test fixtures with synthetic passwords), add an allowlist to `.gitleaks.toml`:

```toml
[allowlist]
paths = [
  '''tools/testing/k6/.*''',
  '''.*\.test\.go$''',
]
```

See [gitleaks docs](https://github.com/gitleaks/gitleaks#configuration) for full grammar.

## Defense in depth

| Layer | When it runs | Tool | Blocks merge? |
|---|---|---|---|
| Pre-commit (this hook) | local `git commit` | gitleaks (staged) | yes (opt-in per dev) |
| CI on push/PR | GitHub Actions | gitleaks (full repo) | yes |
| CI scheduled | daily 00:00 UTC | gitleaks + Semgrep `p/secrets` | reports only |

The CI layers are mandatory and cannot be bypassed.
