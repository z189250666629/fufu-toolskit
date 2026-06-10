export const DEFAULT_COMMANDS = [
  { name: 'network', command: 'npm', args: ['--prefix', 'apps/network-detect', 'start'] },
  { name: 'act', command: 'npm', args: ['--prefix', 'apps/fufu-act', 'start'] },
  { name: 'y2k', command: 'npm', args: ['--prefix', 'apps/y2k-nav', 'start'] }
];

export function npmCommand(command, platform) {
  return platform === 'win32' && command === 'npm' ? 'npm.cmd' : command;
}

export function createStartAllSupervisor({
  commands = DEFAULT_COMMANDS,
  spawn,
  platform = process.platform,
  env = process.env,
  stdout = process.stdout,
  stderr = process.stderr,
  logger = console,
  onFatalExit = () => {}
} = {}) {
  if (typeof spawn !== 'function') {
    throw new TypeError('spawn is required');
  }

  const children = [];
  const exited = new Set();
  const exitWaiters = [];
  let stopping = false;
  let exitCode = 0;

  function allChildrenExited() {
    return children.every((child) => exited.has(child));
  }

  function notifyExitWaiters() {
    if (!allChildrenExited()) return;
    while (exitWaiters.length > 0) {
      exitWaiters.shift()();
    }
  }

  function createPrefixWriter(stream, name) {
    let atLineStart = true;
    return (chunk) => {
      const text = String(chunk);
      let start = 0;
      while (start < text.length) {
        if (atLineStart) {
          stream.write(`[${name}] `);
          atLineStart = false;
        }
        const newlineIndex = text.indexOf('\n', start);
        if (newlineIndex === -1) {
          stream.write(text.slice(start));
          break;
        }
        stream.write(text.slice(start, newlineIndex + 1));
        atLineStart = true;
        start = newlineIndex + 1;
      }
    };
  }

  function stopAll(signal = 'SIGTERM') {
    stopping = true;
    for (const child of children) {
      if (!exited.has(child) && !child.killed) {
        child.kill(signal);
      }
    }
    return waitForAllExited();
  }

  function handleExit(item, child, code, signal) {
    exited.add(child);
    logger.log(`[${item.name}] exited with ${signal ? `signal ${signal}` : `code ${code}`}`);
    notifyExitWaiters();

    if (!stopping) {
      exitCode = typeof code === 'number' && code > 0 ? code : 1;
      stopAll();
      onFatalExit(exitCode, item, signal);
    }
  }

  function handleError(item, child, error) {
    exited.add(child);
    logger.log(`[${item.name}] failed to start: ${error?.message || error}`);
    notifyExitWaiters();

    if (!stopping) {
      exitCode = 1;
      stopAll();
      onFatalExit(exitCode, item, null);
    }
  }

  function start() {
    for (const item of commands) {
      const child = spawn(npmCommand(item.command, platform), item.args, {
        stdio: 'pipe',
        shell: false,
        env
      });
      children.push(child);
      child.stdout?.on('data', createPrefixWriter(stdout, item.name));
      child.stderr?.on('data', createPrefixWriter(stderr, item.name));
      child.on('exit', (code, signal) => handleExit(item, child, code, signal));
      child.on('error', (error) => handleError(item, child, error));
    }
    return children;
  }

  function waitForAllExited() {
    if (allChildrenExited()) {
      return Promise.resolve();
    }
    return new Promise((resolve) => {
      exitWaiters.push(resolve);
    });
  }

  return {
    get children() {
      return children;
    },
    get exitCode() {
      return exitCode;
    },
    start,
    stopAll,
    waitForAllExited
  };
}

export function installSignalShutdownHandlers(supervisor, {
  process = globalThis.process,
  setTimeout = globalThis.setTimeout,
  clearTimeout = globalThis.clearTimeout,
  graceMs = 5000
} = {}) {
  let shuttingDown = false;

  function shutdown(code) {
    if (shuttingDown) return;
    shuttingDown = true;
    process.exitCode = code;
    supervisor.stopAll();
    const timer = setTimeout(() => {
      process.exit(code);
    }, graceMs);
    supervisor.waitForAllExited().then(() => {
      clearTimeout(timer);
      process.exit(code);
    });
  }

  process.on('SIGINT', () => shutdown(130));
  process.on('SIGTERM', () => shutdown(143));
}
