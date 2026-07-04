// Mô phỏng 1 "session A" giữ transaction mở lâu trên production, giống như
// một request chậm, một job quên COMMIT, hoặc một transaction chờ I/O.
// Chạy script này trước, RỒI mở terminal khác chạy create-index.js /
// add-column-not-null-default.js để xem chúng bị chặn (block) ra sao.
'use strict';

const { createClient } = require('./db');

const HOLD_SECONDS = Number(process.env.HOLD_SECONDS || 30);

async function main() {
  const client = createClient();
  await client.connect();

  console.log('BEGIN transaction...');
  await client.query('BEGIN');

  console.log('UPDATE 1 row (giữ ROW EXCLUSIVE lock trên table + row lock)...');
  await client.query(
    `UPDATE lock_test_orders SET status = 'paid' WHERE id = (SELECT id FROM lock_test_orders LIMIT 1)`
  );

  console.log(`Giữ transaction mở trong ${HOLD_SECONDS}s. Mở terminal khác và thử:`);
  console.log('  npm run lock:create-index');
  console.log('  npm run lock:add-column');
  console.log('để xem chúng phải xếp hàng chờ lock...\n');

  await new Promise((resolve) => setTimeout(resolve, HOLD_SECONDS * 1000));

  console.log('COMMIT. Giải phóng lock.');
  await client.query('COMMIT');
  await client.end();
}

main().catch((err) => {
  console.error('Lỗi:', err);
  process.exit(1);
});
