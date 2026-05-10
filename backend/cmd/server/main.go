package main

import (
	"log"
	"net/http"

	"flutter-admin-go/internal/server"
	"flutter-admin-go/internal/store"
)

func main() {
	if err := store.Init(""); err != nil {
		log.Fatal(err)
	}

	handler := server.NewRouter()
	addr := ":8080"
	log.Printf("server started at http://localhost%s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
