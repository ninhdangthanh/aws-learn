// db.js — nơi tạo connection pool dùng chung + khởi tạo schema.
//
// pg.Pool: tái sử dụng connection thay vì mở/đóng liên tục. Với công việc
// streaming ta thường chỉ cần 1-2 connection nhưng pool giúp code gọn và an toàn.
require('dotenv').config();
const fs = require('fs');
const path = require('path');
const { Pool } = require('pg');

const pool = new Pool({
  connectionString: process.env.DATABASE_URL,
  // max thấp vì các script này chạy tuần tự, không cần nhiều connection.
  max: 4,
});

// Khởi tạo bảng từ schema.sql. Chạy: `npm run init-db`.
async function initSchema() {
  const sql = fs.readFileSync(path.join(__dirname, 'schema.sql'), 'utf8');
  await pool.query(sql);
  console.log('✅ Schema đã sẵn sàng (bảng users).');
}

module.exports = { pool, initSchema };

// Cho phép chạy trực tiếp `node db.js` để tạo bảng.
if (require.main === module) {
  initSchema()
    .then(() => pool.end())
    .catch((err) => {
      console.error('❌ Lỗi init schema:', err);
      process.exit(1);
    });
}
