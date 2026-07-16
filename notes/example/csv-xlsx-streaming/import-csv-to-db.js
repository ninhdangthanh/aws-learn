// import-csv-to-db.js — Đọc STREAMING file CSV ~1GB rồi ghi vào Postgres theo BATCH.
//
// ============================================================================
// Ý TƯỞNG
// ============================================================================
//   file .csv --(read stream)--> csv-parse --(từng object)--> gom thành batch
//     --> INSERT nhiều dòng 1 lệnh --> lặp lại tới hết file.
//
// Vì sao KHÔNG insert từng dòng một? 5 triệu dòng = 5 triệu round-trip tới DB
// -> chậm kinh khủng. Gom BATCH (vd 5000 dòng/lệnh INSERT) giảm round-trip
// hàng nghìn lần.
//
// Vì sao KHÔNG đọc hết file rồi mới insert? File 1GB -> nổ RAM. Ta đọc tới đâu,
// insert tới đó, và QUAN TRỌNG: phải PAUSE stream trong lúc đang insert để
// tránh csv-parse đẩy dữ liệu nhanh hơn tốc độ DB tiêu thụ (đây là backpressure
// làm thủ công vì ta có bước async xen giữa).
//
// 👉 Nếu file "sạch" và không cần transform/validate, hãy dùng import-csv-copy.js
//    (COPY FROM STDIN) — nhanh nhất. File này minh hoạ pattern có validate/transform.
// ============================================================================
require('dotenv').config();
const fs = require('fs');
const { parse } = require('csv-parse');
const { pool } = require('./db');

const CSV_FILE = process.env.CSV_FILE || './data/users.csv';
const BATCH_SIZE = Number(process.env.IMPORT_BATCH_SIZE || 5000);
const COLUMNS = ['uuid', 'first_name', 'last_name', 'email', 'age', 'country', 'balance', 'created_at'];

// Ghép 1 lệnh INSERT nhiều dòng với placeholder $1..$N.
// rows = [[uuid, first, ...], [uuid, first, ...], ...]
async function insertBatch(client, rows) {
  if (rows.length === 0) return;
  const values = [];
  const placeholders = rows.map((row, r) => {
    const base = r * COLUMNS.length;
    values.push(...row);
    const ph = COLUMNS.map((_, c) => `$${base + c + 1}`);
    return `(${ph.join(',')})`;
  });
  const sql = `INSERT INTO users (${COLUMNS.join(',')}) VALUES ${placeholders.join(',')}`;
  await client.query(sql, values);
}

async function main() {
  const client = await pool.connect();
  const startedAt = Date.now();
  let batch = [];
  let total = 0;

  const parser = fs.createReadStream(CSV_FILE).pipe(
    parse({
      columns: true,       // dòng đầu là header -> mỗi record là object {uuid, first_name, ...}
      skip_empty_lines: true,
      trim: true,
    })
  );

  try {
    for await (const record of parser) {
      // (Chỗ này có thể validate/transform tuỳ ý, ví dụ bỏ dòng thiếu email.)
      batch.push([
        record.uuid,
        record.first_name,
        record.last_name,
        record.email,
        Number(record.age),
        record.country,
        record.balance,
        record.created_at,
      ]);

      if (batch.length >= BATCH_SIZE) {
        // `for await` tự PAUSE parser khi thân vòng lặp đang await -> backpressure
        // được xử lý tự động: DB chậm thì việc đọc file cũng chậm theo. RAM phẳng.
        await insertBatch(client, batch);
        total += batch.length;
        batch = [];
        if (total % (BATCH_SIZE * 20) === 0) {
          console.log(`  ...đã insert ${total.toLocaleString()} dòng`);
        }
      }
    }

    // Insert nốt batch cuối còn dư.
    await insertBatch(client, batch);
    total += batch.length;

    const secs = ((Date.now() - startedAt) / 1000).toFixed(1);
    console.log(`✅ Import xong ${total.toLocaleString()} dòng trong ${secs}s`);
  } finally {
    client.release();
    await pool.end();
  }
}

main().catch((err) => {
  console.error('❌ Lỗi import:', err);
  process.exit(1);
});
