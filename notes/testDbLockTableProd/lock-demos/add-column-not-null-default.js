// ALTER TABLE ... ADD COLUMN ... NOT NULL DEFAULT ...: lấy ACCESS EXCLUSIVE
// lock — chặn CẢ read lẫn write trong lúc chờ tới lượt.
// Chạy song song với long-transaction.js để thấy nó xếp hàng phía sau.
'use strict';

const { createClient } = require('./db');

const LOCK_TIMEOUT = process.env.LOCK_TIMEOUT || '10s';

async function main() {
  const client = createClient();
  await client.connect();

  await client.query(`SET lock_timeout = '${LOCK_TIMEOUT}'`);
  await client.query('ALTER TABLE lock_test_orders DROP COLUMN IF EXISTS is_priority');

  console.log(`ALTER TABLE ADD COLUMN NOT NULL DEFAULT... (lock_timeout=${LOCK_TIMEOUT})`);
  const startedAt = Date.now();

  try {
    await client.query(
      'ALTER TABLE lock_test_orders ADD COLUMN is_priority boolean NOT NULL DEFAULT false'
    );
    console.log(`Xong sau ${((Date.now() - startedAt) / 1000).toFixed(1)}s.`);
  } catch (err) {
    console.error(`Thất bại sau ${((Date.now() - startedAt) / 1000).toFixed(1)}s:`, err.message);
  } finally {
    await client.end();
  }
}

main().catch((err) => {
  console.error('Lỗi:', err);
  process.exit(1);
});
