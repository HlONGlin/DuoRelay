package overlayutil

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func DecodeObject(input string) (map[string]any, error) {
	var object map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &object); err != nil {
		return nil, fmt.Errorf("invalid json config: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("invalid json config: object required")
	}
	return object, nil
}

func Lookup(object map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := object[key]; ok && value != nil {
			return value, true
		}
	}
	return nil, false
}

func String(object map[string]any, keys ...string) (string, bool, error) {
	value, ok := Lookup(object, keys...)
	if !ok {
		return "", false, nil
	}
	text, err := stringValue(value)
	if err != nil {
		return "", true, fieldError(keys, err)
	}
	return text, true, nil
}

func Bool(object map[string]any, keys ...string) (bool, bool, error) {
	value, ok := Lookup(object, keys...)
	if !ok {
		return false, false, nil
	}
	flag, err := boolValue(value)
	if err != nil {
		return false, true, fieldError(keys, err)
	}
	return flag, true, nil
}

func Int(object map[string]any, keys ...string) (int, bool, error) {
	value, ok := Lookup(object, keys...)
	if !ok {
		return 0, false, nil
	}
	number, err := intValue(value)
	if err != nil {
		return 0, true, fieldError(keys, err)
	}
	return number, true, nil
}

func Strings(object map[string]any, keys ...string) ([]string, bool, error) {
	value, ok := Lookup(object, keys...)
	if !ok {
		return nil, false, nil
	}
	items, err := stringsValue(value)
	if err != nil {
		return nil, true, fieldError(keys, err)
	}
	return items, true, nil
}

func QueryString(query url.Values, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func QueryBool(query url.Values, keys ...string) (bool, bool, error) {
	text := QueryString(query, keys...)
	if text == "" {
		return false, false, nil
	}
	flag, err := boolValue(text)
	if err != nil {
		return false, true, fieldError(keys, err)
	}
	return flag, true, nil
}

func QueryInt(query url.Values, keys ...string) (int, bool, error) {
	text := QueryString(query, keys...)
	if text == "" {
		return 0, false, nil
	}
	number, err := intValue(text)
	if err != nil {
		return 0, true, fieldError(keys, err)
	}
	return number, true, nil
}

func QueryStrings(query url.Values, keys ...string) []string {
	var result []string
	seen := map[string]bool{}
	for _, key := range keys {
		for _, value := range query[key] {
			for _, part := range strings.Split(value, ",") {
				part = strings.TrimSpace(part)
				if part != "" && !seen[part] {
					seen[part] = true
					result = append(result, part)
				}
			}
		}
	}
	return result
}

func stringValue(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case json.Number:
		return typed.String(), nil
	default:
		return "", fmt.Errorf("expected string, got %T", value)
	}
}

func boolValue(value any) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true, nil
		case "0", "false", "no", "off":
			return false, nil
		}
		return false, fmt.Errorf("expected boolean, got %q", typed)
	default:
		return false, fmt.Errorf("expected boolean, got %T", value)
	}
}

func intValue(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case uint64:
		return int(typed), nil
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("expected integer, got %v", typed)
		}
		return int(typed), nil
	case json.Number:
		number, err := strconv.Atoi(typed.String())
		if err != nil {
			return 0, fmt.Errorf("expected integer, got %q", typed)
		}
		return number, nil
	case string:
		number, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("expected integer, got %q", typed)
		}
		return number, nil
	default:
		return 0, fmt.Errorf("expected integer, got %T", value)
	}
}

func stringsValue(value any) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return compact(typed), nil
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, err := stringValue(item)
			if err != nil {
				return nil, err
			}
			result = append(result, text)
		}
		return compact(result), nil
	case string:
		return compact(strings.Split(typed, ",")), nil
	default:
		return nil, fmt.Errorf("expected string list, got %T", value)
	}
}

func compact(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func fieldError(keys []string, err error) error {
	return fmt.Errorf("%s: %w", strings.Join(keys, "/"), err)
}
