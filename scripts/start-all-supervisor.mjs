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
  let stopping = false;
  let exitCode = 0;

  function prefixWrite(stream, name, chunk) {
    stream.write(`[${name}] ${chunk}`);
  }

  function stopAll(signal = 'SIGTERM') {
    stopping = true;
    for (const child of children) {
      if (!exited.has(child) && !child.killed) {
        child.kill(signal);
      }
    }
  }

  function handleExit(item, child, code, signal) {
    exited.add(child);
    logger.log(`[${item.name}] exited with ${signal ? `signal ${signal}` : `code ${code}`}`);

    const failed = (typeof code === 'number' && code !== 0) || signal;
    if (!stopping && failed) {
      exitCode = typeof code === 'number' && code > 0 ? code : 1;
      stopAll();
      onFatalExit(exitCode, item, signal);
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
      child.stdout?.on('data', (chunk) => prefixWrite(stdout, item.name, chunk));
      child.stderr?.on('data', (chunk) => prefixWrite(stderr, item.name, chunk));
      child.on('exit', (code, signal) => handleExit(item, child, code, signal));
    }
    return children;
  }

  return {
    get children() {
      return children;
    },
    get exitCode() {
      return exitCode;
    },
    start,
    stopAll
  };
}
