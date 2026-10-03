// Negative tests for verify-reconciliation-manifest.js.
// A guard that cannot fail is worse than no guard, so each rule is proven
// to actually trip before we trust it.

const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const MANIFEST = path.join(repoRoot, 'infra/k8s/data/postgresql/affiliate-reconciliation/affiliate-reconciliation.yaml');
const CHECK = path.join(repoRoot, 'tools/testing/affiliate/verify-reconciliation-manifest.js');

const original = fs.readFileSync(MANIFEST, 'utf8');
const run = () => {
  try {
    const out = execFileSync(process.execPath, [CHECK], { cwd: repoRoot, env: { ...process.env, NODE_PATH: path.join(repoRoot, 'node_modules') } });
    return { code: 0, out: out.toString() };
  } catch (e) {
    return { code: e.status === undefined ? -1 : e.status, out: (e.stdout || '').toString() + (e.stderr || '').toString() };
  }
};

const MUTATIONS = [
  {
    name: 'PGPASSWORD as literal value',
    apply: (m) =>
      m.replace(
        /- name: PGPASSWORD\n\s+valueFrom:\n\s+secretKeyRef:\n\s+name: postgresql-secrets\n\s+key: postgres-password/,
        '- name: PGPASSWORD\n                  value: hunter2'
      ),
  },
  {
    name: 'SQL volume mounted writable',
    apply: (m) => m.replace('- name: sql\n                  mountPath: /sql\n                  readOnly: true', '- name: sql\n                  mountPath: /sql'),
  },
  {
    name: 'readOnlyRootFilesystem disabled',
    apply: (m) => m.replace('readOnlyRootFilesystem: true', 'readOnlyRootFilesystem: false'),
  },
  { name: 'overlapping runs allowed', apply: (m) => m.replace('concurrencyPolicy: Forbid', 'concurrencyPolicy: Allow') },
  { name: 'ConfigMap payload tampered', apply: (m) => m.replace('WITH derived AS (', 'WITH derived AS ( -- tampered') },
  // NOTE: mutations that edit ConfigMap payload content must keep the YAML block
  // scalar indentation (4 spaces), otherwise they trip the YAML parser instead of
  // the rule under test — a false proof.
  {
    name: 'gate split into two statements',
    apply: (m) =>
      m.replace(
        '    SELECT count(*) AS diverging_affiliates',
        '    SELECT 1;\n    SELECT count(*) AS diverging_affiliates'
      ),
  },
  {
    name: 'mutation in the gate SQL',
    apply: (m) =>
      m.replace(
        '    SELECT count(*) AS diverging_affiliates',
        '    UPDATE affiliate_ledger_accounts SET balance = 0;\n    SELECT count(*) AS diverging_affiliates'
      ),
  },
  { name: '/tmp volume removed', apply: (m) => m.replace('                - name: tmp\n                  mountPath: /tmp', '                - name: gone\n                  mountPath: /tmp') },
  { name: 'runner no longer executed', apply: (m) => m.replace('command: ["/bin/sh", "/runner/run.sh"]', 'command: ["/bin/sh", "-c", "true"]') },
];

let failures = 0;

// Some rules can only be proven by mutating the SOURCE and regenerating the
// ConfigMap: editing the manifest directly just trips the drift check first,
// which would prove nothing about the rule itself.
const GENERATOR = path.join(repoRoot, 'tools/testing/affiliate/generate-reconciliation-configmap.js');
const SOURCE_MUTATIONS = [
  {
    name: 'gate returns two statements (regenerated)',
    file: 'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation_gate.sql',
    apply: (c) => c.replace('SELECT count(*) AS diverging_affiliates', 'SELECT 1;\nSELECT count(*) AS diverging_affiliates'),
  },
  {
    name: 'gate writes to the ledger (regenerated)',
    file: 'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation_gate.sql',
    apply: (c) => c.replace('SELECT count(*) AS diverging_affiliates', 'UPDATE affiliate_ledger_accounts SET balance = 0;\nSELECT count(*) AS diverging_affiliates'),
  },
  {
    name: 'gate stops aliasing the count (regenerated)',
    file: 'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation_gate.sql',
    apply: (c) => c.replace('count(*) AS diverging_affiliates', 'count(*)'),
  },
];

for (const m of SOURCE_MUTATIONS) {
  const abs = path.join(repoRoot, m.file);
  const originalSrc = fs.readFileSync(abs, 'utf8');
  const mutated = m.apply(originalSrc);
  if (mutated === originalSrc) {
    console.error(`  MISS  ${m.name} — source mutation did not apply`);
    failures++;
    continue;
  }
  fs.writeFileSync(abs, mutated, 'utf8');
  execFileSync(process.execPath, [GENERATOR], { cwd: repoRoot });
  const r = run();
  fs.writeFileSync(abs, originalSrc, 'utf8');
  execFileSync(process.execPath, [GENERATOR], { cwd: repoRoot });
  fs.writeFileSync(MANIFEST, original, 'utf8');
  if (r.code === 0) {
    console.error(`  VACUOUS  ${m.name} — guard still passed`);
    failures++;
  } else {
    const reason = (r.out.split('\n').find((l) => l.trim().startsWith('- ')) || '').trim();
    console.log(`  TRIPS  ${m.name} -> ${reason}`);
  }
}

if (run().code !== 0) {
  console.error('FATAL: manifest was not restored after source mutations');
  process.exit(1);
}

// Baseline: unmutated manifest must pass.
const base = run();
if (base.code !== 0) {
  console.error('BASELINE FAILED — guard rejects the real manifest:\n' + base.out);
  process.exit(1);
}
console.log('baseline: PASS');

for (const m of MUTATIONS) {
  const mutated = m.apply(original);
  if (mutated === original) {
    console.error(`  MISS  ${m.name} — mutation did not apply (pattern stale)`);
    failures++;
    continue;
  }
  fs.writeFileSync(MANIFEST, mutated, 'utf8');
  const r = run();
  fs.writeFileSync(MANIFEST, original, 'utf8');
  if (r.code === 0) {
    console.error(`  VACUOUS  ${m.name} — guard still passed`);
    failures++;
  } else {
    const reason = (r.out.split('\n').find((l) => l.trim().startsWith('- ')) || '').trim();
    console.log(`  TRIPS  ${m.name} -> ${reason}`);
  }
}

if (failures) {
  console.error(`\n${failures} guard rule(s) are vacuous or stale`);
  process.exit(1);
}
console.log(`\nall ${MUTATIONS.length} guard rules verified to trip`);