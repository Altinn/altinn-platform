// Helpers for the files and workflow commands of the GitHub Actions runner.
// The action has no dependencies, so it does not use @actions/core.

import { randomUUID } from 'node:crypto';

// Formats step outputs for the file in GITHUB_OUTPUT. Each value gets a random
// delimiter that it does not contain, so a value with line breaks cannot end
// its output early or add another output.
export function formatOutputs(outputs) {
  let text = '';
  for (const [name, value] of Object.entries(outputs)) {
    const content = String(value);
    let delimiter;
    do {
      delimiter = `ghadelimiter_${randomUUID()}`;
    } while (content.includes(delimiter));
    text += `${name}<<${delimiter}\n${content}\n${delimiter}\n`;
  }
  return text;
}

// Escapes the message of a workflow command, so the message stays on one line.
export function escapeData(value) {
  return String(value).replaceAll('%', '%25').replaceAll('\r', '%0D').replaceAll('\n', '%0A');
}

export function escapeProperty(value) {
  return escapeData(value).replaceAll(':', '%3A').replaceAll(',', '%2C');
}

export function errorCommand(title, message) {
  return `::error title=${escapeProperty(title)}::${escapeData(message)}\n`;
}
