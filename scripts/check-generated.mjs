import { readFile, readdir } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const generated = [
  'services/agent-service/internal/adapters/inbound/http/gen/api.gen.go',
  'apps/admin-web/src/shared/api/generated/schema.d.ts',
];
const sqlDirectory = 'services/agent-service/internal/adapters/outbound/postgres/sqlcgen';

async function snapshot() {
  const files = [...generated];
  try {
    for (const entry of await readdir(`${root}${sqlDirectory}`)) {
      if (entry.endsWith('.go')) files.push(`${sqlDirectory}/${entry}`);
    }
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
  }
  return new Map(await Promise.all(files.map(async (file) => [file, await readFile(`${root}${file}`)])));
}

const before = await snapshot();
const result = spawnSync('make', ['gen'], { cwd: root, stdio: 'inherit' });
if (result.error) throw result.error;
if (result.status !== 0) process.exit(result.status ?? 1);
const after = await snapshot();
const changed = [...new Set([...before.keys(), ...after.keys()])].filter((file) =>
  !before.has(file) || !after.has(file) || !before.get(file).equals(after.get(file)),
);
if (changed.length) {
  console.error(`Mã sinh chưa đồng bộ; đã cập nhật các file sau:\n${changed.join('\n')}`);
  process.exit(1);
}
console.log('Mã sinh đã đồng bộ và không thay đổi sau khi sinh lại.');
