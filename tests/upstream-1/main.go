package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9091"
	}

	mux := http.NewServeMux()

	// GET /api/v1/users — list users
	// POST /api/v1/users — create user
	mux.HandleFunc("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, http.StatusOK, map[string]any{
				"service": "upstream-1",
				"data": []map[string]any{
					{"id": 1, "name": "Alice", "email": "alice@example.com"},
					{"id": 2, "name": "Bob", "email": "bob@example.com"},
					{"id": 3, "name": "Charlie", "email": "charlie@example.com"},
				},
			})
		case http.MethodPost:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			body["id"] = 4
			jsonResponse(w, http.StatusCreated, map[string]any{
				"service": "upstream-1",
				"data":    body,
			})
		default:
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	// GET /api/v1/users/{id}
	// DELETE /api/v1/users/{id}
	mux.HandleFunc("/api/v1/users/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/users/"), "/")
		id := parts[0]
		if id == "" {
			// Fall through to the /api/v1/users handler behaviour for trailing slash.
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "missing user id"})
			return
		}

		// /api/v1/users/{id}/orders
		if len(parts) == 2 && parts[1] == "orders" {
			jsonResponse(w, http.StatusOK, map[string]any{
				"service": "upstream-1",
				"user_id": id,
				"data": []map[string]any{
					{"order_id": "ord-101", "total": 29.99, "status": "shipped"},
					{"order_id": "ord-102", "total": 49.50, "status": "pending"},
				},
			})
			return
		}

		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, http.StatusOK, map[string]any{
				"service": "upstream-1",
				"data":    map[string]any{"id": id, "name": "Alice", "email": "alice@example.com"},
			})
		case http.MethodDelete:
			jsonResponse(w, http.StatusOK, map[string]any{
				"service": "upstream-1",
				"deleted": id,
			})
		default:
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	// GET /healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]string{"status": "ok", "service": "upstream-1"})
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("upstream-1 listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
