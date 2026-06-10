import test from 'node:test';
import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';

import { createStartAllSupervisor, installSignalShutdownHandlers } from './start-all-supervisor.mjs';

class FakeChild extends EventEmitter {
  constructor() {
    super();
    this.stdout = new EventEmitter();
    this.stderr = new EventEmitter();
    this.killed = false;
    this.killSignals = [];
  }

  kill(signal) {
    this.killed = true;
    this.killSignals.push(signal);
    return true;
  }
}

test('startAll stops remaining children when a service exits non-zero', () => {
  const spawned = [];
  const logs = [];
  const fatalExits = [];
  const supervisor = createStartAllSupervisor({
    platform: 'win32',
    env: { NODE_ENV: 'test' },
    logger: { log: (message) => logs.push(message) },
    stdout: { write: () => {} },
    stderr: { write: () => {} },
    onFatalExit: (code, item) => fatalExits.push([code, item.name]),
    spawn: (command, args, options) => {
      const child = new FakeChild();
      spawned.push({ command, args, options, child });
      return child;
    }
  });

  supervisor.start();

  assert.deepEqual(spawned.map((entry) => entry.command), ['npm.cmd', 'npm.cmd', 'npm.cmd']);
  assert.equal(spawned[0].options.env.NODE_ENV, 'test');

  spawned[0].child.emit('exit', 1, null);

  assert.equal(spawned[0].child.killed, false);
  assert.deepEqual(spawned[1].child.killSignals, ['SIGTERM']);
  assert.deepEqual(spawned[2].child.killSignals, ['SIGTERM']);
  assert.deepEqual(fatalExits, [[1, 'network']]);
  assert.equal(supervisor.exitCode, 1);
  assert.match(logs.at(-1), /\[network\] exited with code 1/);
});

test('startAll stops remaining children when a service exits zero unexpectedly', () => {
  const spawned = [];
  const fatalExits = [];
  const supervisor = createStartAllSupervisor({
    logger: { log: () => {} },
    stdout: { write: () => {} },
    stderr: { write: () => {} },
    onFatalExit: (code, item) => fatalExits.push([code, item.name]),
    spawn: () => {
      const child = new FakeChild();
      spawned.push(child);
      return child;
    }
  });

  supervisor.start();

  spawned[0].emit('exit', 0, null);

  assert.equal(spawned[0].killed, false);
  assert.deepEqual(spawned[1].killSignals, ['SIGTERM']);
  assert.deepEqual(spawned[2].killSignals, ['SIGTERM']);
  assert.deepEqual(fatalExits, [[1, 'network']]);
  assert.equal(supervisor.exitCode, 1);
});

test('startAll stops remaining children when a service emits spawn error', () => {
  const spawned = [];
  const logs = [];
  const fatalExits = [];
  const supervisor = createStartAllSupervisor({
    logger: { log: (message) => logs.push(message) },
    stdout: { write: () => {} },
    stderr: { write: () => {} },
    onFatalExit: (code, item, signal) => fatalExits.push([code, item.name, signal]),
    spawn: () => {
      const child = new FakeChild();
      spawned.push(child);
      return child;
    }
  });

  supervisor.start();

  spawned[0].emit('error', new Error('spawn ENOENT'));

  assert.equal(spawned[0].killed, false);
  assert.deepEqual(spawned[1].killSignals, ['SIGTERM']);
  assert.deepEqual(spawned[2].killSignals, ['SIGTERM']);
  assert.deepEqual(fatalExits, [[1, 'network', null]]);
  assert.equal(supervisor.exitCode, 1);
  assert.match(logs.at(-1), /\[network\] failed to start: spawn ENOENT/);
});

test('startAll prefixes every line from multiline stdout chunks', () => {
  const spawned = [];
  const writes = [];
  const supervisor = createStartAllSupervisor({
    logger: { log: () => {} },
    stdout: { write: (chunk) => writes.push(chunk) },
    stderr: { write: () => {} },
    spawn: () => {
      const child = new FakeChild();
      spawned.push(child);
      return child;
    }
  });

  supervisor.start();
  spawned[0].stdout.emit('data', 'ready\nlistening\n');

  assert.equal(writes.join(''), '[network] ready\n[network] listening\n');
});

test('startAll runs workspace commands from the configured repo root', () => {
  const spawned = [];
  const supervisor = createStartAllSupervisor({
    cwd: 'C:\\repo\\fufu-toolskit',
    logger: { log: () => {} },
    stdout: { write: () => {} },
    stderr: { write: () => {} },
    spawn: (command, args, options) => {
      const child = new FakeChild();
      spawned.push({ command, args, options });
      return child;
    }
  });

  supervisor.start();

  assert.equal(spawned.length, 3);
  assert.deepEqual(spawned.map((entry) => entry.options.cwd), [
    'C:\\repo\\fufu-toolskit',
    'C:\\repo\\fufu-toolskit',
    'C:\\repo\\fufu-toolskit'
  ]);
});

test('stopAll waits until every child exits', async () => {
  const spawned = [];
  const supervisor = createStartAllSupervisor({
    logger: { log: () => {} },
    stdout: { write: () => {} },
    stderr: { write: () => {} },
    spawn: () => {
      const child = new FakeChild();
      spawned.push(child);
      return child;
    }
  });

  supervisor.start();

  let resolved = false;
  const stopped = supervisor.stopAll().then(() => {
    resolved = true;
  });

  await Promise.resolve();
  assert.equal(resolved, false);

  spawned[0].emit('exit', null, 'SIGTERM');
  await Promise.resolve();
  assert.equal(resolved, false);

  spawned[1].emit('exit', null, 'SIGTERM');
  spawned[2].emit('exit', null, 'SIGTERM');
  await stopped;

  assert.equal(resolved, true);
});

test('signal shutdown waits for children before exiting', async () => {
  let resolveExited;
  const stopped = [];
  const exits = [];
  const clearedTimers = [];
  const handlers = {};
  const supervisor = {
    stopAll: () => stopped.push('stop'),
    waitForAllExited: () => new Promise((resolve) => {
      resolveExited = resolve;
    })
  };
  const fakeProcess = {
    exitCode: undefined,
    on: (signal, handler) => {
      handlers[signal] = handler;
    },
    exit: (code) => {
      exits.push(code);
    }
  };

  installSignalShutdownHandlers(supervisor, {
    process: fakeProcess,
    setTimeout: () => 'timer-1',
    clearTimeout: (timer) => clearedTimers.push(timer),
    graceMs: 50
  });

  handlers.SIGINT();

  assert.deepEqual(stopped, ['stop']);
  assert.equal(fakeProcess.exitCode, 130);
  assert.deepEqual(exits, []);

  resolveExited();
  await Promise.resolve();

  assert.deepEqual(clearedTimers, ['timer-1']);
  assert.deepEqual(exits, [130]);
});
