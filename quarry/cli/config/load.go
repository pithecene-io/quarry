package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML config file, expands environment variables, and
// unmarshals into a Config struct. Unknown keys are rejected to catch
// typos early.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("cannot read config file %q: %w", path, err)
	}

	expanded := ExpandEnv(string(data))

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader([]byte(expanded)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("invalid YAML in %s: %w", path, err)
	}

	return &cfg, nil
}
