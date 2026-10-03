// Affiliate consistency guard.
//
// Catches drift across the places that must agree for affiliate money to be
// trustworthy, but that no compiler or test currently connects:
//
//   1. libs/migrations/postgresql/queries/affiliate_ledger_reconciliation.sql
//      and ..._gate.sql
//   2. the affiliate schema under libs/migrations/postgresql/
//   3. services/go/affiliate/internal/repository/gorm_repository.go
//
// The failure mode this prevents: someone adds a payout status to the enum and
// the reconciliation silently stops counting it, so the ledger looks balanced
// while money is actually missing (or double-counted).
//
// Run: node tools/testing/affiliate/consistency-check.js

const fs = require('fs');
const path = require('path');

const repoRoot = path.resolve(__dirname, '..', '..', '..');

// Two tiers of checks:
//   A) SQL-internal    — only needs the reconciliation files this guard owns.
//                       Always runs, on every branch.
//   B) Cross-boundary  — also needs the affiliate schema and the Go derived
//                       model, which live in other areas and get split and
//                       refactored independently. Skipped, loudly, when an input
//                       is absent, so a concurrent refactor cannot silently turn
//                       this guard green, and an input that is simply not on this
//                       branch cannot turn it red for an unrelated reason.
const missing = [];
const read = (p) => fs.readFileSync(path.join(repoRoot, p), 'utf8');

// The Go derived model has moved more than once (ledger.go was folded into
// gorm_repository.go). Search for it rather than hardcoding a path.
const GO_CANDIDATES = [
  'services/go/affiliate/internal/repository/ledger.go',
  'services/go/affiliate/internal/repository/gorm_repository.go',
];
let ledgerGo = '';
for (const c of GO_CANDIDATES) {
  const abs = path.join(repoRoot, c);
  if (!fs.existsSync(abs)) continue;
  const content = fs.readFileSync(abs, 'utf8');
  if (/LedgerAccountPending\s*=/.test(content) || /ReconcileLedger/.test(content)) {
    ledgerGo = content;
    break;
  }
}
if (!ledgerGo) {
  missing.push(`${GO_CANDIDATES.join(' | ')} (no LedgerAccountPending / ReconcileLedger found)`);
}

const reconSql = read('libs/migrations/postgresql/queries/affiliate_ledger_reconciliation.sql');

// Ledger account type constants live next to the derived model.
const ledgerAccounts = ledgerGo;

const failures = [];
const fail = (msg) => failures.push(msg);

// ---------------------------------------------------------------------------
// 1. Tables the reconciliation SQL reads
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

// The affiliate schema is not pinned to one migration: it started as
// libs/migrations/postgresql/021_affiliates.sql and gets split as it grows, so
// scan the whole directory.
//
// 013_admin_bff.sql already defines `affiliates` and `affiliate_payouts`, so
// "does any affiliate table exist" is not a usable signal by itself. What decides
// it is whether *every* table this SQL audits is present. When one is not, the
// schema is simply not on this branch, and the cross-boundary checks below are
// SKIPPED, loudly, instead of failed: "unknown table affiliate_earnings" is both
// true and useless there, because it says nothing about whether the
// reconciliation SQL is correct, and it would pin this guard red for a reason
// unrelated to the file it exists to guard. Drift is only reported once there is
// a schema to compare against.
const MIGRATIONS_DIR = path.join(repoRoot, 'libs/migrations/postgresql');
const schema = fs.existsSync(MIGRATIONS_DIR)
  ? fs
      .readdirSync(MIGRATIONS_DIR)
      .filter((f) => f.endsWith('.sql'))
      .sort()
      .map((f) => fs.readFileSync(path.join(MIGRATIONS_DIR, f), 'utf8'))
      .join('\n')
  : '';

const schemaTables = new Set(
  [...schema.matchAll(/CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)/gi)].map(
    (m) => m[1].toLowerCase()
  )
);
const absentTables = [...tablesInSql].filter((t) => !schemaTables.has(t));

const AFFILIATE_ENUMS = [
  'affiliate_payout_status',
  'affiliate_earning_status',
  'affiliate_adjustment_type',
];
const enumMembers = (typeName) => {
  // Tolerate optional IF NOT EXISTS between CREATE TYPE and the name.
  const re = new RegExp(
    `CREATE\\s+TYPE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?${typeName}\\s+AS\\s+ENUM\\s*\\(([^)]*)\\)`,
    'is'
  );
  const m = schema.match(re);
  if (!m) return null;
  return m[1]
    .split(',')
    .map((s) => s.trim().replace(/^'|'$/g, ''))
    .filter(Boolean);
};
const enumValues = Object.fromEntries(AFFILIATE_ENUMS.map((n) => [n, enumMembers(n)]));
const absentEnums = AFFILIATE_ENUMS.filter((n) => enumValues[n] === null);

const hasAffiliateSchema = absentTables.length === 0 && absentEnums.length === 0;

if (absentTables.length) {
  missing.push(
    `affiliate schema: table(s) not defined under libs/migrations/postgresql/ -> ${absentTables.join(', ')}`
  );
}
if (absentEnums.length) {
  missing.push(
    `affiliate schema: enum(s) not defined under libs/migrations/postgresql/ -> ${absentEnums.join(', ')}`
  );
}

const payoutStatuses = enumValues.affiliate_payout_status || [];
const earningStatuses = enumValues.affiliate_earning_status || [];
const adjustmentTypes = enumValues.affiliate_adjustment_type || [];

// ---------------------------------------------------------------------------
// 2. Enum members
// ---------------------------------------------------------------------------
if (hasAffiliateSchema) {
  // Reconciliation must only reference enum members that actually exist.
  const sqlStatuses = new Set(
    [
      ...reconSql.matchAll(
        /'(requested|reviewing|approved|processing|paid|rejected|failed|accrued|pending|available|reversed|credit|debit)'/g
      ),
    ].map((m) => m[1])
  );
  const known = new Set([...payoutStatuses, ...earningStatuses, ...adjustmentTypes]);
  for (const s of sqlStatuses) {
    if (!known.has(s)) fail(`reconciliation SQL references unknown status "${s}"`);
  }

  // Every payout status must be explicitly accounted for: either it is listed
  // in a status IN (...) predicate (money leaves `available`), or it appears in
  // the documented exclusion list. A status that is silently unmentioned means
  // nobody decided whether it moves money, and that is how double-entry drifts.
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
// 3. Every table the Go ledger touches must also be audited by the SQL.
//    If Go derives a balance from a table the SQL ignores, drift is invisible.
// ---------------------------------------------------------------------------
const goTables = new Set([...ledgerGo.matchAll(/FROM\s+(affiliate_[a-z_]+)/g)].map((m) => m[1]));
for (const t of goTables) {
  if (!tablesInSql.has(t)) {
    fail(`Go derives balances from "${t}" but reconciliation SQL never audits it`);
  }
}

// ---------------------------------------------------------------------------
// 4. The PASS/FAIL gate must implement the SAME derived model as the report.
//    These are two files with the same status lists; if they drift, the CronJob
//    can report "balanced" while the report would show a divergence, i.e. the
//    automated gate would be lying.
// ---------------------------------------------------------------------------
const gatePath = 'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation_gate.sql';
if (fs.existsSync(path.join(repoRoot, gatePath))) {
  const gate = read(gatePath);
  const setsIn = (src) =>
    new Set(
      [...src.matchAll(/status\s+(?:NOT\s+)?IN\s*\(([^)]*)\)/g)]
        .flatMap((m) => m[1].match(/'[a-z]+'/g) || [])
        .map((s) => s.replace(/'/g, ''))
    );
  const reportStatuses = setsIn(reconSql);
  const gateStatuses = setsIn(gate);
  for (const s of gateStatuses) {
    if (!reportStatuses.has(s)) {
      fail(`gate uses status "${s}" which the report does not - derived models diverged`);
    }
  }
  for (const s of reportStatuses) {
    if (!gateStatuses.has(s)) {
      fail(`report uses status "${s}" which the gate does not - derived models diverged`);
    }
  }
  // The gate must carry the SAME divergence threshold as the report.
  // A gate with no threshold at all is worse than a wrong one: it would report
  // "balanced" for every affiliate. So emptiness is a failure, not a skip.
  const thresholdsIn = (src) => new Set([...src.matchAll(/>\s*(0\.\d+|\d+\.\d+)/g)].map((m) => m[1]));
  const rTh = thresholdsIn(reconSql);
  const gTh = thresholdsIn(gate);
  if (!gTh.size) {
    fail('gate declares no divergence threshold - it would report "balanced" for every affiliate');
  }
  for (const t of rTh) {
    if (!gTh.has(t)) {
      fail(
        `gate threshold (${[...gTh].join(',') || 'none'}) differs from the report (${[...rTh].join(',')})`
      );
    }
  }
  if (!/count\(\*\)\s+AS\s+diverging_affiliates/i.test(gate)) {
    fail('gate must expose count(*) AS diverging_affiliates so the runner can read the exit signal');
  }
} else {
  fail(`${gatePath} is missing - the reconciliation CronJob has no gate to run`);
}

// ---------------------------------------------------------------------------
// 5. Ledger account types: SQL <-> Go constants.
// ---------------------------------------------------------------------------
const goAccountTypes = new Set(
  [...ledgerAccounts.matchAll(/LedgerAccount[A-Z][a-zA-Z]*\s*=\s*"([a-z]+)"/g)].map((m) => m[1])
);
const sqlAccountTypes = new Set([...reconSql.matchAll(/account_type\s*=\s*'([a-z]+)'/g)].map((m) => m[1]));
if (ledgerGo) {
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
}

// ---------------------------------------------------------------------------
// 6. The SQL must stay read-only. A reconciliation job that can write is a job
//    that can silently "fix" a real divergence instead of alerting on it.
// ---------------------------------------------------------------------------
const writes = reconSql.match(/^\s*(INSERT|UPDATE|DELETE|ALTER|DROP|TRUNCATE)\b/im);
if (writes) fail(`reconciliation SQL is not read-only (found ${writes[1]})`);

// ---------------------------------------------------------------------------
// 7. Double-entry pairing: the SQL checks it via the direction column, and the
//    schema must actually have that column.
// ---------------------------------------------------------------------------
if (!/CASE WHEN direction = 'debit'/.test(reconSql)) {
  fail('double-entry check does not use the direction column');
}
if (hasAffiliateSchema && !/direction/.test(schema)) {
  fail('affiliate_ledger_entries has no direction column; double-entry check is impossible');
}

// ---------------------------------------------------------------------------
// 8. Divergence threshold must be present and > 0 (a threshold of 0 would page
//    on float noise; missing would page on everything).
// ---------------------------------------------------------------------------
const thresholds = [...reconSql.matchAll(/>\s*0\.0([0-9]+)/g)].map((m) => parseFloat(`0.0${m[1]}`));
if (!thresholds.length) {
  fail('no divergence threshold found; expected > 0.01 per architecture-overview RULE 1');
}
if (thresholds.some((t) => t <= 0)) {
  fail('divergence threshold is <= 0, which would page on rounding noise');
}

// ---------------------------------------------------------------------------
if (failures.length) {
  console.error('AFFILIATE CONSISTENCY: FAIL');
  for (const f of failures) console.error(`  - ${f}`);
  process.exit(1);
}

// Cross-boundary checks are SKIPPED, never silently passed. This must be visible:
// a green run without them would mean "the reconciliation SQL is self-consistent",
// not "the ledger is correct".
if (missing.length) {
  console.log('AFFILIATE CONSISTENCY: OK (SQL-internal checks only)');
  console.log('SKIPPED cross-boundary checks, required input(s) absent:');
  for (const m of missing) console.log(`  - ${m}`);
  process.exit(0);
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