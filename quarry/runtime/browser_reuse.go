package runtime

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pithecene-io/quarry/types"
)

// BrowserDiscovery is the on-disk schema for a reusable browser server.
// Written to $QUARRY_RUNTIME_DIR/browser.json.
type BrowserDiscovery struct {
	WSEndpoint string `json:"ws_endpoint"`
	PID        int    `json:"pid"`
	ProxyHash  string `json:"proxy_hash"`
	StartedAt  string `json:"started_at"`
}

// ReusableBrowserConfig holds inputs for AcquireReusableBrowser.
type ReusableBrowserConfig struct {
	ExecutorPath string
	ScriptPath   string
	Proxy        *types.ProxyEndpoint
	IdleTimeout  time.Duration // 0 means default (60s)
}

// defaultIdleTimeout is used when ReusableBrowserConfig.IdleTimeout is zero.
const defaultIdleTimeout = 60 * time.Second

// AcquireReusableBrowser returns a WS endpoint for a reusable browser server.
//
// Flow:
//  1. Resolve discovery dir
//  2. Acquire flock on browser.lock
//  3. Read browser.json — if valid + healthy + proxy hash matches → return
//  4. If stale/missing → launch --browser-server, read endpoint, write browser.json
//  5. Release lock
func AcquireReusableBrowser(ctx context.Context, cfg ReusableBrowserConfig) (string, error) {
	dir, err := discoveryDir()
	if err != nil {
		return "", fmt.Errorf("browser reuse: %w", err)
	}

	lockPath := filepath.Join(dir, "browser.lock")
	discoveryPath := filepath.Join(dir, "browser.json")

	// Acquire exclusive file lock
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return "", fmt.Errorf("browser reuse: open lock: %w", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
	}()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return "", fmt.Errorf("browser reuse: flock: %w", err)
	}

	wantHash := proxyHash(cfg.Proxy)

	// Try existing discovery file
	disc, err := readDiscovery(discoveryPath)
	if err == nil {
		// Validate proxy hash
		if disc.ProxyHash != wantHash {
			fmt.Fprintf(os.Stderr, "Proxy mismatch, skipping browser reuse for this run\n")
			return "", errors.New("browser reuse: proxy mismatch")
		}

		// Health check
		if err := healthCheck(disc.WSEndpoint); err == nil {
			age := time.Since(mustParseTime(disc.StartedAt))
			fmt.Fprintf(os.Stderr, "Reusing browser server (pid=%d, age=%s)\n", disc.PID, age.Round(time.Second))
			return disc.WSEndpoint, nil
		}

		// Stale — remove and relaunch
		fmt.Fprintf(os.Stderr, "Stale browser server detected (pid=%d), relaunching\n", disc.PID)
		_ = os.Remove(discoveryPath)
	}

	// Launch new browser server
	idleTimeout := cfg.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = defaultIdleTimeout
	}

	wsEndpoint, pid, err := launchBrowserServerProcess(ctx, cfg.ExecutorPath, cfg.ScriptPath, discoveryPath, idleTimeout, cfg.Proxy)
	if err != nil {
		return "", fmt.Errorf("browser reuse: launch: %w", err)
	}

	// Write discovery file
	disc = &BrowserDiscovery{
		WSEndpoint: wsEndpoint,
		PID:        pid,
		ProxyHash:  wantHash,
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeDiscovery(discoveryPath, disc); err != nil {
		return "", fmt.Errorf("browser reuse: write discovery: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Starting reusable browser server (pid=%d)\n", pid)
	return wsEndpoint, nil
}

// discoveryDir returns the directory for browser discovery files.
// Uses $XDG_RUNTIME_DIR/quarry/ on Linux, falls back to $TMPDIR/quarry-$UID/.
func discoveryDir() (string, error) {
	var dir string
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		dir = filepath.Join(xdg, "quarry")
	} else {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("quarry-%d", os.Getuid()))
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create discovery dir %s: %w", dir, err)
	}
	return dir, nil
}

// healthCheck verifies a browser server is alive by hitting /json/version.
func healthCheck(wsEndpoint string) error {
	u, err := url.Parse(wsEndpoint)
	if err != nil {
		return fmt.Errorf("parse ws endpoint: %w", err)
	}

	healthURL := fmt.Sprintf("http://127.0.0.1:%s/json/version", u.Port())

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %d", resp.StatusCode)
	}
	return nil
}

// proxyHash returns a deterministic hash of proxy config for mismatch detection.
// Returns empty string for nil proxy.
func proxyHash(proxy *types.ProxyEndpoint) string {
	if proxy == nil {
		return ""
	}

	// Hash host, port, protocol, username (not password — avoid leaking secrets to disk)
	h := sha256.New()
	fmt.Fprintf(h, "%s:%s:%d", proxy.Protocol, proxy.Host, proxy.Port)
	if proxy.Username != nil {
		fmt.Fprintf(h, ":%s", *proxy.Username)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

// launchBrowserServerProcess starts the executor in --browser-server mode as a
// detached process. Returns the WS endpoint and PID.
//
// Uses exec.Command (not CommandContext) because the browser server must outlive
// the parent quarry run invocation — that is the entire point of reuse.
func launchBrowserServerProcess(
	ctx context.Context,
	executorPath, scriptPath, discoveryPath string,
	idleTimeout time.Duration,
	proxy *types.ProxyEndpoint,
) (wsEndpoint string, pid int, err error) {
	cmd := exec.Command(executorPath, "--browser-server", scriptPath)

	// Detach: new session so the browser server outlives the parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Pass discovery file path and config so the server can self-manage
	env := append(os.Environ(),
		fmt.Sprintf("QUARRY_BROWSER_DISCOVERY_FILE=%s", discoveryPath),
		fmt.Sprintf("QUARRY_BROWSER_IDLE_TIMEOUT=%d", int(idleTimeout.Seconds())),
	)

	// Pass proxy URL so the browser server launches Chromium with --proxy-server
	if proxy != nil {
		env = append(env, fmt.Sprintf("QUARRY_BROWSER_PROXY=%s://%s:%d", proxy.Protocol, proxy.Host, proxy.Port))
	}

	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", 0, fmt.Errorf("stdout pipe: %w", err)
	}

	// Inherit stderr for diagnostics
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", 0, fmt.Errorf("start browser server: %w", err)
	}

	// Read WS endpoint from first line of stdout
	scanner := bufio.NewScanner(stdout)
	wsURLCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		if scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "ws://") || strings.HasPrefix(line, "wss://") {
				wsURLCh <- line
				return
			}
			errCh <- fmt.Errorf("unexpected browser server output: %q", line)
			return
		}
		if scanErr := scanner.Err(); scanErr != nil {
			errCh <- fmt.Errorf("reading browser server stdout: %w", scanErr)
			return
		}
		errCh <- errors.New("browser server exited without printing WS endpoint")
	}()

	select {
	case ws := <-wsURLCh:
		// Detached process — do NOT call cmd.Wait() (we don't own the lifecycle)
		return ws, cmd.Process.Pid, nil
	case readErr := <-errCh:
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", 0, readErr
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", 0, errors.New("timed out waiting for browser server WS endpoint")
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", 0, ctx.Err()
	}
}

// readDiscovery reads and parses a browser discovery file.
func readDiscovery(path string) (*BrowserDiscovery, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var disc BrowserDiscovery
	if err := json.Unmarshal(data, &disc); err != nil {
		return nil, fmt.Errorf("parse discovery: %w", err)
	}
	if disc.WSEndpoint == "" {
		return nil, errors.New("discovery file missing ws_endpoint")
	}
	return &disc, nil
}

// writeDiscovery atomically writes a discovery file.
func writeDiscovery(path string, disc *BrowserDiscovery) error {
	data, err := json.MarshalIndent(disc, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file, then rename for atomicity
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// mustParseTime parses an RFC3339 time string, returning zero time on error.
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// IdleTimeoutFromEnv reads QUARRY_BROWSER_IDLE_TIMEOUT from the environment.
// Returns 0 if unset or invalid (caller should use defaultIdleTimeout).
func IdleTimeoutFromEnv() time.Duration {
	s := os.Getenv("QUARRY_BROWSER_IDLE_TIMEOUT")
	if s == "" {
		return 0
	}
	sec, err := strconv.Atoi(s)
	if err != nil || sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}
