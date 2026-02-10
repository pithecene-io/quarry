// Package runtime provides E2E tests for browser reuse.
//
// These tests spawn real browser server processes and validate the full
// AcquireReusableBrowser flow including discovery file I/O, health checks,
// idle timeout, and proxy mismatch handling.
//
// Test gating:
//   - All tests require QUARRY_E2E=1 (slow, requires Node + Puppeteer + Chromium)
//   - The executor must be built: pnpm -C executor-node run build
package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/pithecene-io/quarry/types"
)

// e2ePaths holds resolved paths for browser reuse E2E tests.
type e2ePaths struct {
	repoRoot    string
	executorBin string
}

// resolveE2EPaths finds the repository root and executor binary.
func resolveE2EPaths(t *testing.T) e2ePaths {
	t.Helper()

	// This file is at quarry/runtime/browser_reuse_e2e_test.go
	// Repo root is two levels up from quarry/runtime/
	_, thisFile, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("failed to get caller info")
	}
	runtimeDir := filepath.Dir(thisFile)
	quarryDir := filepath.Dir(runtimeDir)
	repoRoot := filepath.Dir(quarryDir)

	return e2ePaths{
		repoRoot:    repoRoot,
		executorBin: filepath.Join(repoRoot, "executor-node", "dist", "bin", "executor.js"),
	}
}

// skipUnlessE2E skips the test if QUARRY_E2E=1 is not set.
func skipUnlessE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("QUARRY_E2E") != "1" {
		t.Skip("QUARRY_E2E=1 not set, skipping browser reuse E2E test")
	}
}

// skipUnlessNode skips if Node.js is not available.
func skipUnlessNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available, skipping E2E test")
	}
}

// skipUnlessExecutorBuilt skips if the executor is not built.
func skipUnlessExecutorBuilt(t *testing.T, paths e2ePaths) {
	t.Helper()
	if _, err := os.Stat(paths.executorBin); os.IsNotExist(err) {
		t.Skipf("executor not built at %s, run 'pnpm -C executor-node run build'", paths.executorBin)
	}
}

// findExampleScript returns the path to a simple example script for testing.
// Falls back to creating a minimal inline script if no example is found.
func findExampleScript(t *testing.T, repoRoot string) string {
	t.Helper()

	// Use the demo example if available
	demo := filepath.Join(repoRoot, "examples", "demo.ts")
	if _, err := os.Stat(demo); err == nil {
		return demo
	}

	// Create a minimal script
	dir := t.TempDir()
	script := filepath.Join(dir, "test-script.ts")
	content := `import { type QuarryContext } from '@pithecene-io/quarry-sdk';
export default async function(ctx: QuarryContext) {
  await ctx.emit.runComplete();
}`
	if err := os.WriteFile(script, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}
	return script
}

// cleanupBrowserServer kills a browser server by PID (best effort).
func cleanupBrowserServer(t *testing.T, discoveryPath string) {
	t.Helper()
	data, err := os.ReadFile(discoveryPath)
	if err != nil {
		return
	}
	var disc BrowserDiscovery
	if err := json.Unmarshal(data, &disc); err != nil {
		return
	}
	if disc.PID > 0 {
		proc, err := os.FindProcess(disc.PID)
		if err == nil {
			_ = proc.Signal(syscall.SIGTERM)
			// Give it a moment to shut down
			time.Sleep(500 * time.Millisecond)
			_ = proc.Signal(syscall.SIGKILL)
		}
	}
	_ = os.Remove(discoveryPath)
}

// TestE2E_BrowserReuse_SequentialAcquire verifies that two sequential
// AcquireReusableBrowser calls reuse the same browser (same WS endpoint).
func TestE2E_BrowserReuse_SequentialAcquire(t *testing.T) {
	skipUnlessE2E(t)
	skipUnlessNode(t)
	paths := resolveE2EPaths(t)
	skipUnlessExecutorBuilt(t, paths)

	// Use a temp dir for discovery so we don't interfere with real state
	discoveryDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", discoveryDir)
	t.Setenv("QUARRY_NO_SANDBOX", "1")

	scriptPath := findExampleScript(t, paths.repoRoot)
	discoveryPath := filepath.Join(discoveryDir, "quarry", "browser.json")

	ctx := t.Context()
	cfg := ReusableBrowserConfig{
		ExecutorPath: paths.executorBin,
		ScriptPath:   scriptPath,
		IdleTimeout:  30 * time.Second,
	}

	// Cleanup on exit
	t.Cleanup(func() {
		cleanupBrowserServer(t, discoveryPath)
	})

	// First acquire — should launch a new browser
	ws1, err := AcquireReusableBrowser(ctx, cfg)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if ws1 == "" {
		t.Fatal("first acquire returned empty endpoint")
	}
	if !strings.HasPrefix(ws1, "ws://") {
		t.Errorf("expected ws:// prefix, got %q", ws1)
	}
	t.Logf("first acquire: %s", ws1)

	// Verify discovery file exists
	if _, err := os.Stat(discoveryPath); err != nil {
		t.Fatalf("discovery file not created: %v", err)
	}

	// Second acquire — should reuse the same browser
	ws2, err := AcquireReusableBrowser(ctx, cfg)
	if err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	if ws2 != ws1 {
		t.Errorf("expected same endpoint %q, got %q (browser not reused)", ws1, ws2)
	}
	t.Logf("second acquire: %s (reused: %v)", ws2, ws1 == ws2)
}

// TestE2E_BrowserReuse_ProxyMismatch verifies that a proxy mismatch causes
// AcquireReusableBrowser to return an error (fallback to per-run launch).
func TestE2E_BrowserReuse_ProxyMismatch(t *testing.T) {
	skipUnlessE2E(t)
	skipUnlessNode(t)
	paths := resolveE2EPaths(t)
	skipUnlessExecutorBuilt(t, paths)

	discoveryDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", discoveryDir)
	t.Setenv("QUARRY_NO_SANDBOX", "1")

	scriptPath := findExampleScript(t, paths.repoRoot)
	discoveryPath := filepath.Join(discoveryDir, "quarry", "browser.json")

	ctx := t.Context()

	// First acquire with no proxy
	cfg1 := ReusableBrowserConfig{
		ExecutorPath: paths.executorBin,
		ScriptPath:   scriptPath,
		IdleTimeout:  30 * time.Second,
	}

	t.Cleanup(func() {
		cleanupBrowserServer(t, discoveryPath)
	})

	ws1, err := AcquireReusableBrowser(ctx, cfg1)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	t.Logf("first acquire (no proxy): %s", ws1)

	// Second acquire with a proxy — should fail with mismatch
	cfg2 := ReusableBrowserConfig{
		ExecutorPath: paths.executorBin,
		ScriptPath:   scriptPath,
		Proxy: &types.ProxyEndpoint{
			Protocol: types.ProxyProtocolHTTP,
			Host:     "proxy.example.com",
			Port:     8080,
		},
		IdleTimeout: 30 * time.Second,
	}

	_, err = AcquireReusableBrowser(ctx, cfg2)
	if err == nil {
		t.Error("expected error for proxy mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "proxy mismatch") {
		t.Errorf("expected 'proxy mismatch' in error, got: %v", err)
	}
	t.Logf("proxy mismatch correctly detected: %v", err)
}

// TestE2E_BrowserReuse_StaleRecovery verifies that a stale discovery file
// (browser process dead) triggers a relaunch.
func TestE2E_BrowserReuse_StaleRecovery(t *testing.T) {
	skipUnlessE2E(t)
	skipUnlessNode(t)
	paths := resolveE2EPaths(t)
	skipUnlessExecutorBuilt(t, paths)

	discoveryDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", discoveryDir)
	t.Setenv("QUARRY_NO_SANDBOX", "1")

	scriptPath := findExampleScript(t, paths.repoRoot)
	quarryDir := filepath.Join(discoveryDir, "quarry")
	discoveryPath := filepath.Join(quarryDir, "browser.json")

	ctx := t.Context()
	cfg := ReusableBrowserConfig{
		ExecutorPath: paths.executorBin,
		ScriptPath:   scriptPath,
		IdleTimeout:  30 * time.Second,
	}

	t.Cleanup(func() {
		cleanupBrowserServer(t, discoveryPath)
	})

	// First acquire
	ws1, err := AcquireReusableBrowser(ctx, cfg)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	t.Logf("first acquire: %s", ws1)

	// Read PID from discovery file
	data, err := os.ReadFile(discoveryPath)
	if err != nil {
		t.Fatalf("failed to read discovery file: %v", err)
	}
	var disc BrowserDiscovery
	if err := json.Unmarshal(data, &disc); err != nil {
		t.Fatalf("failed to parse discovery: %v", err)
	}

	// Kill the browser process to simulate crash
	proc, err := os.FindProcess(disc.PID)
	if err != nil {
		t.Fatalf("failed to find process %d: %v", disc.PID, err)
	}
	if err := proc.Signal(syscall.SIGKILL); err != nil {
		t.Logf("warning: failed to kill process %d: %v", disc.PID, err)
	}
	// Wait for process to die
	time.Sleep(1 * time.Second)

	// Second acquire — should detect stale, relaunch
	ws2, err := AcquireReusableBrowser(ctx, cfg)
	if err != nil {
		t.Fatalf("second acquire (stale recovery) failed: %v", err)
	}
	if ws2 == "" {
		t.Fatal("second acquire returned empty endpoint")
	}
	if ws2 == ws1 {
		t.Error("expected different endpoint after stale recovery, got same")
	}
	t.Logf("stale recovery: old=%s new=%s", ws1, ws2)
}

// TestE2E_BrowserReuse_HealthCheck verifies that the health check correctly
// detects a live browser server.
func TestE2E_BrowserReuse_HealthCheck(t *testing.T) {
	skipUnlessE2E(t)
	skipUnlessNode(t)
	paths := resolveE2EPaths(t)
	skipUnlessExecutorBuilt(t, paths)

	discoveryDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", discoveryDir)
	t.Setenv("QUARRY_NO_SANDBOX", "1")

	scriptPath := findExampleScript(t, paths.repoRoot)
	discoveryPath := filepath.Join(discoveryDir, "quarry", "browser.json")

	ctx := t.Context()
	cfg := ReusableBrowserConfig{
		ExecutorPath: paths.executorBin,
		ScriptPath:   scriptPath,
		IdleTimeout:  30 * time.Second,
	}

	t.Cleanup(func() {
		cleanupBrowserServer(t, discoveryPath)
	})

	ws, err := AcquireReusableBrowser(ctx, cfg)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// Health check should succeed
	if err := healthCheck(ws); err != nil {
		t.Errorf("health check failed on live browser: %v", err)
	}

	// Also verify /json/version returns useful data
	port := extractPort(t, ws)
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/json/version", port))
	if err != nil {
		t.Fatalf("GET /json/version failed: %v", err)
	}
	defer resp.Body.Close()

	var version map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		t.Fatalf("failed to decode /json/version: %v", err)
	}

	if _, ok := version["Browser"]; !ok {
		t.Error("expected 'Browser' field in /json/version response")
	}
	t.Logf("browser version: %v", version["Browser"])
}

// TestE2E_BrowserReuse_IdleShutdown verifies that the browser server
// self-terminates after the idle timeout expires.
func TestE2E_BrowserReuse_IdleShutdown(t *testing.T) {
	skipUnlessE2E(t)
	skipUnlessNode(t)
	paths := resolveE2EPaths(t)
	skipUnlessExecutorBuilt(t, paths)

	if testing.Short() {
		t.Skip("skipping idle shutdown test in short mode (waits for timeout)")
	}

	discoveryDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", discoveryDir)
	t.Setenv("QUARRY_NO_SANDBOX", "1")

	scriptPath := findExampleScript(t, paths.repoRoot)
	discoveryPath := filepath.Join(discoveryDir, "quarry", "browser.json")

	ctx := t.Context()

	// Use a very short idle timeout (10s) for this test
	cfg := ReusableBrowserConfig{
		ExecutorPath: paths.executorBin,
		ScriptPath:   scriptPath,
		IdleTimeout:  10 * time.Second,
	}

	ws, err := AcquireReusableBrowser(ctx, cfg)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	t.Logf("launched browser server: %s", ws)

	// Verify it's alive
	if err := healthCheck(ws); err != nil {
		t.Fatalf("browser not alive after launch: %v", err)
	}

	// Wait for idle timeout + poll interval + grace
	// idle_timeout=10s, poll_interval=5s, so max wait ~20s
	t.Log("waiting for idle shutdown (~20s)...")
	deadline := time.Now().Add(25 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		lastErr = healthCheck(ws)
		if lastErr != nil {
			t.Logf("browser shut down after idle timeout (health check failed: %v)", lastErr)
			return
		}
	}

	// Cleanup manually if it didn't shut down
	cleanupBrowserServer(t, discoveryPath)
	t.Errorf("browser did not self-terminate after idle timeout; last health check: %v", lastErr)
}

// extractPort extracts the port from a ws:// URL.
func extractPort(t *testing.T, wsEndpoint string) string {
	t.Helper()
	// ws://127.0.0.1:PORT/devtools/browser/UUID
	parts := strings.Split(wsEndpoint, ":")
	if len(parts) < 3 {
		t.Fatalf("cannot extract port from %q", wsEndpoint)
	}
	portAndPath := parts[2]
	port := strings.Split(portAndPath, "/")[0]
	return port
}
