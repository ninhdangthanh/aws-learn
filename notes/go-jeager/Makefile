.DEFAULT_GOAL := help
.PHONY: help up down restart reset logs ps build proto tidy vet check happy oos slow panic async load all psql rabbit jaeger

help: ## Liệt kê các lệnh
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

up: ## Build và khởi động toàn bộ lab
	docker compose up -d --build
	@echo ""
	@echo "  Jaeger UI    → http://localhost:16686"
	@echo "  RabbitMQ UI  → http://localhost:15673  (lab / lab)"
	@echo "  Gateway      → http://localhost:8080"
	@echo ""
	@echo "  Kiểm tra:  make check"

down: ## Dừng lab (giữ dữ liệu)
	docker compose down

reset: ## Dừng lab và xoá sạch dữ liệu Postgres
	docker compose down -v

restart: ## Build lại và khởi động lại
	docker compose up -d --build

logs: ## Xem log tất cả service (Ctrl-C để thoát)
	docker compose logs -f gateway order inventory notification

ps: ## Trạng thái container
	docker compose ps

build: ## Build Go binary tại máy (không qua Docker)
	go build ./...

proto: ## Sinh lại code gRPC từ file .proto
	buf generate

tidy: ## Dọn go.mod
	go mod tidy

vet: ## Chạy go vet
	go vet ./...

check: ## Kịch bản 0 — kiểm tra hệ thống sẵn sàng
	@./scripts/00-check.sh

happy: ## Kịch bản 1 — luồng thành công
	@./scripts/01-happy.sh

oos: ## Kịch bản 2 — hết hàng (lỗi nghiệp vụ)
	@./scripts/02-out-of-stock.sh

slow: ## Kịch bản 3 — truy vấn DB chậm
	@./scripts/03-slow-db.sh

panic: ## Kịch bản 4 — service panic và chết
	@./scripts/04-panic.sh

async: ## Kịch bản 5 — lỗi ở nhánh async
	@./scripts/05-async-fail.sh

load: ## Kịch bản 6 — lưu lượng hỗn hợp (make load N=100)
	@./scripts/06-load.sh $(or $(N),40)

all: ## Chạy tuần tự cả 6 kịch bản
	@./scripts/run-all.sh

psql: ## Mở psql vào database lab
	docker exec -it lab-postgres psql -U lab -d lab

rabbit: ## Mở RabbitMQ UI
	open http://localhost:15673 || xdg-open http://localhost:15673

jaeger: ## Mở Jaeger UI
	open http://localhost:16686 || xdg-open http://localhost:16686
