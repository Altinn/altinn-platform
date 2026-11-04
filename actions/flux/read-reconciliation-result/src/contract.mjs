// Validation and classification of Flux events that GitHub delivers as
// repository_dispatch events. RFC 0019 defines the contract:
// https://github.com/Altinn/altinn-platform/blob/main/rfcs/0019-flux-workflow-notifications.md

// The values of metadata.schema_version that this action supports.
export const CONTRACT_VERSIONS = ['1'];

// The action configuration is wrong. The workflow must be fixed.
export class InvalidInputError extends Error {}

// The event does not satisfy the contract. It is not a deployment result.
export class InvalidEventError extends Error {}

// Kubernetes object names (RFC 1123 subdomain) and namespaces (RFC 1123 label).
const OBJECT_NAME = /^(?=.{1,253}$)[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/;
const NAMESPACE = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;
// Product, environment and cluster identifiers. No wildcards.
const IDENTIFIER = /^[A-Za-z0-9]([A-Za-z0-9._-]{0,61}[A-Za-z0-9])?$/;
const SENDER_ID = /^[1-9][0-9]{0,19}$/;
const KIND = /^[A-Z][A-Za-z0-9]{0,62}$/;
const REASON = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/;
const CONTROLLER = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;
// An OCI artifact revision with a SHA-256 digest, for example main@sha256:<digest>.
const OCI_REVISION = /^([A-Za-z0-9_][A-Za-z0-9._-]{0,127}@)?sha256:[a-f0-9]{64}$/;
const APP_VERSION = /^[A-Za-z0-9][A-Za-z0-9.+_-]{0,127}$/;
const APP_COMMIT = /^[a-f0-9]{40}$/;
// An RFC 3339 time, as Kubernetes writes metav1.Time.
const TIMESTAMP = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

const SEVERITIES = new Set(['info', 'error', 'trace']);

// The controller that must report events for each supported kind.
const CONTROLLERS = new Map([
  ['Kustomization', 'kustomize-controller'],
  ['OCIRepository', 'source-controller'],
]);

const SUBSCRIPTION_KEYS = new Set(['environment', 'cluster', 'namespace', 'kustomization', 'oci_repository']);

// kustomize-controller emits this event when a new revision changed resources
// and passed the health checks, or when a failed rollout recovers. The final
// ReconciliationSucceeded event fires after every reconcile, and
// notification-controller does not send it to githubdispatch. The event has
// no reason of its own, so the action matches the message. Altinn/altinn-platform#4373
// asks Flux for a stable reason.
const HEALTH_CHECK_PASSED = 'Health check passed in ';

export function parseInputs(env) {
  const senderId = (env.INPUT_EXPECTED_SENDER_ID ?? '').trim();
  if (!SENDER_ID.test(senderId)) {
    throw new InvalidInputError('expected_sender_id must be the numeric id of the bot account of the dispatch App.');
  }
  const product = (env.INPUT_PRODUCT ?? '').trim();
  if (!IDENTIFIER.test(product)) {
    throw new InvalidInputError('product must be a product identifier, for example dialogporten.');
  }
  return { senderId, product, subscriptions: parseSubscriptions(env.INPUT_SUBSCRIPTIONS ?? '') };
}

function parseSubscriptions(raw) {
  if (raw.trim() === '') {
    throw new InvalidInputError('subscriptions must be a JSON array with at least one subscription.');
  }
  let list;
  try {
    list = JSON.parse(raw);
  } catch {
    throw new InvalidInputError('subscriptions is not valid JSON.');
  }
  if (!Array.isArray(list) || list.length === 0) {
    throw new InvalidInputError('subscriptions must be a JSON array with at least one subscription.');
  }
  return list.map((entry, index) => {
    const at = `subscriptions[${index}]`;
    if (!isObject(entry)) {
      throw new InvalidInputError(`${at} must be an object.`);
    }
    for (const key of Object.keys(entry)) {
      if (!SUBSCRIPTION_KEYS.has(key)) {
        throw new InvalidInputError(`${at} has the unknown key ${quote(key)}.`);
      }
    }
    requireMatch(entry.environment, IDENTIFIER, `${at}.environment`);
    requireMatch(entry.cluster, IDENTIFIER, `${at}.cluster`);
    requireMatch(entry.namespace, NAMESPACE, `${at}.namespace`);
    requireMatch(entry.kustomization, OBJECT_NAME, `${at}.kustomization`);
    if (entry.oci_repository !== undefined) {
      requireMatch(entry.oci_repository, OBJECT_NAME, `${at}.oci_repository`);
    }
    return {
      environment: entry.environment,
      cluster: entry.cluster,
      namespace: entry.namespace,
      kustomization: entry.kustomization,
      ociRepository: entry.oci_repository ?? null,
    };
  });
}

function requireMatch(value, pattern, name) {
  if (typeof value !== 'string' || !pattern.test(value)) {
    throw new InvalidInputError(`${name} must be an exact, valid name. Wildcards are not permitted. Got ${quote(value)}.`);
  }
}

// Validates a repository_dispatch event and classifies the Flux event in it.
// Throws InvalidEventError if the event does not satisfy the contract.
export function evaluate({ inputs, eventName, event, repository }) {
  if (eventName !== 'repository_dispatch') {
    throw new InvalidEventError(`The action supports only repository_dispatch events, got ${quote(eventName)}.`);
  }
  if (!isObject(event)) {
    throw new InvalidEventError('The event payload is not a JSON object.');
  }

  const sender = isObject(event.sender) ? event.sender : {};
  if (!Number.isSafeInteger(sender.id) || String(sender.id) !== inputs.senderId) {
    throw new InvalidEventError(
      `The event was sent by ${quote(sender.login)} with id ${quote(sender.id)}, not by the dispatch App (id ${inputs.senderId}).`,
    );
  }
  const eventRepository = isObject(event.repository) ? event.repository.full_name : undefined;
  if (!sameRepository(eventRepository, repository)) {
    throw new InvalidEventError(`The event belongs to ${quote(eventRepository)}, not to ${repository}.`);
  }
  if (!isObject(event.client_payload)) {
    throw new InvalidEventError('The event has no client_payload object.');
  }

  const flux = readFluxEvent(event.client_payload);
  const resource = `${flux.kind}/${flux.name}.${flux.namespace}`;
  if (event.action !== resource) {
    throw new InvalidEventError(`The event type ${quote(event.action)} does not match the involved object ${quote(resource)}.`);
  }

  const metadata = flux.metadata;
  if (!CONTRACT_VERSIONS.includes(metadata.schema_version)) {
    throw new InvalidEventError(
      `Unsupported contract version ${quote(metadata.schema_version)}. Supported: ${CONTRACT_VERSIONS.join(', ')}.`,
    );
  }
  if (metadata.product !== inputs.product) {
    throw new InvalidEventError(`The event is for the product ${quote(metadata.product)}, not ${quote(inputs.product)}.`);
  }
  if (!sameRepository(metadata.repository, repository)) {
    throw new InvalidEventError(`The subscription names the repository ${quote(metadata.repository)}, not ${repository}.`);
  }

  const controller = CONTROLLERS.get(flux.kind);
  if (controller === undefined) {
    throw new InvalidEventError(`Unsupported resource kind ${quote(flux.kind)}.`);
  }
  if (flux.reportingController !== controller) {
    throw new InvalidEventError(
      `${flux.kind} events must come from ${controller}, got ${quote(flux.reportingController)}.`,
    );
  }

  const subscription = inputs.subscriptions.find(
    (s) =>
      s.environment === metadata.environment &&
      s.cluster === metadata.cluster &&
      s.namespace === flux.namespace &&
      (flux.kind === 'Kustomization' ? s.kustomization === flux.name : s.ociRepository === flux.name),
  );
  if (subscription === undefined) {
    throw new InvalidEventError(
      `No subscription matches environment ${quote(metadata.environment)}, cluster ${quote(metadata.cluster)} and ${resource}.`,
    );
  }

  const outcome = classify(flux);
  const revision = typeof metadata.revision === 'string' && OCI_REVISION.test(metadata.revision) ? metadata.revision : '';
  if (outcome === 'succeeded' && revision === '') {
    throw new InvalidEventError(
      `A success event must have an OCI artifact revision with a SHA-256 digest, got ${quote(metadata.revision)}.`,
    );
  }

  const release = bindRelease(metadata, revision);
  const payload = structuredClone(event.client_payload);
  for (const key of release.unbound) {
    delete payload.metadata[key];
  }

  return {
    outcome,
    environment: subscription.environment,
    cluster: subscription.cluster,
    revision,
    appVersion: release.appVersion,
    appCommit: release.appCommit,
    eventJson: JSON.stringify(payload),
    resource,
    reason: flux.reason,
  };
}

function readFluxEvent(payload) {
  const object = payload.involvedObject;
  if (!isObject(object)) {
    throw new InvalidEventError('client_payload.involvedObject must be an object.');
  }
  requireEventMatch(object.kind, KIND, 'client_payload.involvedObject.kind');
  requireEventMatch(object.name, OBJECT_NAME, 'client_payload.involvedObject.name');
  requireEventMatch(object.namespace, NAMESPACE, 'client_payload.involvedObject.namespace');
  if (!SEVERITIES.has(payload.severity)) {
    throw new InvalidEventError(`client_payload.severity must be info, error or trace, got ${quote(payload.severity)}.`);
  }
  requireEventMatch(payload.reason, REASON, 'client_payload.reason');
  requireEventMatch(payload.reportingController, CONTROLLER, 'client_payload.reportingController');
  if (typeof payload.message !== 'string') {
    throw new InvalidEventError('client_payload.message must be a string.');
  }
  if (
    typeof payload.timestamp !== 'string' ||
    !TIMESTAMP.test(payload.timestamp) ||
    Number.isNaN(Date.parse(payload.timestamp))
  ) {
    throw new InvalidEventError(`client_payload.timestamp must be a time, got ${quote(payload.timestamp)}.`);
  }
  // Flux 2.10 events (event API v1) add the action field. The action is only
  // compared, so any short text is accepted.
  if (payload.action !== undefined && (typeof payload.action !== 'string' || payload.action.length > 128)) {
    throw new InvalidEventError(`client_payload.action is not valid, got ${quote(payload.action)}.`);
  }
  const metadata = payload.metadata ?? {};
  if (!isObject(metadata) || !Object.values(metadata).every((value) => typeof value === 'string')) {
    throw new InvalidEventError('client_payload.metadata must be an object with string values.');
  }
  return {
    kind: object.kind,
    name: object.name,
    namespace: object.namespace,
    severity: payload.severity,
    reason: payload.reason,
    message: payload.message,
    action: payload.action,
    reportingController: payload.reportingController,
    metadata,
  };
}

function requireEventMatch(value, pattern, name) {
  if (typeof value !== 'string' || !pattern.test(value)) {
    throw new InvalidEventError(`${name} is not valid, got ${quote(value)}.`);
  }
}

function classify(flux) {
  if (flux.severity === 'error') {
    return 'failed';
  }
  if (flux.severity === 'info' && flux.kind === 'Kustomization' && isHealthCheckPassed(flux)) {
    return 'succeeded';
  }
  return 'ignored';
}

function isHealthCheckPassed(flux) {
  // The event reason is the Ready reason during the health checks. Flux 2.10
  // also sets the action HealthCheck. notification-controller 1.10 sends an
  // empty action for an event from an older kustomize-controller.
  return (
    flux.message.startsWith(HEALTH_CHECK_PASSED) &&
    flux.reason === 'Progressing' &&
    (!flux.action || flux.action === 'HealthCheck')
  );
}

// Release metadata describes one artifact. It applies only when its
// release_revision equals the revision that the controller reports.
function bindRelease(metadata, revision) {
  const bound = revision !== '' && metadata.release_revision === revision;
  const appVersion = bound && APP_VERSION.test(metadata.app_version ?? '') ? metadata.app_version : '';
  const appCommit = bound && APP_COMMIT.test(metadata.app_commit ?? '') ? metadata.app_commit : '';
  const unbound = [];
  if (!appVersion && 'app_version' in metadata) unbound.push('app_version');
  if (!appCommit && 'app_commit' in metadata) unbound.push('app_commit');
  if ((!bound || (!appVersion && !appCommit)) && 'release_revision' in metadata) unbound.push('release_revision');
  return { appVersion, appCommit, unbound };
}

// GitHub repository names are not case sensitive.
function sameRepository(value, repository) {
  return typeof value === 'string' && typeof repository === 'string' && value.toLowerCase() === repository.toLowerCase();
}

function isObject(value) {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

// Quotes a value from the event for an error message. The result is one line.
export function quote(value) {
  const text = value === undefined ? 'undefined' : JSON.stringify(value) ?? String(value);
  return text.length > 120 ? `${text.slice(0, 117)}...` : text;
}
