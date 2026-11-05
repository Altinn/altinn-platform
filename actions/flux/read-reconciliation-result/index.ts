// Entry point of the read-reconciliation-result action.

import { appendFileSync, readFileSync } from 'node:fs';
import { run } from './src/main.ts';

process.exitCode = run(process.env, {
  readFile: (path) => readFileSync(path, 'utf8'),
  appendFile: (path, text) => appendFileSync(path, text, 'utf8'),
  write: (text) => process.stdout.write(text),
});
