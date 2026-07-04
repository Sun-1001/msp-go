package httpquery

import (
	"net/url"
	"strings"
)

// StringList splits repeated query values and comma-separated entries.
func StringList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

// NamedStringList reads both name and name[] query values as a string list.
func NamedStringList(query url.Values, name string) []string {
	values := make([]string, 0, len(query[name])+len(query[name+"[]"]))
	values = append(values, query[name]...)
	values = append(values, query[name+"[]"]...)
	return StringList(values)
}
