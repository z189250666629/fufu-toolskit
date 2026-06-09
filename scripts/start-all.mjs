import { spawn } from 'node:child_process';

const commands = [
  { name: 'network', command: 'npm', args: ['--prefix', 'apps/network-detect', 'start'] },
  { name: 'act', command: 'npm', args: ['--prefix', 'apps/fufu-act', 'start'] },
  { name: 'y2k', command: 'npm', args: ['--prefix', 'apps/y2k-nav', 'start'] },
];

const children = [];
function npmCommand(command) { return process.platform === 'win32' && command === 'npm' ? 'npm.cmd' : command; }
for (const item of commands) {
  const child = spawn(npmCommand(item.command), item.args, { stdio: 'pipe', shell: false, env: process.env });
  children.push(child);
  child.stdout.on('data', (chunk) => process.stdout.write(`[${item.name}] ${chunk}`));
  child.stderr.on('data', (chunk) => process.stderr.write(`[${item.name}] ${chunk}`));
  child.on('exit', (code, signal) => console.log(`[${item.name}] exited with ${signal ? `signal ${signal}` : `code ${code}`}`));
}
function stopAll() { for (const child of children) if (!child.killed) child.kill('SIGTERM'); }
process.on('SIGINT', () => { stopAll(); process.exit(130); });
process.on('SIGTERM', () => { stopAll(); process.exit(143); });
