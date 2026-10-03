// Proves the gate-vs-report derived-model rule actually trips.
// A guard rule that has never been observed failing is a guess, not a guard.
//
// Run: node tools/testing/affiliate/negative-consistency-check.js

const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const GATE = path.join(repoRoot, 'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation_gate.sql');
const CHECK = path.join(repoRoot, 'tools/testing/affiliate/consistency-check.js');
const GEN = path.join(repoRoot, 'tools/testing/affiliate/generate-reconciliation-configmap.js');

const original = fs.readFileSync(GATE, 'utf8');
const run = () => {
  try {
    execFileSync(process.execPath, [CHECK], { cwd: repoRoot, env: { ...process.env, NODE_PATH: path.join(repoRoot, 'node_modules') } });
    return { code: 0, out: '' };
  } catch (e) {
    return { code: 1, out: (e.stdout || '').toString() + (e.stderr || '').toString() };
  }
};

if (run().code !== 0) {
  console.error('baseline must pass before testing mutations');
  process.exit(1);
}
console.log('baseline: PASS');

// Remove one payout status from the gate only. The report keeps it, so the two
// derived models diverge — exactly the bug that would let the CronJob report
// "balanced" while money is missing.
const MUTATIONS = [
  {
    name: 'gate drops "reviewing"',
    apply: (c) => c.replace(/'reviewing',\s*/g, ''),
  },
  {
    name: 'gate drops "approved"',
    apply: (c) => c.replace(/'approved',\s*/g, ''),
  },
  {
    name: 'gate gains a status the report lacks',
    apply: (c) => c.replace(/'requested',(\s*)'reviewing'/, "'requested',$1'refunded',$1'reviewing'"),
  },
  {
    name: 'gate stops aliasing the count',
    apply: (c) => c.replace(/AS\s+diverging_affiliates/g, ''),
  },
  {
    name: 'gate threshold loosened to 1.00',
    apply: (c) => c.replace(/>\s*0\.01/g, '> 1.00'),
  },
];

let bad = 0;
for (const m of MUTATIONS) {
  const mutated = m.apply(original);
  if (mutated === original) {
    console.error(`  MISS  ${m.name} — pattern did not match`);
    bad++;
    continue;
  }
  fs.writeFileSync(GATE, mutated, 'utf8');
  const r = run();
  fs.writeFileSync(GATE, original, 'utf8');
  if (r.code === 0) {
    console.error(`  VACUOUS  ${m.name}`);
    bad++;
  } else {
    const reason = (r.out.split('\n').find((l) => l.trim().startsWith('- ')) || '').trim();
    console.log(`  TRIPS  ${m.name} -> ${reason}`);
  }
}

// Leave the tree exactly as found.
fs.writeFileSync(GATE, original, 'utf8');
execFileSync(process.execPath, [GEN], { cwd: repoRoot });
if (run().code !== 0) {
  console.error('FATAL: gate file was not restored');
  process.exit(1);
}
if (bad) {
  console.error(`\n${bad} rule(s) unproven`);
  process.exit(1);
}
console.log(`\nall ${MUTATIONS.length} consistency rules verified to trip`);