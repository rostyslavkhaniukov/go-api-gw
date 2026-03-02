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
		port = "9092"
	}

	mux := http.NewServeMux()

	// GET /api/v1/products — list products
	// POST /api/v1/products — create product
	mux.HandleFunc("/api/v1/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, http.StatusOK, map[string]any{
				"service": "upstream-2",
				"data": []map[string]any{
					{"id": 1, "name": "Widget", "price": 9.99, "category": "tools"},
					{"id": 2, "name": "Gadget", "price": 24.99, "category": "electronics"},
					{"id": 3, "name": "Gizmo", "price": 14.50, "category": "electronics"},
				},
			})
		case http.MethodPost:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			body["id"] = 4
			jsonResponse(w, http.StatusCreated, map[string]any{
				"service": "upstream-2",
				"data":    body,
			})
		default:
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	// GET /api/v1/products/categories
	// GET /api/v1/products/{id}
	mux.HandleFunc("/api/v1/products/", func(w http.ResponseWriter, r *http.Request) {
		sub := strings.TrimPrefix(r.URL.Path, "/api/v1/products/")
		if sub == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "missing product id"})
			return
		}

		// /api/v1/products/categories
		if sub == "categories" {
			jsonResponse(w, http.StatusOK, map[string]any{
				"service": "upstream-2",
				"data":    []string{"tools", "electronics", "home", "garden"},
			})
			return
		}

		parts := strings.Split(sub, "/")
		id := parts[0]

		// /api/v1/products/{id}/reviews
		if len(parts) == 2 && parts[1] == "reviews" {
			jsonResponse(w, http.StatusOK, map[string]any{
				"service":    "upstream-2",
				"product_id": id,
				"data": []map[string]any{
					{"review_id": "rev-1", "rating": 5, "text": "Excellent product!"},
					{"review_id": "rev-2", "rating": 4, "text": "Good value for money"},
				},
			})
			return
		}

		// GET /api/v1/products/{id}
		if r.Method == http.MethodGet {
			jsonResponse(w, http.StatusOK, map[string]any{
				"service": "upstream-2",
				"data":    map[string]any{"id": id, "name": "Widget", "price": 9.99, "category": "tools"},
			})
		} else {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	// GET /healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]string{"status": "ok", "service": "upstream-2"})
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("upstream-2 listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
