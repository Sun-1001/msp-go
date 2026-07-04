package sliceutil

// CloneStrings copies values while preserving the repository/application DTO convention that nil becomes an empty slice.
func CloneStrings(values []string) []string {
	return append([]string{}, values...)
}
