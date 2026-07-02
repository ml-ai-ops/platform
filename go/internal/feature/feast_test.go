package feature

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeastClientTranslatesColumnarResponse(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/get-online-features" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		_, _ = w.Write([]byte(`{
			"metadata": {"feature_names": ["plan", "open_tickets"]},
			"results": [
				{"values": ["pro", "free"], "statuses": ["PRESENT", "PRESENT"]},
				{"values": [1, 0], "statuses": ["PRESENT", "NOT_FOUND"]}
			]
		}`))
	}))
	defer server.Close()

	client := NewFeastClient(server.URL)
	response, err := client.Lookup(Request{
		FeatureService: "customer_profile",
		Entities:       []map[string]any{{"user_id": "u123"}, {"user_id": "u456"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	entities := received["entities"].(map[string]any)["user_id"].([]any)
	if len(entities) != 2 || entities[0] != "u123" {
		t.Fatalf("feast request not columnar: %v", received)
	}
	if len(response.Results) != 2 {
		t.Fatalf("expected 2 entity rows, got %d", len(response.Results))
	}
	first := response.Results[0]
	if first.Values["plan"] != "pro" || first.Values["open_tickets"] != float64(1) {
		t.Fatalf("row translation wrong: %v", first.Values)
	}
	second := response.Results[1]
	if second.Statuses["open_tickets"] != "NOT_FOUND" {
		t.Fatalf("statuses not carried: %v", second.Statuses)
	}
}

func TestFeastClientRejectsMisalignedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"metadata": {"feature_names": ["a", "b"]}, "results": [{"values": [1]}]}`))
	}))
	defer server.Close()
	_, err := NewFeastClient(server.URL).Lookup(Request{
		FeatureService: "svc", Entities: []map[string]any{{"id": "x"}},
	})
	if err == nil {
		t.Fatal("expected misalignment error")
	}
}

func TestFeastClientSurfacesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	_, err := NewFeastClient(server.URL).Lookup(Request{
		FeatureService: "svc", Entities: []map[string]any{{"id": "x"}},
	})
	if err == nil {
		t.Fatal("expected upstream error to fail closed")
	}
}
