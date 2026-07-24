---
name: es-troubleshoot
description: >
  Gỡ lỗi vận hành project elasticsearchStack — stack Docker không lên, cluster
  red/vm.max_map_count, port bị chiếm (9200/5601/5433/8090), backend fail migrate/không
  connect Postgres, reset sạch volume, chơi lại Phase 2-4 khi backend chiếm alias.
  Dùng khi hỏi "ES không lên", "docker lỗi", "port bận", "go run fail", "reset stack",
  "chạy lại phase". Chẩn đoán vận hành, không sửa code.
---

# Troubleshoot — vận hành ES stack

Stack local học (security tắt). Lệnh chạy từ `notes/elasticsearchStack/`.

## Kiểm tra nhanh trạng thái

```bash
docker compose ps                                  # container nào up/healthy
docker logs es-learn --tail 50                     # log ES
docker logs pg-learn --tail 30                     # log Postgres
./scripts/verify.sh                                # ES root + cluster health + Kibana
curl -s localhost:9200/_cluster/health?pretty      # status: green/yellow OK, red = hỏng
```

## ES không lên / bị OOM

- ES cần bộ nhớ; compose đã set `ES_JAVA_OPTS=-Xms512m -Xmx512m`. Nếu container chết ngay → tăng RAM cho Docker Desktop.
- Log báo `max virtual memory areas vm.max_map_count [65530] is too low` → trên host Linux chạy `sudo sysctl -w vm.max_map_count=262144` (Docker Desktop macOS thường không cần).
- Kibana lên chậm hơn ES (~30s) — `verify.sh` báo WARN là bình thường, thử lại sau.
- Cluster `yellow` là **OK** ở đây (`number_of_replicas: 0`, single-node) — không phải lỗi.

## Port bị chiếm

| Port | Service | Nếu bận |
|---|---|---|
| 9200 | Elasticsearch | có ES khác đang chạy — `docker ps`, tắt cái cũ |
| 5601 | Kibana | |
| 5433 | Postgres (đã map lệch để tránh 5432) | đổi mapping trong `docker-compose.yml` + `DATABASE_URL` |
| 8090 | backend Go | đổi `PORT` trong `backend/.env` (và `VITE_API_BASE` FE) |

```bash
lsof -i :9200   # xem process nào giữ port (macOS)
```

## Backend `go run .` lỗi

- `postgres: ...` / không connect → Postgres chưa up hoặc DSN sai. `docker compose up -d postgres`, check `DATABASE_URL` (mặc định `postgres://app:app@localhost:5433/shop?sslmode=disable`).
- `đọc migration` → phải chạy `go run .` **từ thư mục `backend/`** (đọc `migrations/001_init.sql` theo relative path).
- `[warn] ES chưa sẵn sàng` → **không fatal**, server vẫn chạy, worker retry khi ES lên.
- Worker không sync → kiểm tra `RUN_WORKER=true` và `WRITE_MODE=outbox` (mode `dual` cố tình tắt worker).

## Reset sạch

```bash
docker compose down                 # dừng, GIỮ data (volume es_data/pg_data)
docker compose down -v              # dừng + XOÁ volume → mất hết index & bảng, dựng lại từ đầu
docker compose up -d elasticsearch kibana postgres
```

## Chơi lại Phase 2-4 khi đã chạy backend

Backend "chiếm" tên `products` làm **alias** (xoá index practice nếu còn). Muốn seed index thường để luyện Query DSL/aggregation:

```bash
# đảm bảo backend KHÔNG chạy, rồi:
./scripts/seed-products.sh          # xoá + tạo lại index products + bulk 30 doc + refresh + count
```
Chạy backend trở lại sẽ tự `EnsureIndexAlias` (chuyển `products` thành alias). Hai chế độ này xung khắc — chọn 1.

Overview: skill `es-stack-map`. Lệch data (không phải hạ tầng): skill `es-data-verify`.
