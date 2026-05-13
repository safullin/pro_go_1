package main

import (
	"log"
	"net/http"

	"pro_go_1/internal/config"
	dbconfig "pro_go_1/internal/config/db"
	"pro_go_1/internal/handler"
	"pro_go_1/internal/repository/postgres"
)

func main() {
	cfg := config.New()

	db, err := dbconfig.Open(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.Handle("/ping", handler.NewPingHandler(postgres.NewPinger(db)))

	if err = http.ListenAndServe(cfg.Address, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
