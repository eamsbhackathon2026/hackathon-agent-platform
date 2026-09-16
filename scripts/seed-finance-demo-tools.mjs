// Tạo API connection và HTTP tool cho năm service finance-demo.
//
// Cách dùng:
//   AGENT_TOKEN=<access token> node scripts/seed-finance-demo-tools.mjs [--dry-run] [--force]
//
// Biến môi trường:
//   AGENT_API            Địa chỉ API Agent Platform, mặc định http://127.0.0.1:8080
//   AGENT_TOKEN          Access token JWT của người dùng owner/admin (bắt buộc)
//   FINANCE_<TÊN>_URL    Ghi đè địa chỉ từng service, xem bảng SERVICES bên dưới
//
// Chạy lại được: tool đã có mà giống hệt thì bỏ qua, khác thì in khác biệt và chỉ
// ghi đè khi truyền --force.

import { buildToolPayloads, toolDiffers } from './finance-demo-tool-mapping.mjs';

const SERVICES = [
  { slug: 'finance-transaction', displayName: 'Giao dịch', env: 'FINANCE_TRANSACTION_URL', fallback: 'http://127.0.0.1:8081' },
  { slug: 'finance-customer', displayName: 'Hồ sơ khách hàng', env: 'FINANCE_CUSTOMER_URL', fallback: 'http://127.0.0.1:8082' },
  { slug: 'finance-risk', displayName: 'Chấm điểm rủi ro', env: 'FINANCE_RISK_URL', fallback: 'http://127.0.0.1:8083' },
  { slug: 'finance-scam', displayName: 'Tri thức lừa đảo', env: 'FINANCE_SCAM_URL', fallback: 'http://127.0.0.1:8084' },
  { slug: 'finance-action', displayName: 'Hành động và phản hồi', env: 'FINANCE_ACTION_URL', fallback: 'http://127.0.0.1:8085' },
];

const api = (process.env.AGENT_API ?? 'http://127.0.0.1:8080').replace(/\/+$/u, '');
const token = process.env.AGENT_TOKEN ?? '';
const dryRun = process.argv.includes('--dry-run');
const force = process.argv.includes('--force');

if (!token && !dryRun) {
  console.error('Thiếu AGENT_TOKEN. Đăng nhập rồi truyền access token, hoặc chạy với --dry-run để chỉ xem trước.');
  process.exit(1);
}

async function readJson(url, label) {
  let response;
  try {
    response = await fetch(url, { signal: AbortSignal.timeout(15_000) });
  } catch (cause) {
    // Nguyên nhân hay gặp nhất là tunnel port-forward đã đóng, nên nói thẳng ra.
    const code = cause?.cause?.code ?? cause?.name ?? 'lỗi mạng';
    throw new Error(`${label}: không kết nối được ${url} (${code}). Kiểm tra kubectl port-forward còn chạy không.`);
  }
  if (!response.ok) throw new Error(`${label}: ${url} trả ${response.status}`);
  return response.json();
}

async function callApi(path, { method = 'GET', body } = {}) {
  const response = await fetch(`${api}${path}`, {
    method,
    headers: { Authorization: `Bearer ${token}`, ...(body ? { 'Content-Type': 'application/json' } : {}) },
    ...(body ? { body: JSON.stringify(body) } : {}),
    signal: AbortSignal.timeout(30_000),
  });
  const text = await response.text();
  if (!response.ok) {
    // Lỗi của API ở dạng Problem Details; in nguyên văn để biết trường nào sai.
    throw new Error(`${method} ${path} trả ${response.status}: ${text.slice(0, 600)}`);
  }
  return text ? JSON.parse(text) : null;
}

/** Đọc hết các trang của một collection có con trỏ. */
async function listAll(path) {
  const items = [];
  let cursor = null;
  do {
    const query = new URLSearchParams({ limit: '100', ...(cursor ? { cursor } : {}) });
    const page = await callApi(`${path}?${query}`);
    items.push(...page.items);
    cursor = page.next_cursor;
  } while (cursor);
  return items;
}

async function ensureConnection(service, existing) {
  const baseUrl = process.env[service.env] ?? service.fallback;
  const found = existing.find((item) => item.slug === service.slug);
  if (found) {
    if (found.base_url !== baseUrl) {
      console.log(`  kết nối ${service.slug}: địa chỉ hiện tại ${found.base_url}, script muốn ${baseUrl}`);
      if (force && !dryRun) {
        await callApi(`/v1/api-connections/${found.id}`, { method: 'PATCH', body: { base_url: baseUrl } });
        console.log('    đã cập nhật địa chỉ');
      } else {
        console.log('    giữ nguyên; chạy với --force để đổi');
      }
    }
    return { id: found.id, baseUrl };
  }
  if (dryRun) {
    console.log(`  kết nối ${service.slug}: sẽ tạo mới trỏ ${baseUrl}`);
    return { id: `dry-run-${service.slug}`, baseUrl };
  }
  const created = await callApi('/v1/api-connections', {
    method: 'POST',
    body: { slug: service.slug, display_name: service.displayName, base_url: baseUrl },
  });
  console.log(`  kết nối ${service.slug}: đã tạo, trỏ ${baseUrl}`);
  return { id: created.id, baseUrl };
}

async function syncTool(desired, existingBySlug, counters) {
  const existing = existingBySlug.get(desired.slug);
  if (!existing) {
    if (dryRun) {
      console.log(`  + ${desired.slug} (${desired.method} ${desired.url_template}, ${desired.params.length} tham số)`);
      counters.created += 1;
      return;
    }
    await callApi('/v1/tools', { method: 'POST', body: desired });
    console.log(`  + ${desired.slug}`);
    counters.created += 1;
    return;
  }
  if (!toolDiffers(existing, desired)) {
    counters.unchanged += 1;
    return;
  }
  console.log(`  ~ ${desired.slug}: cấu hình trên máy chủ khác với bản sinh ra`);
  if (!force) {
    console.log('    bỏ qua; chạy với --force để ghi đè');
    counters.drifted += 1;
    return;
  }
  if (dryRun) {
    console.log('    sẽ ghi đè');
    counters.updated += 1;
    return;
  }
  const { slug, ...patch } = desired;
  void slug;
  await callApi(`/v1/tools/${existing.id}`, { method: 'PATCH', body: patch });
  console.log('    đã ghi đè');
  counters.updated += 1;
}

async function main() {
  const connections = dryRun && !token ? [] : await listAll('/v1/api-connections');
  const existingTools = dryRun && !token ? [] : await listAll('/v1/tools');
  const existingBySlug = new Map(existingTools.map((tool) => [tool.slug, tool]));
  const counters = { created: 0, updated: 0, unchanged: 0, drifted: 0 };
  const notes = [];

  for (const service of SERVICES) {
    const baseUrl = process.env[service.env] ?? service.fallback;
    console.log(`\n${service.displayName} (${baseUrl})`);
    const [manifest, openapi] = await Promise.all([
      readJson(`${baseUrl}/agent/tools`, service.slug),
      readJson(`${baseUrl}/openapi.json`, service.slug),
    ]);
    const connection = await ensureConnection(service, connections);
    const built = buildToolPayloads({ manifest, openapi, connectionId: connection.id });
    notes.push(...built.notes);
    for (const tool of built.tools) await syncTool(tool, existingBySlug, counters);
  }

  console.log(`\nTạo mới ${counters.created} · ghi đè ${counters.updated} · giữ nguyên ${counters.unchanged} · lệch chưa xử lý ${counters.drifted}`);
  if (notes.length) {
    console.log('\nCần chú ý:');
    for (const note of notes) console.log(`  - ${note}`);
  }
  if (dryRun) console.log('\nĐây là lần chạy thử, không ghi gì lên máy chủ.');
}

main().catch((error) => {
  console.error(`\nDừng lại: ${error.message}`);
  process.exit(1);
});
