package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) JSON(ctx context.Context, method, path string, input, output any) error {
	if c.BaseURL == "" {
		return errors.New("integration base URL is not configured")
	}
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("integration returned %d: %s", response.StatusCode, strings.TrimSpace(string(raw)))
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(output)
}

type KFP struct{ client *Client }

func NewKFP(baseURL, token string) KFP { return KFP{client: New(baseURL, token)} }
func (k KFP) Submit(ctx context.Context, experimentID, pipelineVersionID, name string, parameters map[string]string) (map[string]any, error) {
	inputs := make([]map[string]string, 0, len(parameters))
	for key, value := range parameters {
		inputs = append(inputs, map[string]string{"name": key, "value": value})
	}
	payload := map[string]any{"display_name": name, "experiment_id": experimentID, "pipeline_version_reference": map[string]string{"pipeline_version_id": pipelineVersionID}, "runtime_config": map[string]any{"parameters": inputs}}
	var result map[string]any
	err := k.client.JSON(ctx, http.MethodPost, "/apis/v2beta1/runs", payload, &result)
	return result, err
}

type MLflow struct{ client *Client }

func NewMLflow(baseURL, token string) MLflow { return MLflow{client: New(baseURL, token)} }
func (m MLflow) TransitionStage(ctx context.Context, name, version, stage string) error {
	return m.client.JSON(ctx, http.MethodPost, "/api/2.0/mlflow/model-versions/transition-stage", map[string]any{"name": name, "version": version, "stage": stage, "archive_existing_versions": stage == "Production"}, &map[string]any{})
}

type Langfuse struct{ client *Client }

func NewLangfuse(baseURL, token string) Langfuse { return Langfuse{client: New(baseURL, token)} }
func (l Langfuse) Ingest(ctx context.Context, batch []map[string]any) error {
	return l.client.JSON(ctx, http.MethodPost, "/api/public/ingestion", map[string]any{"batch": batch}, &map[string]any{})
}

type KafkaREST struct{ client *Client }

func NewKafkaREST(baseURL, token string) KafkaREST { return KafkaREST{client: New(baseURL, token)} }
func (k KafkaREST) Publish(ctx context.Context, topic string, event any) error {
	if topic == "" {
		return errors.New("topic is required")
	}
	return k.client.JSON(ctx, http.MethodPost, "/topics/"+url.PathEscape(topic), map[string]any{"records": []map[string]any{{"value": event}}}, &map[string]any{})
}
