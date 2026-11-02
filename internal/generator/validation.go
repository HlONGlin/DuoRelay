package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// ValidateJSONConfig validates the syntax and minimum shape of a generated
// JSON configuration. Kernel-specific semantic validation is performed by
// kernel.Manager, which has access to the selected sing-box binary.
func ValidateJSONConfig(config []byte, required ...string) error {
	if len(bytes.TrimSpace(config)) == 0 {
		return fmt.Errorf("generated JSON config is empty")
	}
	var root map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(config))
	if err := decoder.Decode(&root); err != nil {
		return fmt.Errorf("invalid generated JSON config: %w", err)
	}
	if root == nil {
		return fmt.Errorf("generated JSON config must be an object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid trailing JSON in generated config: %w", err)
	} else if err == nil {
		return fmt.Errorf("generated JSON config contains multiple documents")
	}
	for _, field := range required {
		value, ok := root[field]
		if !ok || len(bytes.TrimSpace(value)) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("generated JSON config is missing %q", field)
		}
	}
	return nil
}

// ValidateYAMLConfig validates the syntax and minimum shape of a generated
// YAML configuration. Kernel-specific semantic validation is performed by
// kernel.Manager, which has access to the selected mihomo binary.
func ValidateYAMLConfig(config []byte, required ...string) error {
	if len(bytes.TrimSpace(config)) == 0 {
		return fmt.Errorf("generated YAML config is empty")
	}
	var root map[string]any
	decoder := yaml.NewDecoder(bytes.NewReader(config))
	if err := decoder.Decode(&root); err != nil {
		return fmt.Errorf("invalid generated YAML config: %w", err)
	}
	if root == nil {
		return fmt.Errorf("generated YAML config must be a mapping")
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid trailing YAML in generated config: %w", err)
	} else if err == nil {
		return fmt.Errorf("generated YAML config contains multiple documents")
	}
	for _, field := range required {
		value, ok := root[field]
		if !ok || value == nil {
			return fmt.Errorf("generated YAML config is missing %q", field)
		}
	}
	return nil
}
