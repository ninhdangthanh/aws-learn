.PHONY: help dev dev-be dev-fe build up down logs clean install-be install-fe

# ── defaults ──────────────────────────────────────────────────────────────────
BE_DIR  := ./backend
FE_DIR  := ./frontend
PORT_BE := 8080
PORT_FE := 5173   # Vite default dev port

# ── help ──────────────────────────────────────────────────────────────────────
help:
	@echo ""
	@echo "  Video Call – available targets"
	@echo "  ─────────────────────────────────────────────────────"
	@echo "  make dev          Run backend + frontend locally (parallel)"
	@echo "  make dev-be       Run backend only"
	@echo "  make dev-fe       Run frontend only"
	@echo "  make install      Install all dependencies"
	@echo "  make install-be   Download Go modules"
	@echo "  make install-fe   npm install for frontend"
	@echo "  make build        Build both via Docker Compose"
	@echo "  make up           docker compose up (detached)"
	@echo "  make down         docker compose down"
	@echo "  make logs         Follow docker compose logs"
	@echo "  make clean        Remove build artifacts & docker volumes"
	@echo ""
	@echo "  Note: To test on mobile/other devices on the same WiFi,"
	@echo "        use http://<your-local-ip>:5173"
	@echo ""

# ── install ───────────────────────────────────────────────────────────────────
install: install-be install-fe

install-be:
	@echo "📦 Downloading Go modules..."
	cd $(BE_DIR) && go mod tidy && go mod download

install-fe:
	@echo "📦 Installing frontend npm packages..."
	cd $(FE_DIR) && npm install

# ── local dev (parallel) ──────────────────────────────────────────────────────
dev:
	@echo "🚀 Starting backend + frontend..."
	$(MAKE) -j2 dev-be dev-fe

dev-be:
	@echo "🔧 Backend  → http://localhost:$(PORT_BE)"
	cd $(BE_DIR) && go run ./main.go

dev-fe:
	@echo "🎨 Frontend → http://localhost:$(PORT_FE)"
	cd $(FE_DIR) && npm run dev

# ── docker ────────────────────────────────────────────────────────────────────
build:
	@echo "🐳 Building Docker images..."
	docker compose build

up:
	@echo "🐳 Starting services (detached)..."
	docker compose up -d
	@echo "✅ Backend  → http://localhost:$(PORT_BE)"
	@echo "✅ Frontend → http://localhost:3000"

down:
	@echo "🛑 Stopping services..."
	docker compose down

logs:
	docker compose logs -f

clean:
	@echo "🧹 Cleaning..."
	docker compose down -v --remove-orphans
	rm -f $(BE_DIR)/server $(BE_DIR)/videocall.db
	rm -rf $(FE_DIR)/dist $(FE_DIR)/node_modules

