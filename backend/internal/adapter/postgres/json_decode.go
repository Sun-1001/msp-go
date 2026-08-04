package postgres

import (
	"bytes"
	"encoding/json"
)

func decodeFloatMap(raw []byte) (map[string]float64, error) {
	if isEmptyJSON(raw) {
		return map[string]float64{}, nil
	}
	if !hasJSONContainerPrefix(raw, '{') {
		return map[string]float64{}, nil
	}
	values := map[string]float64{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func decodeStringSlice(raw []byte) ([]string, error) {
	if isEmptyJSON(raw) {
		return []string{}, nil
	}
	if !hasJSONContainerPrefix(raw, '[') {
		var scalar string
		if err := json.Unmarshal(raw, &scalar); err == nil {
			if scalar == "" {
				return []string{}, nil
			}
			return []string{scalar}, nil
		}
		return []string{}, nil
	}
	values := []string{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}

func decodeObjectMap(raw []byte) (map[string]any, error) {
	if isEmptyJSON(raw) {
		return map[string]any{}, nil
	}
	if !hasJSONContainerPrefix(raw, '{') {
		return map[string]any{}, nil
	}
	values := map[string]any{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return map[string]any{}, nil
	}
	return values, nil
}

func isEmptyJSON(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func hasJSONContainerPrefix(raw []byte, prefix byte) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == prefix
}
