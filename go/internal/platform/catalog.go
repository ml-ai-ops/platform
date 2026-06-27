package platform

import "github.com/mlaiops/platform/pkg/api"

func Components() []api.Component {
	return []api.Component{
		{Name: "API Gateway", Category: "Control plane", Status: "healthy", Description: "Go REST gateway and unified platform contract"},
		{Name: "Pipeline Engine", Category: "Orchestration", Status: "not_configured", Description: "Connect Kubeflow Pipelines and Argo Workflows"},
		{Name: "Experiment Tracker", Category: "ML lifecycle", Status: "not_configured", Description: "Connect MLflow with PostgreSQL and S3 artifacts"},
		{Name: "Feature Store", Category: "Data", Status: "not_configured", Description: "Connect Feast, Redis, and an offline store"},
		{Name: "Object Store", Category: "Storage", Status: "not_configured", Description: "Connect MinIO or an external S3-compatible store"},
		{Name: "Inference Engine", Category: "Serving", Status: "not_configured", Description: "Connect KServe and Knative"},
		{Name: "Agent Observability", Category: "Agentic AI", Status: "not_configured", Description: "Connect Langfuse for traces, prompts, and evaluations"},
		{Name: "Streaming Broker", Category: "Events", Status: "not_configured", Description: "Connect Apache Kafka via Strimzi"},
	}
}

func Catalog() []api.CatalogItem {
	return []api.CatalogItem{
		{Name: "fraud-risk-v3", Version: "3", Status: "production", Kind: "model", Metadata: []string{"AUC 0.94", "p99 32ms"}},
		{Name: "customer_features", Version: "12", Status: "materialized", Kind: "feature", Metadata: []string{"24 fields", "TTL 1h"}},
		{Name: "research-assistant", Version: "1.4", Status: "ready", Kind: "agent", Metadata: []string{"LangGraph", "2 tools"}},
		{Name: "feature_store_lookup", Version: "1.2", Status: "ready", Kind: "tool", Metadata: []string{"data", "feast"}},
	}
}
