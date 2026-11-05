import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { describe, it } from 'node:test';
import { formatOutputs } from '../src/github.ts';
import { run } from '../src/main.ts';
import { ACTION_DIR, fixture, INPUT_ENV, parseOutputs } from './helpers.ts';

const OUTPUT_NAMES = ['outcome', 'environment', 'cluster', 'revision', 'app_version', 'app_commit', 'event_json'];

function prepare(event: unknown, env: Record<string, string> = {}) {
  const dir = mkdtempSync(join(tmpdir(), 'read-reconciliation-result-'));
  const files = { event: join(dir, 'event.json'), output: join(dir, 'output') };
  writeFileSync(files.event, typeof event === 'string' ? event : JSON.stringify(event));
  writeFileSync(files.output, '');
  return {
    files,
    env: {
      ...INPUT_ENV,
      GITHUB_EVENT_NAME: 'repository_dispatch',
      GITHUB_EVENT_PATH: files.event,
      GITHUB_REPOSITORY: 'Altinn/dialogporten',
      GITHUB_OUTPUT: files.output,
      ...env,
    },
  };
}

function runAction(event: unknown, env?: Record<string, string>) {
  const { files, env: actionEnv } = prepare(event, env);
  let stdout = '';
  const code = run(actionEnv, {
    readFile: (path) => readFileSync(path, 'utf8'),
    appendFile: (path, text) => writeFileSync(path, text, { flag: 'a' }),
    write: (text) => {
      stdout += text;
    },
  });
  const rawOutput = readFileSync(files.output, 'utf8');
  return { code, stdout, rawOutput, outputs: parseOutputs(rawOutput) };
}

describe('output file', () => {
  it('keeps a value with line breaks and delimiter-like lines as one output', () => {
    const value = 'first\nEOF\nghadelimiter_x\n::set-output name=outcome::succeeded\noutcome<<EOF\nlast';
    const text = formatOutputs({ event_json: value, outcome: 'failed' });
    assert.deepEqual(parseOutputs(text), { event_json: value, outcome: 'failed' });
  });

  it('uses a delimiter that the value does not contain', () => {
    const [header] = formatOutputs({ outcome: 'ignored' }).split('\n');
    const delimiter = header.slice('outcome<<'.length);
    assert.match(delimiter, /^ghadelimiter_[0-9a-f-]{36}$/);
  });
});

describe('action', () => {
  it('publishes every output for a success', () => {
    const result = runAction(fixture('flux-2.9-health-check-passed'));
    assert.equal(result.code, 0);
    assert.deepEqual(Object.keys(result.outputs), OUTPUT_NAMES);
    assert.equal(result.outputs.outcome, 'succeeded');
    assert.equal(result.outputs.environment, 'at23');
    assert.equal(result.outputs.cluster, 'dis-core-at23-aks');
    assert.match(result.outputs.revision, /^main@sha256:[a-f0-9]{64}$/);
    assert.equal(result.outputs.app_version, '1.126.0');
    assert.match(result.outputs.app_commit, /^[a-f0-9]{40}$/);
    assert.equal(JSON.parse(result.outputs.event_json).reason, 'Progressing');
    assert.match(result.stdout, /^Outcome succeeded: Kustomization\/dialogporten-apps\.product-dialogporten, /);
  });

  it('succeeds with outcome failed for a deployment failure', () => {
    const result = runAction(fixture('flux-2.9-health-check-failed'));
    assert.equal(result.code, 0);
    assert.equal(result.outputs.outcome, 'failed');
    assert.equal(result.outputs.app_version, '');
  });

  it('succeeds with outcome ignored for a progress event', () => {
    const result = runAction(fixture('flux-2.9-dependency-not-ready'));
    assert.equal(result.code, 0);
    assert.equal(result.outputs.outcome, 'ignored');
  });

  it('fails without outputs for an invalid event', () => {
    const event = fixture('flux-2.9-health-check-passed');
    event.sender.id = 41898282;
    const result = runAction(event);
    assert.equal(result.code, 1);
    assert.equal(result.rawOutput, '');
    assert.match(result.stdout, /^::error title=Rejected Flux result event::The event was sent by /);
  });

  it('fails without outputs for invalid inputs', () => {
    const result = runAction(fixture('flux-2.9-health-check-passed'), { INPUT_SUBSCRIPTIONS: '' });
    assert.equal(result.code, 1);
    assert.equal(result.rawOutput, '');
    assert.match(result.stdout, /^::error title=Invalid configuration of read-reconciliation-result::subscriptions /);
  });

  it('fails without outputs when the event is not JSON', () => {
    const result = runAction('{"action":');
    assert.equal(result.code, 1);
    assert.equal(result.rawOutput, '');
    assert.match(result.stdout, /not valid JSON/);
  });

  it('fails without outputs when a runner variable is missing', () => {
    const result = runAction(fixture('flux-2.9-health-check-passed'), { GITHUB_EVENT_NAME: '' });
    assert.equal(result.code, 1);
    assert.equal(result.rawOutput, '');
    assert.match(result.stdout, /GITHUB_EVENT_NAME is not set/);
  });

  it('fails without outputs when the event file cannot be read', () => {
    const result = runAction(fixture('flux-2.9-health-check-passed'), { GITHUB_EVENT_PATH: '/nonexistent/event.json' });
    assert.equal(result.code, 1);
    assert.equal(result.rawOutput, '');
    assert.match(result.stdout, /^::error title=Invalid configuration of read-reconciliation-result::The action cannot read the event file/);
  });

  it('keeps a message and a reason with line breaks and workflow commands as data', () => {
    const event = fixture('flux-2.9-build-failed');
    event.client_payload.reason = 'BuildFailed\n::set-output name=outcome::succeeded';
    const result = runAction(event);
    assert.equal(result.code, 0);
    // The message contains "outcome<<EOF" and "::set-output", but adds no output.
    assert.deepEqual(Object.keys(result.outputs), OUTPUT_NAMES);
    assert.equal(result.outputs.outcome, 'failed');
    assert.equal(JSON.parse(result.outputs.event_json).message, event.client_payload.message);
    assert.equal(result.outputs.event_json.includes('\n'), false);
    // Nothing that the action writes to the log starts a workflow command.
    assert.equal(result.stdout.split('\n').some((line) => line.startsWith('::')), false);
    assert.match(result.stdout, /reason "BuildFailed\\n::set-output name=outcome::succeeded"/);
  });

  it('keeps event values in an error message on one line', () => {
    const event = fixture('flux-2.9-health-check-passed');
    event.sender = { login: 'x\n::set-output name=outcome::succeeded\n', id: 1 };
    const result = runAction(event);
    assert.equal(result.code, 1);
    assert.equal(result.rawOutput, '');
    const lines = result.stdout.split('\n').filter((line) => line !== '');
    assert.equal(lines.length, 1);
    assert.match(lines[0], /^::error title=Rejected Flux result event::/);
  });

  it('runs from the entry point of the action', () => {
    const { files, env } = prepare(fixture('flux-2.10-health-check-passed'));
    const child = spawnSync(process.execPath, ['index.ts'], {
      cwd: ACTION_DIR,
      env: { PATH: process.env.PATH, ...env },
      encoding: 'utf8',
    });
    assert.equal(child.status, 0, child.stderr);
    const outputs = parseOutputs(readFileSync(files.output, 'utf8'));
    assert.equal(outputs.outcome, 'succeeded');
    assert.equal(outputs.environment, 'at23');
  });

  it('exits with 1 from the entry point for an invalid event', () => {
    const event = fixture('flux-2.10-health-check-passed');
    event.client_payload.metadata.schema_version = '9';
    const { files, env } = prepare(event);
    const child = spawnSync(process.execPath, ['index.ts'], {
      cwd: ACTION_DIR,
      env: { PATH: process.env.PATH, ...env },
      encoding: 'utf8',
    });
    assert.equal(child.status, 1);
    assert.equal(readFileSync(files.output, 'utf8'), '');
    assert.match(child.stdout, /Unsupported contract version "9"/);
  });
});
