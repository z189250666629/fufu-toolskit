import { fileURLToPath } from 'node:url';

export const START_COMMANDS = Object.freeze([
  Object.freeze({
    name: 'tool-site',
    command: 'npm',
    args: Object.freeze(['--prefix', 'apps/fufu-tool-site', 'start'])
  })
]);

export const START_CWD = fileURLToPath(new URL('../', import.meta.url));

export const ROOT_START_SCRIPT = 'node scripts/start-all.mjs';
