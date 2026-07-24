package config

import "os"

// Config — runtime config từ env (có default để chạy ngay với docker-compose).
type Config struct {
	Port        string // HTTP port, ":8080"
	DatabaseURL string // Postgres DSN
	ESURL       string // Elasticsearch base URL
	ESAlias     string // alias app đọc/ghi (KHÔNG trỏ thẳng index thật)
	WriteMode   string // "outbox" (đáng tin) | "dual" (demo lỗi dual-write)
	CORSOrigin  string // origin cho FE
	WorkerPoll  string // chu kỳ poll outbox, ví dụ "1s"
	RunWorker   bool   // chạy outbox worker in-process
}

func Load() *Config {
	return &Config{
		Port:        getenv("PORT", ":8090"), // 8080 hay đụng nginx/dev khác
		DatabaseURL: getenv("DATABASE_URL", "postgres://app:app@localhost:5433/shop?sslmode=disable"),
		ESURL:       getenv("ES_URL", "http://localhost:9200"),
		ESAlias:     getenv("ES_ALIAS", "products"),
		WriteMode:   getenv("WRITE_MODE", "outbox"),
		CORSOrigin:  getenv("CORS_ORIGIN", "http://localhost:5173"),
		WorkerPoll:  getenv("WORKER_POLL", "1s"),
		RunWorker:   getenv("RUN_WORKER", "true") != "false",
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
