import { spawn } from 'node:child_process';
import { createStartAllSupervisor, installSignalShutdownHandlers } from './start-all-supervisor.mjs';

const supervisor = createStartAllSupervisor({
  spawn,
  onFatalExit: (code) => {
    process.exitCode = code;
  }
});

supervisor.start();
installSignalShutdownHandlers(supervisor);
