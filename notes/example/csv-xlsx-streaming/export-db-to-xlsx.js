// export-db-to-xlsx.js — Export ra XLSX bằng streaming writer của exceljs.
//
// ============================================================================
// LƯU Ý QUAN TRỌNG VỀ XLSX
// ============================================================================
//   1. XLSX là file zip chứa nhiều XML. KHÔNG dùng workbook thường
//      (new Excel.Workbook) cho dữ liệu lớn vì nó dựng toàn bộ cây object trong
//      RAM rồi mới ghi -> nổ heap. Phải dùng WorkbookWriter (streaming) như dưới.
//   2. Mỗi sheet Excel GIỚI HẠN 1.048.576 dòng. Nếu bảng của bạn nhiều hơn số
//      này thì XLSX KHÔNG chứa nổi trong 1 sheet -> phải tách sheet, hoặc đơn
//      giản là dùng CSV (không giới hạn dòng). Với "1GB dữ liệu" thì CSV gần như
//      luôn là lựa chọn đúng; XLSX chỉ hợp khi số dòng vừa phải + cần format.
//
//   Ở đây ta demo streaming writer và tự tách sheet khi chạm ngưỡng dòng.
// ============================================================================
require('dotenv').config();
const fs = require('fs');
const path = require('path');
const Excel = require('exceljs');
const QueryStream = require('pg-query-stream');
const { pool } = require('./db');

const EXPORT_FILE = process.env.EXPORT_XLSX_FILE || './data/export.xlsx';
const MAX_ROWS_PER_SHEET = 1_000_000; // < giới hạn 1.048.576 cho an toàn
const COLUMNS = [
  { header: 'uuid', key: 'uuid' },
  { header: 'first_name', key: 'first_name' },
  { header: 'last_name', key: 'last_name' },
  { header: 'email', key: 'email' },
  { header: 'age', key: 'age' },
  { header: 'country', key: 'country' },
  { header: 'balance', key: 'balance' },
  { header: 'created_at', key: 'created_at' },
];

async function main() {
  fs.mkdirSync(path.dirname(EXPORT_FILE), { recursive: true });
  const client = await pool.connect();
  const startedAt = Date.now();
  let rows = 0;
  let sheetIndex = 1;

  try {
    // WorkbookWriter ghi thẳng ra file theo dòng, không giữ cả workbook trong RAM.
    const workbook = new Excel.stream.xlsx.WorkbookWriter({ filename: EXPORT_FILE });
    let sheet = workbook.addWorksheet(`users_${sheetIndex}`);
    sheet.columns = COLUMNS;

    const query = new QueryStream(
      'SELECT uuid, first_name, last_name, email, age, country, balance, created_at FROM users',
      [],
      { batchSize: 10_000 }
    );
    const dbStream = client.query(query);

    for await (const record of dbStream) {
      // Chạm ngưỡng -> commit sheet hiện tại và mở sheet mới.
      if (rows > 0 && rows % MAX_ROWS_PER_SHEET === 0) {
        sheet.commit();
        sheetIndex++;
        sheet = workbook.addWorksheet(`users_${sheetIndex}`);
        sheet.columns = COLUMNS;
      }

      // .commit() trên mỗi row = flush dòng đó ra đĩa ngay -> RAM phẳng.
      sheet.addRow(record).commit();
      rows++;
      if (rows % 500_000 === 0) console.log(`  ...đã ghi ${rows.toLocaleString()} dòng`);
    }

    sheet.commit();
    await workbook.commit(); // đóng file zip

    const secs = ((Date.now() - startedAt) / 1000).toFixed(1);
    console.log(`✅ Export ${rows.toLocaleString()} dòng ra ${EXPORT_FILE} (${sheetIndex} sheet) trong ${secs}s`);
  } finally {
    client.release();
    await pool.end();
  }
}

main().catch((err) => {
  console.error('❌ Lỗi export XLSX:', err);
  process.exit(1);
});
