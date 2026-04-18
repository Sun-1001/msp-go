package postgres

import "fmt"

const (
	// DefaultLimit matches the existing Python repository list default.
	DefaultLimit = 100
	// MaxLimit keeps list endpoints from issuing accidental unbounded scans.
	MaxLimit = 500
)

// Page stores safe SQL LIMIT/OFFSET values.
type Page struct {
	Limit  int
	Offset int
}

// NewPage validates pagination values used by repositories.
func NewPage(offset int, limit int) (Page, error) {
	if offset < 0 {
		return Page{}, fmt.Errorf("offset must be zero or greater, got %d", offset)
	}
	if limit < 0 {
		return Page{}, fmt.Errorf("limit must be zero or greater, got %d", limit)
	}
	if limit == 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	return Page{Limit: limit, Offset: offset}, nil
}
