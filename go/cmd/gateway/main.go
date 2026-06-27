package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mlaiops/platform/internal/httpapi"
	"github.com/mlaiops/platform/internal/integrations"
	"github.com/mlaiops/platform/internal/store"
)

//go:embed web/*
var web embed.FS

func main() {
	static, err := fs.Sub(web, "web")
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var repository store.Repository
	var postgres *store.Postgres
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		postgres, err = store.OpenPostgres(ctx, databaseURL, os.Getenv("MLAIOPS_TENANT"))
		if err != nil {
			log.Fatalf("open PostgreSQL repository: %v", err)
		}
		defer postgres.Close()
		repository = postgres
		if kafkaURL := os.Getenv("KAFKA_REST_URL"); kafkaURL != "" {
			worker := store.NewOutboxWorker(postgres, integrations.NewKafkaREST(kafkaURL, os.Getenv("KAFKA_REST_TOKEN")), time.Second)
			go worker.Run(ctx)
		}
		log.Printf("using PostgreSQL control-plane repository")
	} else {
		dataPath := os.Getenv("MLAIOPS_DATA_PATH")
		if dataPath == "" {
			dataPath = "data/platform.json"
		}
		repository = store.New(dataPath)
		log.Printf("using local file repository at %s", dataPath)
	}
	server := &http.Server{Addr: ":" + port, Handler: httpapi.New(repository, static), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("ml-ai-ops-platform is ready at http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
