// Helpers for the files and workflow commands of the GitHub Actions runner.
// The action has no dependencies, so it does not use @actions/core.

import { randomUUID } from 'node:crypto';

// Formats step outputs for the file in GITHUB_OUTPUT. Each value gets a random
// delimiter that it does not contain, so a value with line breaks cannot end
// its output early or add another output.
export function formatOutputs(outputs: Record<string, string>): string {
  let text = '';
  for (const [name, value] of Object.entries(outputs)) {
    let delimiter: string;
    do {
      delimiter = `ghadelimiter_${randomUUID()}`;
    } while (value.includes(delimiter));
    text += `${name}<<${delimiter}\n${value}\n${delimiter}\n`;
  }
  return text;
}

// Escapes the message of a workflow command, so the message stays on one line.
export function escapeData(value: string): string {
  return value.replaceAll('%', '%25').replaceAll('\r', '%0D').replaceAll('\n', '%0A');
}

export function escapeProperty(value: string): string {
  return escapeData(value).replaceAll(':', '%3A').replaceAll(',', '%2C');
}

export function errorCommand(title: string, message: string): string {
  return `::error title=${escapeProperty(title)}::${escapeData(message)}\n`;
}
