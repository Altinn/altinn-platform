// Runs the action: reads the inputs and the event, validates and classifies
// the event, and publishes the outputs. Invalid inputs or events fail the
// action before it publishes any output.

import { evaluate, InvalidEventError, InvalidInputError, parseInputs, quote, type Result } from './contract.ts';
import { errorCommand, formatOutputs } from './github.ts';

// The files and the log of the runner. The tests replace them.
export interface IO {
  readFile(path: string): string;
  appendFile(path: string, text: string): void;
  write(text: string): void;
}

// Returns the exit code.
export function run(env: NodeJS.ProcessEnv, io: IO): number {
  let result: Result;
  let outputPath: string;
  try {
    const inputs = parseInputs(env);
    const eventName = runnerVariable(env, 'GITHUB_EVENT_NAME');
    const eventPath = runnerVariable(env, 'GITHUB_EVENT_PATH');
    const repository = runnerVariable(env, 'GITHUB_REPOSITORY');
    outputPath = runnerVariable(env, 'GITHUB_OUTPUT');
    let text: string;
    try {
      text = io.readFile(eventPath);
    } catch {
      throw new InvalidInputError('The action cannot read the event file in GITHUB_EVENT_PATH.');
    }
    let event: unknown;
    try {
      event = JSON.parse(text);
    } catch {
      throw new InvalidEventError('The event payload in GITHUB_EVENT_PATH is not valid JSON.');
    }
    result = evaluate({ inputs, eventName, event, repository });
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
    outputPath,
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
  // The reason comes from the event, so it is quoted onto one line. The other
  // values equal inputs or constants, or passed a pattern check.
  io.write(
    `Outcome ${result.outcome}: ${result.resource}, environment ${result.environment}, ` +
      `cluster ${result.cluster}, reason ${quote(result.reason)}, revision ${result.revision || 'none'}\n`,
  );
  return 0;
}

function runnerVariable(env: NodeJS.ProcessEnv, name: string): string {
  const value = env[name];
  if (!value) {
    throw new InvalidInputError(`${name} is not set. Run the action in a GitHub Actions job.`);
  }
  return value;
}
