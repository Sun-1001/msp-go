package numutil

// TotalPages returns ceil(total / pageSize), or 0 for empty or invalid inputs.
func TotalPages(total int, pageSize int) int {
	if total <= 0 || pageSize <= 0 {
		return 0
	}
	return (total + pageSize - 1) / pageSize
}
