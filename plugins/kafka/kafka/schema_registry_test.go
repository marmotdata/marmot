package kafka

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A topic picks up every subject named after it, with basic auth from basic.auth.user.info, and an Avro schema (which the registry sends without a schemaType) is reported as AVRO.
func TestEnrichWithSchemaRegistry(t *testing.T) {
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "sr-key" || pass != "sr:secret" {
			http.Error(w, `{"error_code":401,"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/subjects":
			_ = json.NewEncoder(w).Encode([]string{"orders-value", "orders-key", "orders.v2-value", "other-value"})
		case r.URL.Path == "/subjects/orders-value/versions/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{"subject": "orders-value", "version": 3, "id": 7, "schema": `{"type":"record","name":"Order","fields":[]}`})
		case r.URL.Path == "/subjects/orders-key/versions/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{"subject": "orders-key", "version": 1, "id": 2, "schemaType": "JSON", "schema": `{"type":"string"}`})
		default:
			http.Error(w, `{"error_code":40401,"message":"not found"}`, http.StatusNotFound)
		}
	}))
	defer registry.Close()

	s := &Source{config: &Config{SchemaRegistry: &SchemaRegistryConfig{
		URL:     registry.URL,
		Enabled: true,
		Config:  map[string]string{"basic.auth.user.info": "sr-key:sr:secret"},
	}}}
	if err := s.initSchemaRegistry(); err != nil {
		t.Fatal(err)
	}

	metadata := map[string]interface{}{}
	schema, err := s.enrichWithSchemaRegistry(context.Background(), "orders", metadata)
	if err != nil {
		t.Fatal(err)
	}

	if len(schema) != 2 || !strings.Contains(schema["orders-value"], `"Order"`) || schema["orders-key"] != `{"type":"string"}` {
		t.Fatalf("schema = %v", schema)
	}
	want := map[string]interface{}{
		"value_schema_id": 7, "value_schema_version": 3, "value_schema_type": "AVRO",
		"key_schema_id": 2, "key_schema_version": 1, "key_schema_type": "JSON",
	}
	for k, v := range want {
		if metadata[k] != v {
			t.Errorf("metadata[%q] = %v, want %v", k, metadata[k], v)
		}
	}
}
