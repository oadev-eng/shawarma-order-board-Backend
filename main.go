package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	dataPath := os.Getenv("DATA_FILE")
	if dataPath == "" {
		dataPath = "orders.json"
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		if port := os.Getenv("PORT"); port != "" {
			addr = ":" + port // Railway (and most PaaS hosts) assign this automatically
		} else {
			addr = ":8080"
		}
	}

	store := NewStore(dataPath)
	api := NewAPI(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/menu", api.handleMenu)
	mux.HandleFunc("POST /api/orders", api.handleCreateOrder)
	mux.HandleFunc("GET /api/orders", api.handleListOrders)
	mux.HandleFunc("PUT /api/orders/{id}", api.handleUpdateOrder)
	mux.HandleFunc("POST /api/orders/{id}/complete", api.handleCompleteOrder)
	mux.HandleFunc("POST /api/orders/{id}/reopen", api.handleReopenOrder)
	mux.HandleFunc("DELETE /api/orders/{id}", api.handleDeleteOrder)
	mux.HandleFunc("POST /api/completed/clear", api.handleClearCompleted)
	mux.HandleFunc("GET /api/summary", api.handleSummary)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("shawarma order board backend listening on %s (data file: %s)", addr, dataPath)
	log.Fatal(http.ListenAndServe(addr, withCORS(mux)))
}
