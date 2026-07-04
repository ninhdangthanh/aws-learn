// CREATE INDEX thường (không CONCURRENTLY): lấy SHARE lock, chặn
// INSERT/UPDATE/DELETE cho tới khi build xong index.
// Chạy song song với long-transaction.js để thấy nó phải chờ.
'use strict';

const { createClient } = require('./db');

const LOCK_TIMEOUT = process.env.LOCK_TIMEOUT || '10s';

async function main() {
  const client = createClient();
  await client.connect();

  await client.query(`SET lock_timeout = '${LOCK_TIMEOUT}'`);
  await client.query('DROP INDEX IF EXISTS idx_lock_test_orders_email');

  console.log(`CREATE INDEX (blocking)... (lock_timeout=${LOCK_TIMEOUT})`);
  const startedAt = Date.now();

  try {
    await client.query(
      'CREATE INDEX idx_lock_test_orders_email ON lock_test_orders(email)'
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
