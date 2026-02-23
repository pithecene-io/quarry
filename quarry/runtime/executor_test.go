package runtime

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/pithecene-io/quarry/types"
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

func TestDeduplicateEnv_LastWins(t *testing.T) {
	env := []string{
		"NODE_PATH=/old",
		"HOME=/home/user",
		"NODE_PATH=/new",
	}
	result := deduplicateEnv(env)

	// Last occurrence of NODE_PATH wins
	if !slices.Contains(result, "NODE_PATH=/new") {
		t.Error("expected NODE_PATH=/new to be kept")
	}
	if slices.Contains(result, "NODE_PATH=/old") {
		t.Error("expected NODE_PATH=/old to be removed")
	}
	if !slices.Contains(result, "HOME=/home/user") {
		t.Error("expected HOME=/home/user to be preserved")
	}
}

func TestDeduplicateEnv_NoDuplicates(t *testing.T) {
	env := []string{
		"HOME=/home/user",
		"PATH=/usr/bin",
	}
	result := deduplicateEnv(env)

	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
}

func TestDeduplicateEnv_Empty(t *testing.T) {
	result := deduplicateEnv(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestExecutorConfig_ResolveFromEnvSetup(t *testing.T) {
	// Verify that the config struct accepts ResolveFrom
	config := &ExecutorConfig{
		ExecutorPath: "/usr/bin/node",
		ScriptPath:   "/app/script.ts",
		ResolveFrom:  "/app/node_modules",
		RunMeta: &types.RunMeta{
			RunID:   "run-001",
			Attempt: 1,
		},
	}

	if config.ResolveFrom != "/app/node_modules" {
		t.Errorf("ResolveFrom = %q, want %q", config.ResolveFrom, "/app/node_modules")
	}
}

func TestExecutorInputJSON_IncludesStoragePartition(t *testing.T) {
	input := executorInput{
		RunID:   "run-001",
		Attempt: 1,
		Job:     map[string]any{},
		Storage: &StoragePartition{
			Dataset:  "quarry",
			Source:   "my-source",
			Category: "default",
			Day:      "2026-02-23",
			RunID:    "run-001",
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	storage, ok := decoded["storage"].(map[string]any)
	if !ok {
		t.Fatal("storage field missing or not an object")
	}

	checks := map[string]string{
		"dataset":  "quarry",
		"source":   "my-source",
		"category": "default",
		"day":      "2026-02-23",
		"run_id":   "run-001",
	}
	for key, want := range checks {
		got, exists := storage[key].(string)
		if !exists {
			t.Errorf("storage.%s missing", key)
		} else if got != want {
			t.Errorf("storage.%s = %q, want %q", key, got, want)
		}
	}
}

func TestExecutorInputJSON_OmitsStorageWhenNil(t *testing.T) {
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

	if _, exists := decoded["storage"]; exists {
		t.Error("storage should be omitted when nil")
	}
}

func TestStoragePartition_MatchesGoPathFormula(t *testing.T) {
	// Verify StoragePartition fields produce a path matching
	// the Go buildFilePath() formula in quarry/lode/file_writer.go:
	// datasets/{dataset}/partitions/source={s}/category={c}/day={d}/run_id={r}/files/{filename}
	sp := StoragePartition{
		Dataset:  "quarry",
		Source:   "my-source",
		Category: "default",
		Day:      "2026-02-23",
		RunID:    "run-001",
	}

	filename := "screenshot.png"
	want := "datasets/quarry/partitions/source=my-source/category=default/day=2026-02-23/run_id=run-001/files/screenshot.png"

	// Reproduce the Go formula
	got := "datasets/" + sp.Dataset + "/partitions/source=" + sp.Source + "/category=" + sp.Category + "/day=" + sp.Day + "/run_id=" + sp.RunID + "/files/" + filename

	if got != want {
		t.Errorf("path formula mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}
