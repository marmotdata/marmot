package utils

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"net/http"
	"time"
)

func WaitForPostgres(config TestConfig) error {
	connStr := fmt.Sprintf("host=localhost port=%s user=postgres password=%s dbname=test sslmode=disable",
		config.PostgresPort,
		config.PostgresPassword)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer db.Close()

	for i := 0; i < 60; i++ {
		if err := db.Ping(); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout waiting for postgres")
}

func WaitForApplication(port string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("http://localhost:%s", port)

	for i := 0; i < 60; i++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode != 0 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout waiting for application")
}
