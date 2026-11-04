import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

export const ACTION_DIR = fileURLToPath(new URL('..', import.meta.url));

// The fixtures are repository_dispatch events as GitHub delivers them, with the
// Flux event in client_payload. The ids and digests are examples.
export function fixture(name) {
  return JSON.parse(readFileSync(new URL(`./fixtures/${name}.json`, import.meta.url), 'utf8'));
}

export const SUBSCRIPTIONS = [
  {
    environment: 'at23',
    cluster: 'dis-core-at23-aks',
    namespace: 'product-dialogporten',
    kustomization: 'dialogporten-apps',
    oci_repository: 'dialogporten-manifests',
  },
  {
    environment: 'tt02',
    cluster: 'dis-core-tt02-aks',
    namespace: 'product-dialogporten',
    kustomization: 'dialogporten-apps',
    oci_repository: 'dialogporten-manifests',
  },
];

export const INPUT_ENV = {
  INPUT_EXPECTED_SENDER_ID: '123456789',
  INPUT_PRODUCT: 'dialogporten',
  INPUT_SUBSCRIPTIONS: JSON.stringify(SUBSCRIPTIONS),
};

// Parses a GITHUB_OUTPUT file the same way as the runner: name<<delimiter,
// the value lines, then the delimiter.
export function parseOutputs(text) {
  const outputs = {};
  const lines = text.split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (lines[i] === '') continue;
    const match = /^([A-Za-z_][A-Za-z0-9_-]*)<<(.+)$/.exec(lines[i]);
    if (!match) throw new Error(`Unexpected output line ${JSON.stringify(lines[i])}`);
    const [, name, delimiter] = match;
    const value = [];
    for (i++; i < lines.length && lines[i] !== delimiter; i++) value.push(lines[i]);
    if (i >= lines.length) throw new Error(`No end delimiter for the output ${name}`);
    if (name in outputs) throw new Error(`Duplicate output ${name}`);
    outputs[name] = value.join('\n');
  }
  return outputs;
}
