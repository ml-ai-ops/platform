package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/mlaiops/platform/internal/httpapi"
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
	dataPath := os.Getenv("MLAIOPS_DATA_PATH")
	if dataPath == "" {
		dataPath = "data/platform.json"
	}
	server := &http.Server{Addr: ":" + port, Handler: httpapi.New(store.New(dataPath), static)}
	log.Printf("ml-ai-ops-platform is ready at http://localhost:%s", port)
	log.Fatal(server.ListenAndServe())
}
