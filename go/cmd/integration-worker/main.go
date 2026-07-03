package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/ml-ai-ops/platform/internal/integrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	namespace := env("MLAIOPS_TARGET_NAMESPACE", "default")
	consumer := integrations.NewKafkaConsumer(env("KAFKA_REST_URL", "http://kafka-rest:8082"), "mlaiops-integration", env("HOSTNAME", "worker"))
	topics := []string{"mlaiops.pipeline.commands", "mlaiops.model.commands", "mlaiops.agent.commands", "mlaiops.tool.commands", "mlaiops.connection.commands"}
	if err := consumer.Connect(ctx, topics); err != nil {
		log.Fatal(err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = consumer.Close(closeCtx)
	}()
	dispatcher := integrations.NewDispatcher(client, namespace)
	for ctx.Err() == nil {
		records, err := consumer.Poll(ctx)
		if err != nil {
			log.Printf("Kafka poll failed: %v", err)
			time.Sleep(time.Second)
			continue
		}
		for _, record := range records {
			if err := dispatcher.Dispatch(ctx, record); err != nil {
				log.Printf("dispatch topic=%s failed: %v", record.Topic, err)
			}
		}
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
