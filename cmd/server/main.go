package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/safullin/pro_go_1/internal/config"
	"github.com/safullin/pro_go_1/internal/repository"
	"github.com/safullin/pro_go_1/internal/server"
)

func main() {
	cfg, err := config.ParseServerConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	storage := repository.NewMemStorage()
	if err := http.ListenAndServe(cfg.Address, server.NewServer(storage)); err != nil {
		log.Fatal(err)
	}
}
