package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type KafkaRecord struct {
	Topic string          `json:"topic"`
	Key   json.RawMessage `json:"key"`
	Value json.RawMessage `json:"value"`
}

type KafkaConsumer struct {
	restURL  string
	baseURI  string
	group    string
	instance string
	client   *http.Client
}

func NewKafkaConsumer(restURL, group, instance string) *KafkaConsumer {
	return &KafkaConsumer{restURL: strings.TrimRight(restURL, "/"), group: group, instance: instance, client: &http.Client{Timeout: 35 * time.Second}}
}

func (c *KafkaConsumer) Connect(ctx context.Context, topics []string) error {
	payload := map[string]any{"name": c.instance, "format": "json", "auto.offset.reset": "earliest", "auto.commit.enable": "true"}
	var response struct {
		BaseURI string `json:"base_uri"`
	}
	if err := c.request(ctx, http.MethodPost, c.restURL+"/consumers/"+c.group, payload, &response); err != nil {
		return err
	}
	c.baseURI = response.BaseURI
	if c.baseURI == "" {
		return fmt.Errorf("Kafka REST consumer did not return base_uri")
	}
	return c.request(ctx, http.MethodPost, c.baseURI+"/subscription", map[string]any{"topics": topics}, nil)
}

func (c *KafkaConsumer) Poll(ctx context.Context) ([]KafkaRecord, error) {
	if c.baseURI == "" {
		return nil, fmt.Errorf("consumer is not connected")
	}
	var records []KafkaRecord
	err := c.request(ctx, http.MethodGet, c.baseURI+"/records", nil, &records)
	return records, err
}

func (c *KafkaConsumer) Close(ctx context.Context) error {
	if c.baseURI == "" {
		return nil
	}
	return c.request(ctx, http.MethodDelete, c.baseURI, nil, nil)
}

func (c *KafkaConsumer) request(ctx context.Context, method, endpoint string, input, output any) error {
	var body *bytes.Reader
	if input == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/vnd.kafka.v2+json")
	request.Header.Set("Accept", "application/vnd.kafka.json.v2+json")
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Kafka REST returned %d", response.StatusCode)
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(output)
}
