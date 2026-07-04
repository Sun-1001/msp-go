package postgres

import "encoding/json"

func decodeFloatMap(raw []byte) (map[string]float64, error) {
	if len(raw) == 0 {
		return map[string]float64{}, nil
	}
	values := map[string]float64{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func decodeStringSlice(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	values := []string{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func decodeObjectMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
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
