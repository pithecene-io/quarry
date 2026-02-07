// Package config handles YAML config file loading for quarry run.
package config

import (
	"os"
	"regexp"
)

// envVarPattern matches ${VAR} and ${VAR:-default} patterns.
// - ${VAR} expands to the env var value, or empty string if unset
// - ${VAR:-default} expands to the env var value, or "default" if unset/empty
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// ExpandEnv replaces ${VAR} and ${VAR:-default} patterns in the input string
// with their corresponding environment variable values.
//
// Unset variables without defaults expand to empty string (not an error).
// This is intentional: required secrets will fail at downstream validation
// (e.g., proxy endpoint auth pair validation).
func ExpandEnv(input string) string {
	return envVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		groups := envVarPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}

		varName := groups[1]
		value, ok := os.LookupEnv(varName)
		if ok && value != "" {
			return value
		}

		// Use default if provided (groups[2] is the default value)
		if len(groups) >= 3 && groups[2] != "" {
			return groups[2]
		}

		// Unset without default: empty string
		return ""
	})
}
