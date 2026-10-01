// Affiliate consistency guard.
//
// Catches drift across the three places that must agree for affiliate money to
// be trustworthy, but that no compiler or test currently connects:
//
//   1. libs/migrations/postgresql/queries/affiliate_ledger_reconciliation.sql
//   2. libs/migrations/postgresql/021_affiliates.sql        (enums, tables)
//   3. services/go/affiliate/internal/repository/ledger.go   (derived model)
//
// The failure mode this prevents: someone adds a payout status to the enum and
// the reconciliation silently stops counting it, so the ledger looks balanced
// while money is actually missing (or double-counted).
//
// Run: node tools/testing/affiliate/consistency-check.js

const fs = require('fs');
const path = require('path');

const repoRoot = path.resolve(__dirname, '..', '..', '..');

// Some inputs are owned by other stages and may not be present in every
// checkout (e.g. the affiliate schema lands with the DATA_ENGINEER stage).
// Missing input => SKIP with a clear reason, never a crash and never a silent
// pass, because "no findings" from an absent schema would be a false green.
const DEPENDENCIES = {
  reconSql: 'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation.sql',
  schema: 'libs/migrations/postgresql/021_affiliates.sql',
  ledgerGo: 'services/go/affiliate/internal/repository/ledger.go',
};

const missing = [];
const read = (key) => {
  const p = path.join(repoRoot, DEPENDENCIES[key]);
  if (!fs.existsSync(p)) {
    missing.push(DEPENDENCIES[key]);
    return '';
  }
  return fs.readFileSync(p, 'utf8');
};

const reconSql = read('reconSql');
const schema = read('schema');
const ledgerGo = read('ledgerGo');
// Ledger account type constants live next to the derived model, in ledger.go.
const ledgerAccounts = ledgerGo;

if (missing.length) {
  console.log('AFFILIATE CONSISTENCY: SKIP — missing required input(s):');
  for (const m of missing) console.log('  - ' + m);
  console.log('This guard only runs once the affiliate schema and ledger exist.');
  process.exit(0);
}

const failures = [];
const fail = (msg) => failures.push(msg);

// ---------------------------------------------------------------------------
// 1. Enum members
// ---------------------------------------------------------------------------
const enumMembers = (typeName) => {
  const m = schema.match(
    new RegExp(`CREATE TYPE ${typeName} AS ENUM \\(([^)]*)\\)`, 'i')
  );
  if (!m) {
    fail(`enum ${typeName} not found in 021_affiliates.sql`);
    return [];
  }
  return m[1]
    .split(',')
    .map((s) => s.trim().replace(/^'|'$/g, ''))
    .filter(Boolean);
};

const payoutStatuses = enumMembers('affiliate_payout_status');
const earningStatuses = enumMembers('affiliate_earning_status');
const adjustmentTypes = enumMembers('affiliate_adjustment_type');

if (payoutStatuses.length && adjustmentTypes.length && earningStatuses.length) {
  // Reconciliation must only reference enum members that actually exist.
  const sqlStatuses = new Set(
    [...reconSql.matchAll(/'(requested|reviewing|approved|processing|paid|rejected|failed|accrued|pending|available|reversed|credit|debit)'/g)].map(
      (m) => m[1]
    )
  );
  const known = new Set([...payoutStatuses, ...earningStatuses, ...adjustmentTypes]);
  for (const s of sqlStatuses) {
    if (!known.has(s)) fail(`reconciliation SQL references unknown status "${s}"`);
  }

  // Every payout status must be explicitly accounted for: either it is listed
  // in a status IN (...) predicate (money leaves `available`), or it appears in
  // the documented exclusion list. A status that is silently unmentioned means
  // nobody decided whether it moves money — that is how double-entry drifts.
  const statusInLists = new Set(
    [...reconSql.matchAll(/status\s+(?:NOT\s+)?IN\s*\(([^)]*)\)/g)]
      .flatMap((m) => m[1].match(/'[a-z]+'/g) || [])
      .map((s) => s.replace(/'/g, ''))
  );
  const excluded = new Set(
    [...reconSql.matchAll(/EXCLUDED_STATUSES:\s*([a-z_,\s']+)/g)]
      .flatMap((m) => m[1].match(/'[a-z]+'/g) || [])
      .map((s) => s.replace(/'/g, ''))
  );
  for (const st of payoutStatuses) {
    if (!statusInLists.has(st) && !excluded.has(st)) {
      fail(
        `payout status "${st}" is neither reconciled nor listed as excluded; ` +
          'decide explicitly whether it moves money out of `available`'
      );
    }
  }
  for (const st of excluded) {
    if (!payoutStatuses.includes(st)) {
      fail(`EXCLUDED_STATUSES names "${st}", which is not a payout status`);
    }
  }
}

// ---------------------------------------------------------------------------
// 2. Tables referenced by the SQL must exist in the schema
// ---------------------------------------------------------------------------
// CTE aliases are not tables. Strip SQL comments first, then drop any identifier
// that the script itself defines with `WITH x AS (...)`.
const sqlNoComments = reconSql
  .replace(/--[^\n]*/g, '')
  .replace(/\/\*[\s\S]*?\*\//g, '');

const cteNames = new Set(
  [...sqlNoComments.matchAll(/(?:WITH|,)\s*([a-z_]+)\s+AS\s*\(/gi)].map((m) => m[1].toLowerCase())
);

const tablesInSql = new Set(
  [...sqlNoComments.matchAll(/FROM\s+([a-z_][a-z0-9_]*)/gi)]
    .map((m) => m[1].toLowerCase())
    .filter((t) => !cteNames.has(t))
);
for (const t of tablesInSql) {
  if (!new RegExp(`CREATE TABLE IF NOT EXISTS ${t}\\b`, 'i').test(schema)) {
    fail(`reconciliation SQL references unknown table "${t}"`);
  }
}

// ---------------------------------------------------------------------------
// 3. Every table the Go ledger touches must also be audited by the SQL.
//    If Go derives a balance from a table the SQL ignores, drift is invisible.
// ---------------------------------------------------------------------------
const goTables = new Set(
  [...ledgerGo.matchAll(/FROM\s+(affiliate_[a-z_]+)/g)].map((m) => m[1])
);
for (const t of goTables) {
  if (!tablesInSql.has(t)) {
    fail(`Go derives balances from "${t}" but reconciliation SQL never audits it`);
  }
}

// ---------------------------------------------------------------------------
// 4. Ledger account types: SQL <-> Go constants <-> CHECK/domain values.
// ---------------------------------------------------------------------------
// LedgerAccountPending   = "pending"   (const block in ledger.go)
const goAccountTypes = new Set(
  [...ledgerAccounts.matchAll(/LedgerAccount[A-Z][a-zA-Z]*\s*=\s*"([a-z]+)"/g)].map((m) => m[1])
);
const sqlAccountTypes = new Set(
  [...reconSql.matchAll(/account_type\s*=\s*'([a-z]+)'/g)].map((m) => m[1])
);
for (const t of goAccountTypes) {
  if (!sqlAccountTypes.has(t)) {
    fail(`Go ledger account type "${t}" is not reconciled by the SQL`);
  }
}
for (const t of sqlAccountTypes) {
  if (!goAccountTypes.has(t)) {
    fail(`SQL reconciles account type "${t}" which Go does not define`);
  }
}

// ---------------------------------------------------------------------------
// 5. The SQL must stay read-only. A reconciliation job that can write is a job
//    that can silently "fix" a real divergence instead of alerting on it.
// ---------------------------------------------------------------------------
const writes = reconSql.match(/^\s*(INSERT|UPDATE|DELETE|ALTER|DROP|TRUNCATE)\b/im);
if (writes) fail(`reconciliation SQL is not read-only (found ${writes[1]})`);

// ---------------------------------------------------------------------------
// 6. Double-entry pairing: Go posts debit+credit with a ":credit" idempotency
//    suffix. The SQL must check the same invariant via the direction column,
//    and the schema must actually have that column.
// ---------------------------------------------------------------------------
if (!/direction/.test(schema)) {
  fail('affiliate_ledger_entries has no direction column; double-entry check is impossible');
}
if (!/CASE WHEN direction = 'debit'/.test(reconSql)) {
  fail('double-entry check does not use the direction column');
}

// ---------------------------------------------------------------------------
// 7. Divergence threshold must be present and > 0 (a threshold of 0 would page
//    on float noise; missing would page on everything).
// ---------------------------------------------------------------------------
const thresholds = [...reconSql.matchAll(/>\s*0\.0([0-9]+)/g)].map((m) => parseFloat('0.0' + m[1]));
if (!thresholds.length) {
  fail('no divergence threshold found; expected > 0.01 per architecture-overview RULE 1');
}
if (thresholds.some((t) => t <= 0)) {
  fail('divergence threshold is <= 0, which would page on rounding noise');
}

// ---------------------------------------------------------------------------
if (failures.length) {
  console.error('AFFILIATE CONSISTENCY: FAIL');
  for (const f of failures) console.error('  - ' + f);
  process.exit(1);
}
console.log(
  'AFFILIATE CONSISTENCY: OK (tables=' +
    tablesInSql.size +
    ', accounts=' +
    goAccountTypes.size +
    ', payouts=' +
    payoutStatuses.length +
    ', earnings=' +
    earningStatuses.length +
    ')'
);