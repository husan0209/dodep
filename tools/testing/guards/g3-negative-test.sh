#!/usr/bin/env bash
# Proves the repaired G3 secret guard still FAILS on a real key.
#
# The guard was previously red on main for the wrong reason: it matched its own
# search strings and sanitised documentation examples. Narrowing a security check
# is only acceptable if it still catches the thing it exists to catch - that is
# what this asserts.
#
# Run: bash tools/testing/guards/g3-negative-test.sh
set -uo pipefail
cd "$(dirname "$0")/../../.." || exit 1

EXCLUDES=(
  ':!.github/workflows/architecture-guards.yml'
  ':!*.md'
)

scan() {
  git grep -n -I -e 'BEGIN RSA PRIVATE KEY' -e 'BEGIN OPENSSH PRIVATE KEY' \
    -- . "${EXCLUDES[@]}" 2>/dev/null || true
}

fails() {
  # Truthy only when there is actual output. Do NOT use `wc -l` here:
  # `echo ""` is one line, so an empty result would count as a hit.
  [ -n "$1" ]
}

rc=0

echo "--- case 1: real key committed in a source file must FAIL ---"
probe="services/go/securityprobe_tmp.go"
cat > "$probe" <<'EOF'
package main

// -----BEGIN RSA PRIVATE KEY-----
// MIIEowIBAAKCAQEAxFAKEKEYMATERIALFORGUARDTESTONLYnotarealkey0000000
// -----END RSA PRIVATE KEY-----
func main() {}
EOF
git add "$probe"
out="$(scan)"
if fails "$out"; then
  echo "  TRIPS: $(echo "$out" | head -1)"
else
  echo "  VACUOUS: guard missed a real key in a .go file"
  rc=1
fi
git rm -q --cached "$probe" >/dev/null 2>&1
rm -f "$probe"

echo "--- case 2: real SSH key in a yaml config must FAIL ---"
probe="infra/securityprobe_tmp.yaml"
cat > "$probe" <<'EOF'
some: value
ssh_key: |
  -----BEGIN OPENSSH PRIVATE KEY-----
  b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtfake
  -----END OPENSSH PRIVATE KEY-----
EOF
git add "$probe"
out="$(scan)"
if fails "$out"; then
  echo "  TRIPS: $(echo "$out" | head -1)"
else
  echo "  VACUOUS: guard missed a real key in a .yaml file"
  rc=1
fi
git rm -q --cached "$probe" >/dev/null 2>&1
rm -f "$probe"

echo "--- case 3: documentation example must NOT trip the guard ---"
probe="docs/securityprobe_tmp.md"
cat > "$probe" <<'EOF'
Example only:
    -----BEGIN OPENSSH PRIVATE KEY-----
EOF
git add "$probe"
out="$(scan)"
if fails "$out"; then
  echo "  VACUOUS: docs exclusion is not working, $(echo "$out" | head -1)"
  rc=1
else
  echo "  OK: docs example ignored (intentional)"
fi
git rm -q --cached "$probe" >/dev/null 2>&1
rm -f "$probe"

echo "--- case 4: clean tree must pass ---"
out="$(scan)"
if fails "$out"; then
  echo "  FAIL: guard is still red on a clean tree:"
  echo "$out" | head -5
  rc=1
else
  echo "  OK: no hits on a clean tree"
fi

echo "=== g3 negative test exit=$rc ==="
exit $rc