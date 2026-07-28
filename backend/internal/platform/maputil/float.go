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

// AverageFloatValues returns the average of map values, or 0 when values is empty.
func AverageFloatValues(values map[string]float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

// SortedFloatKeys returns the keys of values in ascending lexical order.
func SortedFloatKeys(values map[string]float64) []string {
	return SortedStringKeys(values)
}

// SortedFloatKeysByValueDesc returns keys ordered by descending value, then ascending key.
func SortedFloatKeysByValueDesc(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if values[keys[i]] == values[keys[j]] {
			return keys[i] < keys[j]
		}
		return values[keys[i]] > values[keys[j]]
	})
	return keys
}
