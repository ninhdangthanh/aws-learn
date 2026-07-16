// import-csv-copy.js — Cách NHANH NHẤT để nạp file CSV lớn vào Postgres:
// dùng lệnh COPY ... FROM STDIN qua pg-copy-streams.
//
// ============================================================================
// COPY vs INSERT theo batch
// ============================================================================
//   - INSERT batch: linh hoạt (validate/transform từng dòng) nhưng vẫn qua lớp
//     parser SQL + planner cho mỗi lệnh.
//   - COPY: đường ống nhị phân/text tối ưu riêng cho nạp khối lượng lớn. Với file
//     "sạch", đúng thứ tự cột, COPY thường nhanh gấp nhiều lần.
//
// Toàn bộ đây là 1 PIPELINE stream thuần:
//   read file .csv  ->  COPY FROM STDIN  ->  Postgres
// Không có bước nào giữ cả file trong RAM, nên 1GB hay 100GB đều chạy phẳng.
//
// LƯU Ý: file phải khớp định dạng khai báo trong COPY (ở đây: có HEADER, phân
// tách bằng dấu phẩy, format CSV). Cột trong COPY phải đúng thứ tự header file.
// ============================================================================
require('dotenv').config();
const fs = require('fs');
const { pipeline } = require('stream/promises');
const copyFrom = require('pg-copy-streams').from;
const { pool } = require('./db');

const CSV_FILE = process.env.CSV_FILE || './data/users.csv';

async function main() {
  const client = await pool.connect();
  const startedAt = Date.now();
  try {
    const ingest = client.query(
      copyFrom(
        `COPY users (uuid, first_name, last_name, email, age, country, balance, created_at)
         FROM STDIN WITH (FORMAT csv, HEADER true)`
      )
    );

    // pipeline nối read stream -> COPY stream và tự xử lý backpressure + lỗi.
    await pipeline(fs.createReadStream(CSV_FILE), ingest);

    const secs = ((Date.now() - startedAt) / 1000).toFixed(1);
    console.log(`✅ COPY xong trong ${secs}s`);
  } finally {
    client.release();
    await pool.end();
  }
}

main().catch((err) => {
  console.error('❌ Lỗi COPY:', err);
  process.exit(1);
});
