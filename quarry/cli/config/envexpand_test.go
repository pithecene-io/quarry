package config

import (
	"testing"
)

func TestExpandEnv_SetVar(t *testing.T) {
	t.Setenv("TEST_VAR", "hello")

	got := ExpandEnv("value: ${TEST_VAR}")
	want := "value: hello"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandEnv_UnsetVar(t *testing.T) {
	got := ExpandEnv("value: ${UNSET_VAR_12345}")
	want := "value: "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandEnv_DefaultUsedWhenUnset(t *testing.T) {
	got := ExpandEnv("value: ${UNSET_VAR_12345:-fallback}")
	want := "value: fallback"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandEnv_DefaultIgnoredWhenSet(t *testing.T) {
	t.Setenv("TEST_VAR", "real")

	got := ExpandEnv("value: ${TEST_VAR:-fallback}")
	want := "value: real"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandEnv_DefaultUsedWhenEmpty(t *testing.T) {
	t.Setenv("TEST_VAR", "")

	got := ExpandEnv("value: ${TEST_VAR:-fallback}")
	want := "value: fallback"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandEnv_MultipleVars(t *testing.T) {
	t.Setenv("USER_A", "alice")
	t.Setenv("USER_B", "bob")

	got := ExpandEnv("${USER_A}:${USER_B}")
	want := "alice:bob"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandEnv_NoVars(t *testing.T) {
	input := "no variables here"
	got := ExpandEnv(input)
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}

func TestExpandEnv_NestedInYAML(t *testing.T) {
	t.Setenv("PROXY_USER", "admin")
	t.Setenv("PROXY_PASS", "secret")

	input := `proxies:
  pool1:
    endpoints:
      - username: ${PROXY_USER}
        password: ${PROXY_PASS}`

	got := ExpandEnv(input)
	want := `proxies:
  pool1:
    endpoints:
      - username: admin
        password: secret`

	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
