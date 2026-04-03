package main

import (
	"log"
	"net/http"

	"github.com/safullin/pro_go_1/internal/repository"
	"github.com/safullin/pro_go_1/internal/server"
)

func main() {
	storage := repository.NewMemStorage()
	if err := http.ListenAndServe("localhost:8080", server.NewServer(storage)); err != nil {
		log.Fatal(err)
	}
}
