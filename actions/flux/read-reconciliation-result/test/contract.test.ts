import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { evaluate, InvalidEventError, InvalidInputError, parseInputs, type Outcome } from '../src/contract.ts';
import { fixture, INPUT_ENV, SUBSCRIPTIONS } from './helpers.ts';

const inputs = parseInputs(INPUT_ENV);

function evaluateEvent(event: unknown, overrides: { eventName?: string } = {}) {
  return evaluate({ inputs, eventName: 'repository_dispatch', event, repository: 'Altinn/dialogporten', ...overrides });
}

function assertRejected(event: unknown, pattern: RegExp, overrides?: { eventName?: string }) {
  assert.throws(
    () => evaluateEvent(event, overrides),
    (error) => error instanceof InvalidEventError && pattern.test(error.message),
  );
}

function changed(name: string, change: (event: any) => void) {
  const event = fixture(name);
  change(event);
  return event;
}

describe('inputs', () => {
  it('accepts the sender id, the product and the subscriptions', () => {
    assert.equal(inputs.senderId, '123456789');
    assert.equal(inputs.product, 'dialogporten');
    assert.deepEqual(inputs.subscriptions, SUBSCRIPTIONS);
  });

  const subscription = SUBSCRIPTIONS[0];
  const cases: [string, Record<string, string>, RegExp][] = [
    ['a missing sender id', { INPUT_EXPECTED_SENDER_ID: '' }, /expected_sender_id/],
    ['a sender name instead of an id', { INPUT_EXPECTED_SENDER_ID: 'altinn-flux-dispatch[bot]' }, /expected_sender_id/],
    ['a missing product', { INPUT_PRODUCT: '' }, /product/],
    ['missing subscriptions', { INPUT_SUBSCRIPTIONS: '' }, /at least one subscription/],
    ['subscriptions that are not JSON', { INPUT_SUBSCRIPTIONS: '[{' }, /at least one subscription/],
    ['an empty subscription list', { INPUT_SUBSCRIPTIONS: '[]' }, /at least one subscription/],
    ['a subscription that is not an object', { INPUT_SUBSCRIPTIONS: '["at23"]' }, /must be an object/],
    [
      'a subscription without a cluster',
      { INPUT_SUBSCRIPTIONS: JSON.stringify([{ ...subscription, cluster: undefined }]) },
      /subscriptions\[0\]\.cluster/,
    ],
    [
      'a subscription with an unknown key',
      { INPUT_SUBSCRIPTIONS: JSON.stringify([{ ...subscription, kustomisation: 'dialogporten-apps' }]) },
      /unknown key "kustomisation"/,
    ],
    [
      'a wildcard namespace',
      { INPUT_SUBSCRIPTIONS: JSON.stringify([{ ...subscription, namespace: '*' }]) },
      /Wildcards are not permitted/,
    ],
    [
      'a wildcard in a name',
      { INPUT_SUBSCRIPTIONS: JSON.stringify([{ ...subscription, kustomization: 'dialogporten-*' }]) },
      /Wildcards are not permitted/,
    ],
    [
      'a wildcard environment',
      { INPUT_SUBSCRIPTIONS: JSON.stringify([{ ...subscription, environment: '*' }]) },
      /Wildcards are not permitted/,
    ],
  ];
  for (const [name, env, pattern] of cases) {
    it(`rejects ${name}`, () => {
      assert.throws(
        () => parseInputs({ ...INPUT_ENV, ...env }),
        (error) => error instanceof InvalidInputError && pattern.test(error.message),
      );
    });
  }
});

describe('classification', () => {
  const cases: [string, Outcome][] = [
    ['flux-2.9-health-check-passed', 'succeeded'],
    ['flux-2.10-health-check-passed', 'succeeded'],
    ['flux-2.9-health-check-failed', 'failed'],
    ['flux-2.9-build-failed', 'failed'],
    ['flux-2.9-source-failed', 'failed'],
    ['flux-2.9-dependency-not-ready', 'ignored'],
    ['flux-2.9-change-set', 'ignored'],
    ['flux-2.9-health-check-canceled', 'ignored'],
    ['flux-2.9-reconciliation-succeeded', 'ignored'],
    ['flux-2.10-apply', 'ignored'],
    ['flux-2.10-health-check-failed', 'failed'],
    ['flux-2.10-health-check-canceled', 'ignored'],
    ['flux-2.9-event-from-notification-controller-2.10', 'succeeded'],
  ];
  for (const [name, outcome] of cases) {
    it(`classifies ${name} as ${outcome}`, () => {
      assert.equal(evaluateEvent(fixture(name)).outcome, outcome);
    });
  }

  it('returns the subscription, the resource and the revision of a success', () => {
    const result = evaluateEvent(fixture('flux-2.9-health-check-passed'));
    assert.equal(result.environment, 'at23');
    assert.equal(result.cluster, 'dis-core-at23-aks');
    assert.equal(result.resource, 'Kustomization/dialogporten-apps.product-dialogporten');
    assert.equal(result.reason, 'Progressing');
    assert.match(result.revision, /^main@sha256:[a-f0-9]{64}$/);
  });

  it('keeps the revision of a failed attempt', () => {
    const result = evaluateEvent(fixture('flux-2.9-health-check-failed'));
    assert.equal(result.outcome, 'failed');
    assert.equal(result.reason, 'HealthCheckFailed');
    assert.match(result.revision, /^main@sha256:[a-f0-9]{64}$/);
  });

  it('reports a source failure without a revision', () => {
    const result = evaluateEvent(fixture('flux-2.9-source-failed'));
    assert.equal(result.outcome, 'failed');
    assert.equal(result.revision, '');
    assert.equal(result.resource, 'OCIRepository/dialogporten-manifests.product-dialogporten');
  });

  it('accepts a success event with an empty action', () => {
    // notification-controller 1.10 sends action "" for an event from kustomize-controller 1.9.
    const event = fixture('flux-2.9-event-from-notification-controller-2.10');
    assert.equal(event.client_payload.action, '');
    assert.equal(evaluateEvent(event).outcome, 'succeeded');
  });

  it('keeps a recovery of the same revision eligible for success', () => {
    const failure = evaluateEvent(fixture('flux-2.9-health-check-failed'));
    const recovery = evaluateEvent(fixture('flux-2.10-health-check-passed'));
    assert.equal(failure.outcome, 'failed');
    assert.equal(recovery.outcome, 'succeeded');
    assert.equal(recovery.revision, failure.revision);
  });

  it('classifies a failure with an unknown action as failed', () => {
    const event = changed('flux-2.10-health-check-failed', (e) => (e.client_payload.action = 'SomeFutureAction'));
    assert.equal(evaluateEvent(event).outcome, 'failed');
  });

  it('does not treat a health check message from another action as success', () => {
    const event = changed('flux-2.10-health-check-passed', (e) => {
      e.client_payload.action = 'Apply';
    });
    assert.equal(evaluateEvent(event).outcome, 'ignored');
  });

  it('does not treat a health check message with another reason as success', () => {
    const event = changed('flux-2.9-health-check-passed', (e) => {
      e.client_payload.reason = 'ReconciliationSucceeded';
    });
    assert.equal(evaluateEvent(event).outcome, 'ignored');
  });

  it('ignores trace events', () => {
    const event = changed('flux-2.9-health-check-failed', (e) => {
      e.client_payload.severity = 'trace';
    });
    assert.equal(evaluateEvent(event).outcome, 'ignored');
  });

  it('ignores source events that are not errors', () => {
    const event = changed('flux-2.9-source-failed', (e) => {
      e.client_payload.severity = 'info';
      e.client_payload.reason = 'NewArtifact';
      e.client_payload.message = "stored artifact for digest 'main@sha256:...'";
    });
    assert.equal(evaluateEvent(event).outcome, 'ignored');
  });
});

describe('rejection', () => {
  const success = 'flux-2.9-health-check-passed';

  it('rejects other GitHub events', () => {
    assertRejected(fixture(success), /only repository_dispatch events, got "push"/, { eventName: 'push' });
  });

  it('rejects an unknown sender', () => {
    const event = changed(success, (e) => {
      e.sender = { login: 'github-actions[bot]', id: 41898282, type: 'Bot' };
    });
    assertRejected(event, /not by the dispatch App/);
  });

  it('rejects a sender id as text', () => {
    assertRejected(changed(success, (e) => (e.sender.id = '123456789')), /not by the dispatch App/);
  });

  it('rejects an event without a sender', () => {
    assertRejected(changed(success, (e) => delete e.sender), /not by the dispatch App/);
  });

  it('rejects an event for another repository', () => {
    assertRejected(changed(success, (e) => (e.repository.full_name = 'Altinn/altinn-authentication')), /belongs to/);
  });

  it('rejects a subscription for another repository', () => {
    const event = changed(success, (e) => (e.client_payload.metadata.repository = 'Altinn/altinn-authentication'));
    assertRejected(event, /names the repository "Altinn\/altinn-authentication"/);
  });

  it('accepts repository names in other letter case', () => {
    const event = changed(success, (e) => {
      e.repository.full_name = 'altinn/Dialogporten';
      e.client_payload.metadata.repository = 'ALTINN/dialogporten';
    });
    assert.equal(evaluateEvent(event).outcome, 'succeeded');
  });

  it('rejects an unsupported contract version', () => {
    assertRejected(changed(success, (e) => (e.client_payload.metadata.schema_version = '2')), /contract version "2"/);
  });

  it('rejects an event without a contract version', () => {
    assertRejected(changed(success, (e) => delete e.client_payload.metadata.schema_version), /contract version/);
  });

  it('rejects an event without metadata', () => {
    assertRejected(changed(success, (e) => delete e.client_payload.metadata), /contract version/);
  });

  it('rejects another product', () => {
    assertRejected(changed(success, (e) => (e.client_payload.metadata.product = 'authentication')), /product "authentication"/);
  });

  it('rejects an unexpected controller', () => {
    const event = changed(success, (e) => (e.client_payload.reportingController = 'helm-controller'));
    assertRejected(event, /must come from kustomize-controller/);
  });

  it('rejects a source event from another controller', () => {
    const event = changed('flux-2.9-source-failed', (e) => (e.client_payload.reportingController = 'kustomize-controller'));
    assertRejected(event, /must come from source-controller/);
  });

  it('rejects an unsupported kind', () => {
    const event = changed(success, (e) => {
      e.client_payload.involvedObject.kind = 'HelmRelease';
      e.action = 'HelmRelease/dialogporten-apps.product-dialogporten';
    });
    assertRejected(event, /Unsupported resource kind "HelmRelease"/);
  });

  it('rejects an event type that does not match the involved object', () => {
    assertRejected(changed(success, (e) => (e.action = 'Kustomization/other-apps.product-dialogporten')), /does not match/);
  });

  it('rejects an environment without a subscription', () => {
    assertRejected(changed(success, (e) => (e.client_payload.metadata.environment = 'prod')), /No subscription matches/);
  });

  it('rejects a resource without a subscription', () => {
    const event = changed(success, (e) => {
      e.client_payload.involvedObject.name = 'other-apps';
      e.action = 'Kustomization/other-apps.product-dialogporten';
    });
    assertRejected(event, /No subscription matches/);
  });

  it('rejects a namespace without a subscription', () => {
    const event = changed(success, (e) => {
      e.client_payload.involvedObject.namespace = 'product-other';
      e.action = 'Kustomization/dialogporten-apps.product-other';
    });
    assertRejected(event, /No subscription matches/);
  });

  it('rejects values from different subscriptions', () => {
    // at23 is in the first subscription and dis-core-tt02-aks in the second.
    assertRejected(changed(success, (e) => (e.client_payload.metadata.cluster = 'dis-core-tt02-aks')), /No subscription matches/);
  });

  it('rejects a source event when no subscription names the source', () => {
    const withoutSource = SUBSCRIPTIONS.map(({ oci_repository, ...rest }) => rest);
    const own = parseInputs({ ...INPUT_ENV, INPUT_SUBSCRIPTIONS: JSON.stringify(withoutSource) });
    assert.throws(
      () =>
        evaluate({
          inputs: own,
          eventName: 'repository_dispatch',
          event: fixture('flux-2.9-source-failed'),
          repository: 'Altinn/dialogporten',
        }),
      (error) => error instanceof InvalidEventError && /No subscription matches/.test(error.message),
    );
  });

  it('rejects a success without a revision', () => {
    assertRejected(changed(success, (e) => delete e.client_payload.metadata.revision), /must have an OCI artifact revision/);
  });

  it('rejects a success with a Git revision', () => {
    const event = changed(success, (e) => (e.client_payload.metadata.revision = `main@sha1:${'a'.repeat(40)}`));
    assertRejected(event, /must have an OCI artifact revision/);
  });

  it('rejects a revision that is not text', () => {
    assertRejected(changed(success, (e) => (e.client_payload.metadata.revision = 42)), /must have an OCI artifact revision/);
  });

  it('rejects an event without a message', () => {
    assertRejected(changed(success, (e) => delete e.client_payload.message), /client_payload\.message must be a string/);
  });

  it('rejects an event without an involved object', () => {
    assertRejected(changed(success, (e) => delete e.client_payload.involvedObject), /involvedObject must be an object/);
  });

  it('rejects an involved object without a name', () => {
    // A missing name must not match a subscription without oci_repository.
    const event = changed('flux-2.9-source-failed', (e) => {
      delete e.client_payload.involvedObject.name;
      e.action = 'OCIRepository/undefined.product-dialogporten';
    });
    assertRejected(event, /involvedObject\.name must be a string/);
  });

  it('rejects metadata that is an array', () => {
    assertRejected(changed(success, (e) => (e.client_payload.metadata = ['at23'])), /contract version/);
  });

  it('rejects an event without client_payload', () => {
    assertRejected(changed(success, (e) => delete e.client_payload), /client_payload/);
  });

  it('rejects a payload that is not an object', () => {
    assertRejected([], /not a JSON object/);
  });
});

describe('release metadata', () => {
  it('returns release metadata for the revision of the event', () => {
    const result = evaluateEvent(fixture('flux-2.9-health-check-passed'));
    assert.equal(result.appVersion, '1.126.0');
    assert.match(result.appCommit, /^[a-f0-9]{40}$/);
    const metadata = JSON.parse(result.eventJson).metadata;
    assert.equal(metadata.release_revision, result.revision);
    assert.equal(metadata.app_version, '1.126.0');
  });

  it('removes release metadata of another revision', () => {
    // The annotation still describes the previous release when a new artifact fails.
    const result = evaluateEvent(fixture('flux-2.9-health-check-failed'));
    assert.equal(result.appVersion, '');
    assert.equal(result.appCommit, '');
    const metadata = JSON.parse(result.eventJson).metadata;
    assert.equal('app_version' in metadata, false);
    assert.equal('app_commit' in metadata, false);
    assert.equal('release_revision' in metadata, false);
    assert.equal(metadata.revision, result.revision);
    assert.ok(metadata.originRevision);
  });

  it('removes an application commit that is not a full commit id', () => {
    const event = changed('flux-2.9-health-check-passed', (e) => (e.client_payload.metadata.app_commit = 'cec401f'));
    const result = evaluateEvent(event);
    assert.equal(result.appVersion, '1.126.0');
    assert.equal(result.appCommit, '');
    const metadata = JSON.parse(result.eventJson).metadata;
    assert.equal('app_commit' in metadata, false);
    assert.equal(metadata.release_revision, result.revision);
  });

  it('removes release metadata when no value of it is valid', () => {
    const event = changed('flux-2.9-health-check-passed', (e) => {
      e.client_payload.metadata.app_version = '1.126.0 (build 5)';
      e.client_payload.metadata.app_commit = 'cec401f';
    });
    const result = evaluateEvent(event);
    assert.equal(result.appVersion, '');
    assert.equal(result.appCommit, '');
    const metadata = JSON.parse(result.eventJson).metadata;
    assert.deepEqual(
      ['app_version', 'app_commit', 'release_revision'].filter((key) => key in metadata),
      [],
    );
  });

  it('returns no release metadata when the event has none', () => {
    const result = evaluateEvent(fixture('flux-2.10-health-check-passed'));
    assert.equal(result.appVersion, '');
    assert.equal(result.appCommit, '');
  });

  it('keeps the other fields of the Flux event', () => {
    const event = JSON.parse(evaluateEvent(fixture('flux-2.10-health-check-passed')).eventJson);
    assert.equal(event.action, 'HealthCheck');
    assert.equal(event.relatedObject.kind, 'OCIRepository');
    assert.equal(event.reportingInstance, 'kustomize-controller-7d8e9f0a1-q4w5e');
    assert.match(event.metadata.originRevision, /^main\/[a-f0-9]{40}$/);
  });
});
