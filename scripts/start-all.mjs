import { spawn } from 'node:child_process';
import { createStartAllSupervisor } from './start-all-supervisor.mjs';

const supervisor = createStartAllSupervisor({
  spawn,
  onFatalExit: (code) => {
    process.exitCode = code;
  }
});

supervisor.start();

process.on('SIGINT', () => {
  supervisor.stopAll();
  process.exit(130);
});
process.on('SIGTERM', () => {
  supervisor.stopAll();
  process.exit(143);
});
