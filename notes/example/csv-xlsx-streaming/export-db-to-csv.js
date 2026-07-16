// export-db-to-csv.js — Đọc STREAMING từ Postgres rồi ghi ra file CSV.
//
// ============================================================================
// VẤN ĐỀ
// ============================================================================
// `SELECT * FROM users` trên bảng vài triệu dòng: driver pg mặc định sẽ nạp
// TOÀN BỘ kết quả vào RAM trước khi trả về -> nổ heap khi export cỡ 1GB.
//
// GIẢI PHÁP: dùng CURSOR phía server qua pg-query-stream. Nó chỉ kéo từng chunk
// dòng về client theo nhịp ta tiêu thụ. Kết hợp với csv-stringify (transform
// object -> dòng CSV) và write stream ra file, ta có 1 pipeline chảy phẳng:
//
//   Postgres cursor -> QueryStream -> csv-stringify -> fs.WriteStream (file)
//
// pipeline() tự lo backpressure giữa các mắt xích: file ghi chậm thì cursor kéo
// dữ liệu chậm lại theo. RAM luôn ổn định bất kể bảng to cỡ nào.
// ============================================================================
require('dotenv').config();
const fs = require('fs');
const path = require('path');
const { pipeline } = require('stream/promises');
const { stringify } = require('csv-stringify');
const QueryStream = require('pg-query-stream');
const { pool } = require('./db');

const EXPORT_FILE = process.env.EXPORT_CSV_FILE || './data/export.csv';

async function main() {
  fs.mkdirSync(path.dirname(EXPORT_FILE), { recursive: true });
  const client = await pool.connect();
  const startedAt = Date.now();
  let rows = 0;

  try {
    // batchSize: số dòng cursor kéo về mỗi lượt. Lớn hơn -> ít round-trip, tốn RAM hơn.
    const query = new QueryStream(
      'SELECT uuid, first_name, last_name, email, age, country, balance, created_at FROM users',
      [],
      { batchSize: 10_000 }
    );
    const dbStream = client.query(query);

    // Đếm dòng cho vui + log tiến độ (không giữ dữ liệu lại).
    dbStream.on('data', () => {
      rows++;
      if (rows % 500_000 === 0) console.log(`  ...đã export ${rows.toLocaleString()} dòng`);
    });

    const csvStream = stringify({
      header: true,
      columns: ['uuid', 'first_name', 'last_name', 'email', 'age', 'country', 'balance', 'created_at'],
    });

    await pipeline(
      dbStream,
      csvStream,
      fs.createWriteStream(EXPORT_FILE, { highWaterMark: 1 << 20 })
    );

    const secs = ((Date.now() - startedAt) / 1000).toFixed(1);
    console.log(`✅ Export ${rows.toLocaleString()} dòng ra ${EXPORT_FILE} trong ${secs}s`);
  } finally {
    client.release();
    await pool.end();
  }
}

main().catch((err) => {
  console.error('❌ Lỗi export CSV:', err);
  process.exit(1);
});
