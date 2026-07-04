package ptrutil

// Clone returns a pointer to a copy of value, or nil when value is nil.
func Clone[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

// ValueOrZero returns the pointed value, or the zero value when value is nil.
func ValueOrZero[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}

// ValueOrDefault returns the pointed value, or fallback when value is nil.
func ValueOrDefault[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}
	return *value
}
