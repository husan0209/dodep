// Helm values sanity checks for the affiliate chart.
//
// The chart deliberately does NOT render ingress / PodDisruptionBudget /
// NetworkPolicy (they are standalone manifests). This test catches the actual
// regression that matters: values keys that no template consumes — dead config
// that silently does nothing in production.

const fs = require('fs');
const path = require('path');

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const chartDir = path.join(repoRoot, 'infra', 'helm', 'charts', 'affiliate');
const templatesDir = path.join(chartDir, 'templates');

const values = fs.readFileSync(path.join(chartDir, 'values.yaml'), 'utf8');
const valuesProd = fs.readFileSync(path.join(chartDir, 'values-production.yaml'), 'utf8');
const templates = fs
  .readdirSync(templatesDir)
  .filter((f) => f.endsWith('.yaml'))
  .map((f) => fs.readFileSync(path.join(templatesDir, f), 'utf8'))
  .join('\n');
const helpers = fs.readFileSync(path.join(templatesDir, '_helpers.tpl'), 'utf8');
const allTemplates = templates + helpers;

let failures = [];
const check = (name, cond, detail) => {
  if (!cond) failures.push(name + (detail ? ': ' + detail : ''));
};

// 1. Every top-level values key must be referenced by some template,
//    OR be one of the explicitly-deferred standalone-manifest keys.
const DEFERRED = new Set(['ingress', 'podDisruptionBudget', 'networkPolicy']);

const topLevelKeys = values
  .split('\n')
  .filter((l) => /^[a-zA-Z][a-zA-Z0-9_]*:/.test(l))
  .map((l) => l.split(':')[0].trim());

for (const key of new Set(topLevelKeys)) {
  const used = new RegExp('\\.Values\\.' + key + '\\b').test(allTemplates);
  if (!used && !DEFERRED.has(key)) {
    failures.push('dead values key: ' + key);
  }
}

// 2. Deferred keys must NOT be rendered by the chart (single owner rule).
for (const key of DEFERRED) {
  if (new RegExp('kind:\\s*(Ingress|PodDisruptionBudget|NetworkPolicy)').test(templates)) {
    failures.push('chart renders a deferred object, expect standalone manifest: ' + key);
  }
}

// 3. Ports must match CONVENTIONS.md (affiliate: 8090 HTTP / 50061 gRPC).
check('http port 8090', /httpPort:\s*8090/.test(values) && /httpTargetPort:\s*8090/.test(values));
check('grpc port 50061', /grpcPort:\s*50061/.test(values) && /grpcTargetPort:\s*50061/.test(values));
// Note: repo files use CRLF, so patterns must not anchor on a bare \n.
check('prod http port 8090', /- name: PORT\r?\n\s*value:\s*"8090"/.test(valuesProd));
check('prod grpc port 50061', /- name: GRPC_PORT\r?\n\s*value:\s*"50061"/.test(valuesProd));

// 4. Deployment must not hardcode a port that disagrees with values.
const deployment = fs.readFileSync(path.join(templatesDir, 'deployment.yaml'), 'utf8');
check(
  'deployment uses values for ports',
  /\.Values\.service\.httpTargetPort/.test(deployment) &&
    /\.Values\.service\.grpcTargetPort/.test(deployment)
);
check('deployment uses serviceAccountName helper', /include "affiliate\.serviceAccountName"/.test(deployment));
check('serviceaccount template exists', fs.existsSync(path.join(templatesDir, 'serviceaccount.yaml')));

// 5. Readiness must hit /ready (main.go exposes /health and /ready separately).
check('readiness probes /ready', /path:\s*\/ready/.test(deployment));
check('liveness probes /health', /livenessProbe:[\s\S]{0,80}path:\s*\{\{ \.Values\.healthCheck\.path \}\}/.test(deployment));

// 6. No literal secrets in values (must be secretKeyRef / configMapKeyRef).
const literalSecret = values.match(/^\s*(jwt_secret|password|api_key)\s*:\s*["'][^"']+["']/im);
check('no literal secrets in values', !literalSecret, literalSecret && literalSecret[0]);

// 7. serviceMonitor must stay disabled while the service has no /metrics.
check('serviceMonitor disabled (dev)', /serviceMonitor:\s*\n\s*#[\s\S]{0,200}?enabled:\s*false/.test(values));
check('serviceMonitor disabled (prod)', /serviceMonitor:\s*\n\s*#[\s\S]{0,200}?enabled:\s*false/.test(valuesProd));

// 8. Produce topics must be dotted (matching gorm_repository.go outbox topics),
//    never the underscore form from the task document.
const topics = (values.match(/affiliate\.[a-z_.\-]+/g) || []).filter((t) => t.startsWith('affiliate.'));
const underscoreTopics = topics.filter((t) => /affiliate\.[a-z]+_[a-z]/.test(t));
check('no underscore topic names', underscoreTopics.length === 0, underscoreTopics.join(','));

if (failures.length) {
  console.error('FAIL:');
  for (const f of failures) console.error('  - ' + f);
  process.exit(1);
}
console.log('OK: values/templates contract holds (keys=' + new Set(topLevelKeys).size + ')');