import { readdir } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const serviceDir = fileURLToPath(new URL('../services/agent-service/', import.meta.url));
const migrations = await readdir(`${serviceDir}migrations`);
if (!migrations.some((file) => file.endsWith('.sql'))) {
  console.log('Chưa có migration SQL; sqlc sẽ sinh mã khi có schema nghiệp vụ.');
  process.exit(0);
}
const result = spawnSync('go', ['tool', 'sqlc', 'generate'], { cwd: serviceDir, stdio: 'inherit' });
if (result.error) throw result.error;
process.exit(result.status ?? 1);
