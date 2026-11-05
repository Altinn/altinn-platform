// Validation and classification of Flux events that GitHub delivers as
// repository_dispatch events. RFC 0019 defines the contract:
// https://github.com/Altinn/altinn-platform/blob/main/rfcs/0019-flux-workflow-notifications.md
//
// The action compares each event field that it uses with an input or a
// constant. It checks the format only of the values that it publishes.

// The values of metadata.schema_version that this action supports.
export const CONTRACT_VERSIONS = ['1'];

// The action configuration is wrong. The workflow must be fixed.
export class InvalidInputError extends Error {}

// The event does not satisfy the contract. It is not a deployment result.
export class InvalidEventError extends Error {}

export type Outcome = 'succeeded' | 'failed' | 'ignored';

export interface Subscription {
  environment: string;
  cluster: string;
  namespace: string;
  kustomization: string;
  oci_repository?: string;
}

export interface Inputs {
  senderId: string;
  product: string;
  subscriptions: Subscription[];
}

export interface Result {
  outcome: Outcome;
  environment: string;
  cluster: string;
  revision: string;
  appVersion: string;
  appCommit: string;
  eventJson: string;
  resource: string;
  reason: string;
}

// A JSON object from the event or the inputs. Its values are not checked yet.
type JsonObject = Record<string, unknown>;

const SENDER_ID = /^[1-9][0-9]{0,19}$/;
// The product and the subscription values. Exact names, so no wildcards.
const NAME = /^[A-Za-z0-9]([A-Za-z0-9._-]{0,251}[A-Za-z0-9])?$/;
// An OCI artifact revision with a SHA-256 digest, for example main@sha256:<digest>.
const OCI_REVISION = /^([A-Za-z0-9_][A-Za-z0-9._-]{0,127}@)?sha256:[a-f0-9]{64}$/;
const APP_VERSION = /^[A-Za-z0-9][A-Za-z0-9.+_-]{0,127}$/;
const APP_COMMIT = /^[a-f0-9]{40}$/;

// The controller that must report events for each supported kind.
const CONTROLLERS = new Map([
  ['Kustomization', 'kustomize-controller'],
  ['OCIRepository', 'source-controller'],
]);

const SUBSCRIPTION_KEYS: readonly string[] = ['environment', 'cluster', 'namespace', 'kustomization', 'oci_repository'];

// kustomize-controller emits this event when a new revision changed resources
// and passed the health checks, or when a failed rollout recovers. The final
// ReconciliationSucceeded event fires after every reconcile, and
// notification-controller does not send it to githubdispatch. The event has
// no reason of its own, so the action matches the message. Altinn/altinn-platform#4373
// asks Flux for a stable reason.
const HEALTH_CHECK_PASSED = 'Health check passed in ';

export function parseInputs(env: NodeJS.ProcessEnv): Inputs {
  const senderId = (env.INPUT_EXPECTED_SENDER_ID ?? '').trim();
  if (!SENDER_ID.test(senderId)) {
    throw new InvalidInputError('expected_sender_id must be the numeric id of the bot account of the dispatch App.');
  }
  const product = (env.INPUT_PRODUCT ?? '').trim();
  if (!NAME.test(product)) {
    throw new InvalidInputError('product must be a product identifier, for example dialogporten.');
  }
  return { senderId, product, subscriptions: parseSubscriptions(env.INPUT_SUBSCRIPTIONS ?? '') };
}

function parseSubscriptions(raw: string): Subscription[] {
  let list: unknown;
  try {
    list = JSON.parse(raw);
  } catch {
    list = null;
  }
  if (!Array.isArray(list) || list.length === 0) {
    throw new InvalidInputError('subscriptions must be a JSON array with at least one subscription.');
  }
  list.forEach((entry: unknown, index) => {
    const at = `subscriptions[${index}]`;
    if (!isObject(entry)) {
      throw new InvalidInputError(`${at} must be an object.`);
    }
    const extra = Object.keys(entry).find((key) => !SUBSCRIPTION_KEYS.includes(key));
    if (extra !== undefined) {
      throw new InvalidInputError(`${at} has the unknown key ${quote(extra)}.`);
    }
    for (const key of SUBSCRIPTION_KEYS) {
      if (key === 'oci_repository' && entry[key] === undefined) continue;
      if (!matches(entry[key], NAME)) {
        throw new InvalidInputError(`${at}.${key} must be an exact name. Wildcards are not permitted. Got ${quote(entry[key])}.`);
      }
    }
  });
  // The loop above checked every entry.
  return list as Subscription[];
}

// Validates a repository_dispatch event and classifies the Flux event in it.
// Throws InvalidEventError if the event does not satisfy the contract.
export function evaluate({ inputs, eventName, event, repository }: {
  inputs: Inputs;
  eventName: string;
  event: unknown;
  repository: string;
}): Result {
  if (eventName !== 'repository_dispatch') {
    throw new InvalidEventError(`The action supports only repository_dispatch events, got ${quote(eventName)}.`);
  }
  if (!isObject(event)) {
    throw new InvalidEventError('The event payload is not a JSON object.');
  }
  const senderId = field(event.sender, 'id');
  if (!Number.isSafeInteger(senderId) || String(senderId) !== inputs.senderId) {
    throw new InvalidEventError(
      `The event was sent by ${quote(field(event.sender, 'login'))} with id ${quote(senderId)}, not by the dispatch App (id ${inputs.senderId}).`,
    );
  }
  const fullName = field(event.repository, 'full_name');
  if (!sameRepository(fullName, repository)) {
    throw new InvalidEventError(`The event belongs to ${quote(fullName)}, not to ${repository}.`);
  }

  const payload = event.client_payload;
  if (!isObject(payload) || !isObject(payload.involvedObject)) {
    throw new InvalidEventError('client_payload.involvedObject must be an object.');
  }
  const object = payload.involvedObject;
  requireStrings(object, ['kind', 'name', 'namespace'], 'client_payload.involvedObject');
  requireStrings(payload, ['severity', 'reason', 'message', 'reportingController'], 'client_payload');
  const metadata: JsonObject = isObject(payload.metadata) ? payload.metadata : {};

  const resource = `${object.kind}/${object.name}.${object.namespace}`;
  if (event.action !== resource) {
    throw new InvalidEventError(`The event type ${quote(event.action)} does not match the involved object ${quote(resource)}.`);
  }
  const version = metadata.schema_version;
  if (typeof version !== 'string' || !CONTRACT_VERSIONS.includes(version)) {
    throw new InvalidEventError(`Unsupported contract version ${quote(version)}. Supported: ${CONTRACT_VERSIONS.join(', ')}.`);
  }
  if (metadata.product !== inputs.product) {
    throw new InvalidEventError(`The event is for the product ${quote(metadata.product)}, not ${quote(inputs.product)}.`);
  }
  if (!sameRepository(metadata.repository, repository)) {
    throw new InvalidEventError(`The subscription names the repository ${quote(metadata.repository)}, not ${repository}.`);
  }
  const controller = CONTROLLERS.get(object.kind);
  if (controller === undefined) {
    throw new InvalidEventError(`Unsupported resource kind ${quote(object.kind)}.`);
  }
  if (payload.reportingController !== controller) {
    throw new InvalidEventError(`${object.kind} events must come from ${controller}, got ${quote(payload.reportingController)}.`);
  }
  const subscription = inputs.subscriptions.find(
    (s) =>
      s.environment === metadata.environment &&
      s.cluster === metadata.cluster &&
      s.namespace === object.namespace &&
      (object.kind === 'Kustomization' ? s.kustomization : s.oci_repository) === object.name,
  );
  if (subscription === undefined) {
    throw new InvalidEventError(
      `No subscription matches environment ${quote(metadata.environment)}, cluster ${quote(metadata.cluster)} and ${quote(resource)}.`,
    );
  }

  const outcome = classify(payload, object.kind);
  const revision = matches(metadata.revision, OCI_REVISION) ? metadata.revision : '';
  if (outcome === 'succeeded' && revision === '') {
    throw new InvalidEventError(
      `A success event must have an OCI artifact revision with a SHA-256 digest, got ${quote(metadata.revision)}.`,
    );
  }

  const release = bindRelease(metadata, revision);
  return {
    outcome,
    environment: subscription.environment,
    cluster: subscription.cluster,
    revision,
    appVersion: release.appVersion,
    appCommit: release.appCommit,
    eventJson: JSON.stringify({ ...payload, metadata: release.metadata }),
    resource,
    reason: payload.reason,
  };
}

function classify(payload: { severity: string; reason: string; message: string; action?: unknown }, kind: string): Outcome {
  if (payload.severity === 'error') {
    return 'failed';
  }
  // The success event has the Ready reason of the health checks. Flux 2.10
  // also sets the action HealthCheck. notification-controller 1.10 sends an
  // empty action for an event from an older kustomize-controller.
  const passed =
    payload.severity === 'info' &&
    kind === 'Kustomization' &&
    payload.reason === 'Progressing' &&
    payload.message.startsWith(HEALTH_CHECK_PASSED) &&
    (!payload.action || payload.action === 'HealthCheck');
  return passed ? 'succeeded' : 'ignored';
}

// Release metadata describes one artifact. It applies only when its
// release_revision equals the revision that the controller reports. The
// returned metadata keeps only the release fields that apply.
function bindRelease(metadata: JsonObject, revision: string) {
  const bound = revision !== '' && metadata.release_revision === revision;
  const appVersion = bound && matches(metadata.app_version, APP_VERSION) ? metadata.app_version : '';
  const appCommit = bound && matches(metadata.app_commit, APP_COMMIT) ? metadata.app_commit : '';
  const published = { ...metadata };
  if (!appVersion) delete published.app_version;
  if (!appCommit) delete published.app_commit;
  if (!appVersion && !appCommit) delete published.release_revision;
  return { appVersion, appCommit, metadata: published };
}

function requireStrings<K extends string>(
  object: JsonObject,
  keys: readonly K[],
  prefix: string,
): asserts object is JsonObject & Record<K, string> {
  for (const key of keys) {
    if (typeof object[key] !== 'string') {
      throw new InvalidEventError(`${prefix}.${key} must be a string, got ${quote(object[key])}.`);
    }
  }
}

function matches(value: unknown, pattern: RegExp): value is string {
  return typeof value === 'string' && pattern.test(value);
}

// GitHub repository names are not case sensitive.
function sameRepository(value: unknown, repository: string): boolean {
  return typeof value === 'string' && value.toLowerCase() === repository.toLowerCase();
}

function isObject(value: unknown): value is JsonObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

// Reads a field of a value that may not be an object.
function field(value: unknown, key: string): unknown {
  return isObject(value) ? value[key] : undefined;
}

// Quotes a value from the event for a message. The result is one line.
export function quote(value: unknown): string {
  const text = value === undefined ? 'undefined' : JSON.stringify(value) ?? String(value);
  return text.length > 120 ? `${text.slice(0, 117)}...` : text;
}
