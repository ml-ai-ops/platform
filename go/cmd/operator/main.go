package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	platformoperator "github.com/mlaiops/platform/internal/operator"
)

func main() {
	port := env("PORT", "8082")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		write(w, http.StatusOK, map[string]string{"status": "ok", "component": "mlaiops-operator"})
	})
	mux.HandleFunc("POST /internal/v1/reconcile/agent", func(w http.ResponseWriter, r *http.Request) {
		var spec platformoperator.AgentSpec
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&spec); err != nil {
			write(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		plan, err := platformoperator.ReconcileAgent(spec)
		if err != nil {
			write(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		write(w, http.StatusOK, plan)
	})
	server := &http.Server{
		Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	log.Printf("mlaiops-operator reconciliation service listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
