package maputil

import "sort"

// SortedStringKeys returns the string keys of values in ascending lexical order.
func SortedStringKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
