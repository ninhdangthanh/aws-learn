// generate-fake-csv.js — Tạo file CSV giả ~1GB bằng @faker-js/faker.
//
// ============================================================================
// TẠI SAO PHẢI STREAMING?
// ============================================================================
// Nếu build cả file 1GB thành 1 chuỗi/1 mảng trong RAM rồi mới ghi -> nổ heap
// (Node mặc định giới hạn ~1.5-2GB, và 1GB text -> vài GB khi ở dạng JS object).
// Giải pháp: GHI TỪNG DÒNG ra một write stream, và TÔN TRỌNG BACKPRESSURE.
//
// BACKPRESSURE LÀ GÌ?
//   stream.write(chunk) trả về `false` khi buffer nội bộ đã đầy (ghi ra đĩa
//   chậm hơn tốc độ ta tạo dữ liệu). Lúc đó phải NGỪNG tạo dữ liệu và chờ sự
//   kiện 'drain' rồi mới ghi tiếp. Nếu phớt lờ giá trị trả về và cứ write() liên
//   tục -> buffer phình vô hạn -> nổ RAM y như cách làm sai ở trên.
// ============================================================================
require('dotenv').config();
const fs = require('fs');
const path = require('path');
const { faker } = require('@faker-js/faker');

const CSV_FILE = process.env.CSV_FILE || './data/users.csv';
const TARGET_BYTES = Number(process.env.FAKE_TARGET_GB || 1) * 1024 * 1024 * 1024;

const HEADER = 'uuid,first_name,last_name,email,age,country,balance,created_at\n';

// Escape CSV tối giản: nếu field chứa dấu phẩy/nháy kép/xuống dòng thì bọc nháy kép.
function csv(field) {
  const s = String(field);
  if (s.includes(',') || s.includes('"') || s.includes('\n')) {
    return '"' + s.replace(/"/g, '""') + '"';
  }
  return s;
}

function buildRow() {
  const first = faker.person.firstName();
  const last = faker.person.lastName();
  return [
    faker.string.uuid(),
    csv(first),
    csv(last),
    csv(faker.internet.email({ firstName: first, lastName: last })),
    faker.number.int({ min: 18, max: 90 }),
    csv(faker.location.country()),
    faker.finance.amount({ min: 0, max: 1_000_000, dec: 2 }),
    faker.date.past({ years: 5 }).toISOString(),
  ].join(',') + '\n';
}

async function main() {
  fs.mkdirSync(path.dirname(CSV_FILE), { recursive: true });

  // highWaterMark = ngưỡng buffer nội bộ trước khi write() trả về false.
  // Đặt 1MB để giảm số lần chờ 'drain' -> ghi mượt hơn.
  const out = fs.createWriteStream(CSV_FILE, { highWaterMark: 1 << 20 });

  out.write(HEADER);
  let bytes = Buffer.byteLength(HEADER);
  let rows = 0;
  const startedAt = Date.now();

  // Vòng lặp ghi. Trả về Promise, resolve khi đã đạt TARGET_BYTES.
  await new Promise((resolve, reject) => {
    out.on('error', reject);

    function writeUntilBackpressure() {
      let ok = true;
      while (ok && bytes < TARGET_BYTES) {
        const row = buildRow();
        bytes += Buffer.byteLength(row);
        rows++;

        // Log tiến độ mỗi 500k dòng cho đỡ sốt ruột.
        if (rows % 500_000 === 0) {
          const gb = (bytes / 1024 / 1024 / 1024).toFixed(3);
          console.log(`  ...${rows.toLocaleString()} dòng, ${gb} GB`);
        }

        // ok=false nghĩa là buffer đầy -> dừng vòng while, chờ 'drain'.
        ok = out.write(row);
      }

      if (bytes >= TARGET_BYTES) {
        out.end(resolve); // ghi nốt buffer rồi đóng file
      } else {
        // Buffer đầy: chờ nhả bớt ('drain') rồi ghi tiếp. Đây chính là
        // chỗ backpressure phát huy tác dụng để RAM luôn phẳng.
        out.once('drain', writeUntilBackpressure);
      }
    }

    writeUntilBackpressure();
  });

  const secs = ((Date.now() - startedAt) / 1000).toFixed(1);
  const gb = (bytes / 1024 / 1024 / 1024).toFixed(3);
  console.log(`✅ Đã tạo ${CSV_FILE}: ${rows.toLocaleString()} dòng, ~${gb} GB trong ${secs}s`);
}

main().catch((err) => {
  console.error('❌ Lỗi khi tạo file:', err);
  process.exit(1);
});
