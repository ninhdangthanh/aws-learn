package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func InitDB(host, port, user, password, dbname string) (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	log.Println("Connected to PostgreSQL successfully")

	if err := createTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			product_id VARCHAR(255) NOT NULL,
			quantity INT NOT NULL,
			price DECIMAL(10, 2) NOT NULL,
			status VARCHAR(50) DEFAULT 'created',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS events (
			event_id VARCHAR(255) PRIMARY KEY,
			event_type VARCHAR(100) NOT NULL,
			data TEXT NOT NULL,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			version INT DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS idempotency (
			key VARCHAR(255) PRIMARY KEY,
			value TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);`,
		`CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			log.Printf("Error creating table: %v", err)
			return err
		}
	}

	log.Println("Tables created/verified successfully")
	return nil
}

func CreateOrder(db *sql.DB, id, userID, productID string, quantity int, price float64) error {
	query := `INSERT INTO orders (id, user_id, product_id, quantity, price, status)
		VALUES ($1, $2, $3, $4, $5, 'created')
		ON CONFLICT (id) DO NOTHING`

	_, err := db.Exec(query, id, userID, productID, quantity, price)
	return err
}

func RecordEvent(db *sql.DB, eventID, eventType, data string) error {
	query := `INSERT INTO events (event_id, event_type, data)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id) DO NOTHING`

	_, err := db.Exec(query, eventID, eventType, data)
	return err
}

func GetIdempotencyValue(db *sql.DB, key string) (string, bool, error) {
	var value string
	query := `SELECT value FROM idempotency WHERE key = $1`
	err := db.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func SetIdempotencyValue(db *sql.DB, key, value string) error {
	query := `INSERT INTO idempotency (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO NOTHING`
	_, err := db.Exec(query, key, value)
	return err
}
