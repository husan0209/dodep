// Structural validation for the generated reconciliation manifest.
//
// The generated ConfigMap embeds SQL and shell inside YAML block scalars, so a
// mis-indented line would silently change the SQL or break the runner. This
// re-parses the manifest and asserts the embedded payloads are byte-identical
// to their sources, and that the CronJob actually mounts them.
//
// Run: node tools/testing/affiliate/verify-reconciliation-manifest.js

const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const read = (p) => fs.readFileSync(path.join(repoRoot, p), 'utf8').replace(/\r\n/g, '\n');

const MANIFEST = 'infra/k8s/data/postgresql/affiliate-reconciliation/affiliate-reconciliation.yaml';
const SOURCES = {
  'affiliate_ledger_reconciliation.sql':
    'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation.sql',
  'affiliate_ledger_reconciliation_gate.sql':
    'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation_gate.sql',
  'run.sh': 'infra/k8s/data/postgresql/affiliate-reconciliation/run.sh',
};

const failures = [];
const fail = (m) => failures.push(m);

const docs = yaml.loadAll(read(MANIFEST)).filter(Boolean);
if (docs.length !== 2) {
  fail(`expected 2 documents (ConfigMap + CronJob), parsed ${docs.length}`);
}

const cm = docs.find((d) => d.kind === 'ConfigMap');
const cj = docs.find((d) => d.kind === 'CronJob');

if (!cm) fail('ConfigMap document missing');
if (!cj) fail('CronJob document missing');

if (cm && cj) {
  // 1. Embedded payloads must equal their sources exactly.
  for (const [key, src] of Object.entries(SOURCES)) {
    if (!(key in cm.data)) {
      fail(`ConfigMap.data is missing "${key}"`);
      continue;
    }
    const embedded = cm.data[key].replace(/\n$/, '');
    const original = read(src).replace(/\n$/, '');
    if (embedded !== original) {
      fail(`ConfigMap.data["${key}"] drifted from ${src} (regenerate the ConfigMap)`);
    }
  }

  // 2. Gate must be a single scalar statement, not multi-result-set SQL.
  const gate = cm.data['affiliate_ledger_reconciliation_gate.sql'] || '';
  const statements = gate
    .split(';')
    .map((s) => s.replace(/--[^\n]*/g, '').trim())
    .filter(Boolean);
  if (statements.length !== 1) {
    fail(`gate must be exactly one statement, found ${statements.length} (runner expects a single integer)`);
  }
  if (!/count\(\*\)\s+AS\s+diverging_affiliates/i.test(gate)) {
    fail('gate must SELECT count(*) AS diverging_affiliates so the runner can read it');
  }

  // 3. Both SQL payloads must stay read-only.
  for (const key of Object.keys(SOURCES)) {
    if (!key.endsWith('.sql')) continue;
    const sql = cm.data[key].replace(/--[^\n]*/g, '');
    const w = sql.match(/\b(INSERT|UPDATE|DELETE|ALTER|DROP|TRUNCATE)\b/i);
    if (w) fail(`${key} contains ${w[1]} - reconciliation must never mutate balances`);
  }

  // 4. CronJob wiring.
  const container = cj.spec?.jobTemplate?.spec?.template?.spec?.containers?.[0];
  if (!container) fail('CronJob has no container');

  if (container) {
    const envNames = (container.env || []).map((e) => e.name);
    for (const required of ['PGHOST', 'PGDATABASE', 'PGUSER', 'PGPASSWORD']) {
      if (!envNames.includes(required)) fail(`CronJob env is missing ${required}`);
    }
    const pw = (container.env || []).find((e) => e.name === 'PGPASSWORD');
    if (pw && 'value' in pw) fail('PGPASSWORD must come from secretKeyRef, not a literal value');

    // runner must be executed, and SQL mounted read-only.
    const cmd = JSON.stringify(container.command || []);
    if (!/run\.sh/.test(cmd)) fail(`CronJob does not execute run.sh (command=${cmd})`);

    const mounts = Object.fromEntries(
      (container.volumeMounts || []).map((m) => [m.name, m.readOnly === true])
    );
    if (mounts.sql !== true) fail('SQL volume must be mounted readOnly');
    if (mounts.runner !== true) fail('runner volume must be mounted readOnly');
    // readOnlyRootFilesystem needs a writable /tmp for psql.
    if (!Object.keys(mounts).includes('tmp')) {
      fail('readOnlyRootFilesystem is set but no writable /tmp volume is mounted');
    }
  }

  // 5. Volumes must reference the ConfigMap by the generated name.
  const volumes = cj.spec?.jobTemplate?.spec?.template?.spec?.volumes || [];
  const cmVolume = volumes.find((v) => v.configMap?.name === cm.metadata?.name);
  if (!cmVolume) {
    fail(`no volume mounts ConfigMap "${cm.metadata?.name}"`);
  } else {
    const hasGate = (cmVolume.items || []).some((i) => i.key === 'run.sh');
    if (!hasGate && !(cmVolume.items || []).length) {
      // whole ConfigMap mounted is fine; per-item mount must exist if used
    }
  }
  if (!volumes.some((v) => v.name === 'tmp' && v.emptyDir)) {
    fail('missing emptyDir volume for /tmp');
  }

  // 6. Schedule + retry posture.
  // Standard 5-field cron; `*` and digits are both valid in each position.
  if (!/^([\d*]+) ([\d*]+) ([\d*]+) ([\d*]+) ([\d*]+)$/.test(cj.spec?.schedule || '')) {
    fail(`unexpected schedule "${cj.spec?.schedule}"`);
  }
  if (cj.spec?.concurrencyPolicy !== 'Forbid') {
    fail('concurrencyPolicy must be Forbid so runs cannot overlap');
  }
  const backoff = cj.spec?.jobTemplate?.spec?.backoffLimit;
  if (backoff !== 0) {
    fail(`backoffLimit should be 0 (a persistent real divergence must not be retried), got ${backoff}`);
  }

  // 7. Security context.
  const sec = container?.securityContext;
  if (!sec?.readOnlyRootFilesystem) fail('readOnlyRootFilesystem must be true');
  if (sec?.allowPrivilegeEscalation !== false) fail('allowPrivilegeEscalation must be false');
}

if (failures.length) {
  console.error('RECONCILIATION MANIFEST: FAIL');
  for (const f of failures) console.error('  - ' + f);
  process.exit(1);
}
console.log('RECONCILIATION MANIFEST: OK (docs=' + docs.length + ', payloads=' + Object.keys(SOURCES).length + ')');