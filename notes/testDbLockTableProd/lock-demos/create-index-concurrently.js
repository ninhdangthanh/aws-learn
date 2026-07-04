// CREATE INDEX CONCURRENTLY: lấy SHARE UPDATE EXCLUSIVE lock, không chặn
// INSERT/UPDATE/DELETE bình thường. Chạy song song với long-transaction.js
// để so sánh với create-index.js (bản blocking).
//
// Lưu ý: không được chạy trong 1 transaction block, nên script này KHÔNG
// dùng BEGIN/COMMIT — mỗi client.query() ở đây tự chạy ngoài transaction.
'use strict';

const { createClient } = require('./db');

async function main() {
  const client = createClient();
  await client.connect();

  await client.query('DROP INDEX CONCURRENTLY IF EXISTS idx_lock_test_orders_email');

  console.log('CREATE INDEX CONCURRENTLY...');
  const startedAt = Date.now();

  try {
    await client.query(
      'CREATE INDEX CONCURRENTLY idx_lock_test_orders_email ON lock_test_orders(email)'
    );
    console.log(`Xong sau ${((Date.now() - startedAt) / 1000).toFixed(1)}s (không block write).`);
  } catch (err) {
    console.error(`Thất bại sau ${((Date.now() - startedAt) / 1000).toFixed(1)}s:`, err.message);
    console.error('Nếu fail giữa chừng, index có thể ở trạng thái INVALID, cần DROP rồi tạo lại.');
  } finally {
    await client.end();
  }
}

main().catch((err) => {
  console.error('Lỗi:', err);
  process.exit(1);
});
