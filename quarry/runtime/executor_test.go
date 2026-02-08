package runtime

import (
	"encoding/json"
	"testing"

	"github.com/justapithecus/quarry/types"
)

func TestExecutorInputJSON_IncludesBrowserWSEndpoint(t *testing.T) {
	input := executorInput{
		RunID:             "run-001",
		Attempt:           1,
		Job:               map[string]any{"url": "https://example.com"},
		BrowserWSEndpoint: "ws://127.0.0.1:9222/devtools/browser/abc-123",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	wsEndpoint, ok := decoded["browser_ws_endpoint"].(string)
	if !ok {
		t.Fatal("browser_ws_endpoint field missing from JSON output")
	}
	if wsEndpoint != "ws://127.0.0.1:9222/devtools/browser/abc-123" {
		t.Errorf("browser_ws_endpoint = %q, want %q", wsEndpoint, "ws://127.0.0.1:9222/devtools/browser/abc-123")
	}
}

func TestExecutorInputJSON_OmitsBrowserWSEndpointWhenEmpty(t *testing.T) {
	input := executorInput{
		RunID:   "run-001",
		Attempt: 1,
		Job:     map[string]any{},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, exists := decoded["browser_ws_endpoint"]; exists {
		t.Error("browser_ws_endpoint should be omitted when empty")
	}
}

func TestExecutorInputJSON_IncludesProxy(t *testing.T) {
	proxy := &types.ProxyEndpoint{
		Protocol: "http",
		Host:     "proxy.example.com",
		Port:     8080,
	}
	input := executorInput{
		RunID:             "run-001",
		Attempt:           1,
		Job:               map[string]any{},
		Proxy:             proxy,
		BrowserWSEndpoint: "ws://127.0.0.1:9222/devtools/browser/abc",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Both fields present
	if _, ok := decoded["proxy"]; !ok {
		t.Error("proxy field missing from JSON output")
	}
	if _, ok := decoded["browser_ws_endpoint"]; !ok {
		t.Error("browser_ws_endpoint field missing from JSON output")
	}
}
