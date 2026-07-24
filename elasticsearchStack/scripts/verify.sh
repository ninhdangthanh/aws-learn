#!/usr/bin/env bash
# Phase 1 — verify ES + Kibana đã sẵn sàng.
set -euo pipefail

ES="${ES_URL:-http://localhost:9200}"
KB="${KB_URL:-http://localhost:5601}"

echo "==> ES root ($ES)"
curl -sf "$ES" | grep -E '"(number|cluster_name)"' || { echo "ES chưa lên"; exit 1; }

echo "==> Cluster health"
health=$(curl -sf "$ES/_cluster/health")
echo "$health"
status=$(echo "$health" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')
case "$status" in
  green|yellow) echo "OK: cluster status = $status" ;;
  *) echo "FAIL: cluster status = '$status'"; exit 1 ;;
esac

echo "==> Kibana status ($KB)"
if curl -sf "$KB/api/status" >/dev/null 2>&1; then
  echo "OK: Kibana trả lời (mở http://localhost:5601 → Dev Tools)"
else
  echo "WARN: Kibana chưa sẵn sàng (khởi động chậm hơn ES, thử lại sau ~30s)"
fi

echo "==> Phase 1 verify DONE"
