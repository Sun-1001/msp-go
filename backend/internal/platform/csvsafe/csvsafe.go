package csvsafe

import "strings"

// Cell escapes values that spreadsheet applications may interpret as formulas.
func Cell(value string) string {
	if value == "" {
		return ""
	}
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

// Row applies Cell to every field in a CSV row.
func Row(values ...string) []string {
	row := make([]string, len(values))
	for index, value := range values {
		row[index] = Cell(value)
	}
	return row
}
