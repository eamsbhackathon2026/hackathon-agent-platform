import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const name = process.env.MIGRATION_NAME ?? '';
if (!/^[a-z][a-z0-9_]*$/.test(name)) {
  console.error('Dùng make migrate-new name=ten_migration (chữ thường, số và dấu gạch dưới).');
  process.exit(1);
}
const result = spawnSync('go', ['tool', 'goose', '-dir', 'migrations', '-s', 'create', name, 'sql'], {
  cwd: fileURLToPath(new URL('../services/agent-service/', import.meta.url)),
  stdio: 'inherit',
});
if (result.error) throw result.error;
process.exit(result.status ?? 1);
