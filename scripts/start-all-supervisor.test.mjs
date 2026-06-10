import test from 'node:test';
import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';

import { createStartAllSupervisor } from './start-all-supervisor.mjs';

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
