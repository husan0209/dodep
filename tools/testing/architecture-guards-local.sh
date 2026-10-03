#!/usr/bin/env bash
# Local mirror of .github/workflows/architecture-guards.yml (G1-G4).
# Run inside a clean worktree so unrelated in-progress edits cannot be blamed
# on this branch.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1

rc=0

echo "=== G1: no float money fields ==="
if grep -rnE "balance.*(f32|f64)" services/rust/wallet-core/src/ 2>/dev/null; then
  echo "G1 FAIL"; rc=1
else echo "G1 OK"; fi
if grep -rnE "balance.*(float32|float64)" services/go/payment/ 2>/dev/null; then
  echo "G1 FAIL"; rc=1
else echo "G1 OK"; fi

echo "=== G2: fmt.Println in Go (non-test) ==="
p="$(grep -rn 'fmt.Println(' services/go/ 2>/dev/null | grep -v '_test.go' | head -5)"
if [ -n "$p" ]; then echo "$p"; else echo "G2 OK"; fi

echo "=== G3: hardcoded private keys ==="
# Mirrors the repaired upstream guard: tracked files only, excluding this
# workflow's own search strings and sanitised documentation examples.
k="$(git grep -n -I -e 'BEGIN RSA PRIVATE KEY' -e 'BEGIN OPENSSH PRIVATE KEY' \
      -- . \
      ':!.github/workflows/architecture-guards.yml' \
      ':!*.md' 2>/dev/null || true)"
if [ -n "$k" ]; then echo "$k"; echo "G3 FAIL"; rc=1; else echo "G3 OK"; fi

echo "=== G4: @ts-ignore in frontend ==="
t="$(grep -rn '@ts-ignore' apps/ 2>/dev/null | head -5)"
if [ -n "$t" ]; then echo "$t"; echo "G4 FAIL"; rc=1; else echo "G4 OK"; fi

echo "=== guards exit=$rc ==="
exit $rc