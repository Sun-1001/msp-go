package maputil

import "sort"

// CloneFloatMap copies values while preserving the repository/application DTO convention that nil becomes an empty map.
func CloneFloatMap(values map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

// SortedFloatKeys returns the keys of values in ascending lexical order.
func SortedFloatKeys(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
