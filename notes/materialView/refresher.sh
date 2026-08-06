#!/bin/sh
# Vòng lặp refresh materialized view mỗi REFRESH_INTERVAL giây.
# Cố tình viết bằng psql thuần để thấy rõ: refresh là một câu lệnh SQL,
# ai gọi nó (cron, worker, k8s CronJob, pg_cron) chỉ là chi tiết triển khai.
set -e

INTERVAL="${REFRESH_INTERVAL:-30}"
echo "[refresher] start, interval=${INTERVAL}s"

# Lúc container postgres init lần đầu nó restart một nhịp, healthcheck có thể
# pass sớm trên server tạm -> chờ cho tới khi hàm refresh thật sự gọi được.
until psql -qtAX -c "SELECT 1 FROM pg_proc WHERE proname = 'refresh_all_matviews'" 2>/dev/null | grep -q 1; do
  echo "[refresher] waiting for database to finish init..."
  sleep 3
done

while true; do
  START=$(date -u +"%H:%M:%S")
  OUT=$(psql -v ON_ERROR_STOP=1 -qtAX -c "SELECT refresh_all_matviews();" 2>&1) || {
    echo "[refresher] ${START} ERROR: ${OUT}"
    sleep "${INTERVAL}"
    continue
  }
  echo "[refresher] ${START} refreshed -> ${OUT}"
  sleep "${INTERVAL}"
done
