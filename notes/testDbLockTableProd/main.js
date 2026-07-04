// Seed script: tạo bảng test và insert ~2 triệu row bằng faker để mô phỏng
// một bảng "production-size" cho các bài test lock ở thư mục lock-demos/.
'use strict';

const { Pool } = require('pg');
const { faker } = require('@faker-js/faker');

const TABLE_NAME = 'lock_test_orders';
const TOTAL_ROWS = Number(process.env.TOTAL_ROWS || 2_000_000);
const BATCH_SIZE = Number(process.env.BATCH_SIZE || 5_000);

const pool = new Pool({
  host: process.env.PGHOST || 'localhost',
  port: Number(process.env.PGPORT || 5432),
  user: process.env.PGUSER || 'app',
  password: process.env.PGPASSWORD || 'app',
  database: process.env.PGDATABASE || 'lock_lab',
});

const STATUSES = ['created', 'paid', 'shipped', 'cancelled'];

function buildBatch(size) {
  const customerName = new Array(size);
  const email = new Array(size);
  const status = new Array(size);
  const amount = new Array(size);
  const createdAt = new Array(size);

  for (let i = 0; i < size; i++) {
    customerName[i] = faker.person.fullName();
    email[i] = faker.internet.email();
    status[i] = faker.helpers.arrayElement(STATUSES);
    amount[i] = faker.commerce.price({ min: 5, max: 2000 });
    createdAt[i] = faker.date.between({ from: '2023-01-01', to: new Date() });
  }

  return { customerName, email, status, amount, createdAt };
}

async function createTable(client) {
  await client.query(`DROP TABLE IF EXISTS ${TABLE_NAME}`);
  await client.query(`
    CREATE TABLE ${TABLE_NAME} (
      id BIGSERIAL PRIMARY KEY,
      customer_name TEXT NOT NULL,
      email TEXT NOT NULL,
      status TEXT NOT NULL,
      amount NUMERIC(10, 2) NOT NULL,
      created_at TIMESTAMPTZ NOT NULL
    )
  `);
}

async function insertBatch(client, size) {
  const { customerName, email, status, amount, createdAt } = buildBatch(size);
  await client.query(
    `
    INSERT INTO ${TABLE_NAME} (customer_name, email, status, amount, created_at)
    SELECT * FROM unnest(
      $1::text[], $2::text[], $3::text[], $4::numeric[], $5::timestamptz[]
    )
    `,
    [customerName, email, status, amount, createdAt]
  );
}

async function main() {
  const client = await pool.connect();
  const startedAt = Date.now();

  try {
    console.log(`Tạo bảng "${TABLE_NAME}"...`);
    await createTable(client);

    console.log(`Bắt đầu insert ${TOTAL_ROWS.toLocaleString()} rows, batch size ${BATCH_SIZE.toLocaleString()}...`);

    let inserted = 0;
    while (inserted < TOTAL_ROWS) {
      const size = Math.min(BATCH_SIZE, TOTAL_ROWS - inserted);
      await insertBatch(client, size);
      inserted += size;

      const elapsedSec = ((Date.now() - startedAt) / 1000).toFixed(1);
      console.log(`  ${inserted.toLocaleString()}/${TOTAL_ROWS.toLocaleString()} rows (${elapsedSec}s)`);
    }

    console.log('Đang ANALYZE bảng để planner có số liệu thống kê mới...');
    await client.query(`ANALYZE ${TABLE_NAME}`);

    const totalSec = ((Date.now() - startedAt) / 1000).toFixed(1);
    console.log(`Xong. Insert ${TOTAL_ROWS.toLocaleString()} rows trong ${totalSec}s.`);
  } finally {
    client.release();
    await pool.end();
  }
}

main().catch((err) => {
  console.error('Seed thất bại:', err);
  process.exit(1);
});
