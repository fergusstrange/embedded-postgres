// Reproduce the detached, hidden Windows spawn used by Node consumers.
import assert from 'node:assert/strict';
import { execFileSync, spawn } from 'node:child_process';
import { mkdtempSync, readdirSync, rmSync } from 'node:fs';
import { createConnection } from 'node:net';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { createInterface } from 'node:readline';

const work = mkdtempSync(join(tmpdir(), 'ep-node-launch-'));
const helper = process.env.EP_SUPERVISOR;
const binaries = process.env.EP_TEST_BIN;
assert.ok(helper && binaries, 'build the CLI and set EP_SUPERVISOR and EP_TEST_BIN');
let ready = false;
let stopped = false;
let port;
let failure;
let stderr = '';
const child = spawn(helper, ['run', '--binaries', dirname(binaries), '--work-dir', work, '--json', '--parent-stdin'], {
  stdio: ['pipe', 'pipe', 'pipe'], windowsHide: true, detached: process.platform === 'win32', shell: false,
});
const timer = setTimeout(() => { failure = new Error('Node-launched CLI timed out'); child.kill('SIGKILL'); }, 90_000);
child.stderr.on('data', chunk => { stderr = (stderr + chunk).slice(-8192); });
const lines = createInterface({ input: child.stdout });
lines.on('line', line => {
  try {
    const event = JSON.parse(line);
    if (event.event === 'error') throw new Error(event.error);
    if (event.event === 'ready') {
      ready = true;
      port = event.port;
      const suffix = process.platform === 'win32' ? '.exe' : '';
      const url = new URL(event.connection_url);
      const env = { ...process.env, PGHOST: url.hostname, PGPORT: url.port,
        PGUSER: decodeURIComponent(url.username), PGPASSWORD: decodeURIComponent(url.password),
        PGDATABASE: decodeURIComponent(url.pathname.slice(1)) };
      const result = execFileSync(join(binaries, `psql${suffix}`), ['-Atc', 'SELECT 42;'], { env, encoding: 'utf8', timeout: 10_000 });
      assert.equal(result.trim(), '42');
      child.stdin.end();
    }
    if (event.event === 'stopped') stopped = true;
  } catch (error) { failure = error; child.stdin.end(); }
});
try {
  const exit = await new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('close', (code, signal) => resolve({ code, signal }));
  });
  if (failure) throw failure;
  assert.deepEqual(exit, { code: 0, signal: null }, stderr);
  assert.ok(ready && stopped, 'CLI must report both readiness and cleanup');
  assert.deepEqual(readdirSync(work), []);
  await new Promise((resolve, reject) => {
    const socket = createConnection({ host: '127.0.0.1', port });
    socket.once('connect', () => { socket.destroy(); reject(new Error('database survived CLI exit')); });
    socket.once('error', resolve);
    socket.setTimeout(2000, () => { socket.destroy(); reject(new Error('cleanup check timed out')); });
  });
  console.log('Detached hidden Node launch: SQL, parent-stdin shutdown, port and workspace cleanup passed.');
} finally { clearTimeout(timer); lines.close(); rmSync(work, { recursive: true, force: true }); }
