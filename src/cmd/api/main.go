package main

import (
	"log"
	"net/http"

	"github.com/xplane/xplane/internal/api"
	"github.com/xplane/xplane/internal/db"
)

func main() {
	d, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(d); err != nil {
		log.Fatal(err)
	}

	s := &api.Server{DB: d}

	mux := http.NewServeMux()
	mux.HandleFunc("/register", s.RegisterNode)
	mux.HandleFunc("/deregister", s.DeregisterNode)
	mux.HandleFunc("/heartbeat", s.Heartbeat)
	mux.HandleFunc("/cleanup-stale", s.CleanupStale)

	addr := ":8000"
	log.Println("API listening on", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
