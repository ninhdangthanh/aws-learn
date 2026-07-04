// Poll pg_locks + pg_stat_activity mỗi giây để xem ai đang block ai.
// Chạy song song 2-3 script khác để quan sát blocking chain theo thời gian thực.
'use strict';

const { createClient } = require('./db');

const INTERVAL_MS = Number(process.env.INTERVAL_MS || 1000);

const QUERY = `
  SELECT
    blocked.pid AS blocked_pid,
    blocked.query AS blocked_query,
    blocking.pid AS blocking_pid,
    blocking.query AS blocking_query
  FROM pg_locks bl
  JOIN pg_stat_activity blocked ON blocked.pid = bl.pid
  JOIN pg_locks kl ON kl.locktype = bl.locktype
    AND kl.database IS NOT DISTINCT FROM bl.database
    AND kl.relation IS NOT DISTINCT FROM bl.relation
    AND kl.pid != bl.pid
    AND kl.granted
  JOIN pg_stat_activity blocking ON blocking.pid = kl.pid
  WHERE NOT bl.granted
`;

async function tick(client) {
  const { rows } = await client.query(QUERY);
  console.clear();
  console.log(new Date().toISOString());

  if (rows.length === 0) {
    console.log('(không có session nào đang bị block)');
    return;
  }

  for (const row of rows) {
    console.log(`PID ${row.blocked_pid} bị chặn bởi PID ${row.blocking_pid}`);
    console.log(`  blocked:  ${row.blocked_query}`);
    console.log(`  blocking: ${row.blocking_query}`);
  }
}

async function main() {
  const client = createClient();
  await client.connect();

  console.log(`Theo dõi lock mỗi ${INTERVAL_MS}ms. Ctrl+C để dừng.`);

  setInterval(() => {
    tick(client).catch((err) => console.error('Lỗi khi query pg_locks:', err.message));
  }, INTERVAL_MS);
}

main().catch((err) => {
  console.error('Lỗi:', err);
  process.exit(1);
});
