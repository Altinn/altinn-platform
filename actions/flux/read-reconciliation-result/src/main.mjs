// Runs the action: reads the inputs and the event, validates and classifies
// the event, and publishes the outputs. Invalid inputs or events fail the
// action before it publishes any output.

import { evaluate, InvalidEventError, InvalidInputError, parseInputs } from './contract.mjs';
import { errorCommand, formatOutputs } from './github.mjs';

// io provides readFile(path), appendFile(path, text) and write(text).
// Returns the exit code.
export function run(env, io) {
  let result;
  try {
    const inputs = parseInputs(env);
    for (const name of ['GITHUB_EVENT_NAME', 'GITHUB_EVENT_PATH', 'GITHUB_REPOSITORY', 'GITHUB_OUTPUT']) {
      if (!env[name]) {
        throw new InvalidInputError(`${name} is not set. Run the action in a GitHub Actions job.`);
      }
    }
    let text;
    try {
      text = io.readFile(env.GITHUB_EVENT_PATH);
    } catch {
      throw new InvalidInputError('The action cannot read the event file in GITHUB_EVENT_PATH.');
    }
    let event;
    try {
      event = JSON.parse(text);
    } catch {
      throw new InvalidEventError('The event payload in GITHUB_EVENT_PATH is not valid JSON.');
    }
    result = evaluate({ inputs, eventName: env.GITHUB_EVENT_NAME, event, repository: env.GITHUB_REPOSITORY });
  } catch (error) {
    if (error instanceof InvalidInputError) {
      io.write(errorCommand('Invalid configuration of read-reconciliation-result', error.message));
      return 1;
    }
    if (error instanceof InvalidEventError) {
      io.write(errorCommand('Rejected Flux result event', error.message));
      return 1;
    }
    throw error;
  }

  io.appendFile(
    env.GITHUB_OUTPUT,
    formatOutputs({
      outcome: result.outcome,
      environment: result.environment,
      cluster: result.cluster,
      revision: result.revision,
      app_version: result.appVersion,
      app_commit: result.appCommit,
      event_json: result.eventJson,
    }),
  );
  if (env.GITHUB_STEP_SUMMARY) {
    io.appendFile(env.GITHUB_STEP_SUMMARY, summary(result));
  }
  // Every value in this line passed a pattern check, so the line cannot
  // contain a line break or start a workflow command.
  io.write(
    `Outcome ${result.outcome}: ${result.resource}, environment ${result.environment}, ` +
      `cluster ${result.cluster}, reason ${result.reason}, revision ${result.revision || 'none'}\n`,
  );
  return 0;
}

function summary(result) {
  const cell = (value) => (value ? `\`${value}\`` : '-');
  return [
    '### Flux reconciliation result',
    '',
    '| Outcome | Environment | Cluster | Resource | Reason | Revision | Application |',
    '|---|---|---|---|---|---|---|',
    `| ${result.outcome} | ${result.environment} | ${result.cluster} | ${cell(result.resource)} | ${result.reason} | ${cell(result.revision)} | ${cell(result.appVersion)} |`,
    '',
  ].join('\n');
}
