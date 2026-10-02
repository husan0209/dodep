// Generates the affiliate reconciliation ConfigMap from the canonical sources.
//
//   libs/migrations/postgresql/queries/affiliate_ledger_reconciliation.sql
//   libs/migrations/postgresql/queries/affiliate_ledger_reconciliation_gate.sql
//   infra/k8s/data/postgresql/affiliate-reconciliation/run.sh
//     -> infra/k8s/data/postgresql/affiliate-reconciliation/affiliate-reconciliation.yaml
//
// Duplicating the SQL inside the manifest would let the two drift silently, and a
// drifted gate is worse than no gate: it would report "balanced" while money is
// missing. So the ConfigMap is generated, and consistency-check.js fails the build
// if the generated file is stale.
//
// Usage: node tools/testing/affiliate/generate-reconciliation-configmap.js [--check]

const fs = require('fs');
const path = require('path');

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const read = (p) => fs.readFileSync(path.join(repoRoot, p), 'utf8');

const SOURCES = {
  'affiliate_ledger_reconciliation.sql':
    'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation.sql',
  'affiliate_ledger_reconciliation_gate.sql':
    'libs/migrations/postgresql/queries/affiliate_ledger_reconciliation_gate.sql',
  'run.sh': 'infra/k8s/data/postgresql/affiliate-reconciliation/run.sh',
};

const OUT = 'infra/k8s/data/postgresql/affiliate-reconciliation/affiliate-reconciliation.yaml';

const block = (indent, text) => {
  // ConfigMap stringData must be valid YAML: indent every line and quote.
  return text
    .replace(/\r\n/g, '\n')
    .split('\n')
    .map((l) => `${' '.repeat(indent)}${l}`)
    .join('\n');
};

let data = '';
for (const [key, src] of Object.entries(SOURCES)) {
  const content = read(src);
  if (content.includes('\t')) {
    console.error(`TAB character in ${src} - would break YAML indentation`);
    process.exit(1);
  }
  data += `  ${key}: |\n${block(4, content)}\n`;
}

const manifest = `# GENERATED FILE - DO NOT EDIT BY HAND.
# Source of truth:
#   ${Object.values(SOURCES).join('\n#   ')}
# Regenerate:  node tools/testing/affiliate/generate-reconciliation-configmap.js
# Verify:      node tools/testing/affiliate/consistency-check.js
apiVersion: v1
kind: ConfigMap
metadata:
  name: affiliate-reconciliation-sql
  namespace: data
  labels:
    app.kubernetes.io/part-of: opus-casino
    app.kubernetes.io/component: affiliate-reconciliation
data:
${data}---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: affiliate-ledger-reconciliation
  namespace: data
  labels:
    app.kubernetes.io/part-of: opus-casino
    app.kubernetes.io/component: affiliate-reconciliation
spec:
  # Hourly, per architecture-overview RULE 1: reconciliation must run every hour,
  # not when somebody remembers. Minute 17 avoids the top-of-hour stampede shared
  # with every other cron in the cluster.
  schedule: "17 * * * *"
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 5
  startingDeadlineSeconds: 300
  jobTemplate:
    spec:
      # A failing reconciliation must not auto-retry: a retry storm would page
      # repeatedly and hide a persistent real divergence behind retries.
      backoffLimit: 0
      ttlSecondsAfterFinished: 86400
      template:
        metadata:
          labels:
            app.kubernetes.io/component: affiliate-reconciliation
        spec:
          restartPolicy: Never
          securityContext:
            runAsNonRoot: true
            runAsUser: 999
            fsGroup: 999
          containers:
            - name: reconcile
              image: postgres:16-alpine
              securityContext:
                allowPrivilegeEscalation: false
                readOnlyRootFilesystem: true
                capabilities:
                  drop:
                    - ALL
              env:
                - name: PGHOST
                  value: pg-coordinator.data.svc.cluster.local
                - name: PGPORT
                  value: "5432"
                - name: PGDATABASE
                  valueFrom:
                    secretKeyRef:
                      name: postgresql-secrets
                      key: postgres-database
                      optional: true
                - name: PGUSER
                  valueFrom:
                    secretKeyRef:
                      name: postgresql-secrets
                      key: postgres-user
                      optional: true
                - name: PGPASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: postgresql-secrets
                      key: postgres-password
                - name: PGSSLMODE
                  value: require
              command: ["/bin/sh", "/runner/run.sh"]
              resources:
                requests:
                  cpu: 50m
                  memory: 64Mi
                limits:
                  cpu: 500m
                  memory: 256Mi
              volumeMounts:
                - name: sql
                  mountPath: /sql
                  readOnly: true
                - name: runner
                  mountPath: /runner
                  readOnly: true
                - name: tmp
                  mountPath: /tmp
          volumes:
            - name: sql
              configMap:
                name: affiliate-reconciliation-sql
            - name: runner
              configMap:
                name: affiliate-reconciliation-sql
                items:
                  - key: run.sh
                    path: run.sh
                defaultMode: 0555
            # readOnlyRootFilesystem needs a writable /tmp for psql.
            - name: tmp
              emptyDir:
                sizeLimit: 16Mi
`;

if (process.argv.includes('--check')) {
  const existing = fs.existsSync(path.join(repoRoot, OUT))
    ? fs.readFileSync(path.join(repoRoot, OUT), 'utf8')
    : '';
  if (existing !== manifest) {
    console.error('STALE: ' + OUT + ' does not match its sources.');
    console.error('Regenerate: node tools/testing/affiliate/generate-reconciliation-configmap.js');
    process.exit(1);
  }
  console.log('OK: reconciliation ConfigMap is in sync with its sources');
} else {
  fs.writeFileSync(path.join(repoRoot, OUT), manifest);
  console.log('wrote ' + OUT);
}